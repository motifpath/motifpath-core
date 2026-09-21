package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/concept"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnode"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/language"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/skill"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntContentNodeRepository persists ContentNode records via ent/Postgres.
type EntContentNodeRepository struct {
	client *ent.Client
}

func NewEntContentNodeRepository(client *ent.Client) *EntContentNodeRepository {
	return &EntContentNodeRepository{client: client}
}

func (r *EntContentNodeRepository) Create(ctx context.Context, node domain.ContentNode) error {
	id, err := uuid.Parse(node.ID)
	if err != nil {
		return err
	}
	teacherID, err := uuid.Parse(node.TeacherID)
	if err != nil {
		return err
	}
	langIDs, err := languageIDsByCode(ctx, r.client.Language, node.Languages)
	if err != nil {
		return err
	}
	skillIDs, err := parseUUIDs(node.Classification.SkillIDs())
	if err != nil {
		return err
	}
	conceptIDs, err := parseUUIDs(node.Classification.ConceptIDs())
	if err != nil {
		return err
	}
	_, err = r.client.ContentNode.Create().
		SetID(id).
		SetTeacherID(teacherID).
		SetTitle(node.Title).
		SetContentType(contentnode.ContentType(node.ContentType)).
		SetDifficultyLevel(contentnode.DifficultyLevel(node.Classification.DifficultyLevel)).
		SetReviewState(contentnode.ReviewState(node.Classification.ReviewState)).
		SetCreatedAt(node.CreatedAt).
		AddLanguageIDs(langIDs...).
		AddSkillIDs(skillIDs...).
		AddConceptIDs(conceptIDs...).
		Save(ctx)
	return err
}

func (r *EntContentNodeRepository) GetByID(ctx context.Context, id string) (domain.ContentNode, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ContentNode{}, domain.ErrNotFound
	}
	row, err := r.client.ContentNode.Query().
		Where(contentnode.ID(parsed)).
		WithLanguages().
		WithSkills().
		WithConcepts().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ContentNode{}, domain.ErrNotFound
		}
		return domain.ContentNode{}, err
	}
	return toDomainContentNode(row), nil
}

func (r *EntContentNodeRepository) GetByIDs(ctx context.Context, ids []string) (map[string]domain.ContentNode, error) {
	result := map[string]domain.ContentNode{}
	if len(ids) == 0 {
		return result, nil
	}

	rows, err := r.client.ContentNode.Query().Where(contentnode.IDIn(parseUUIDsSkippingInvalid(ids)...)).
		WithLanguages().WithSkills().WithConcepts().All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID.String()] = toDomainContentNode(row)
	}
	return result, nil
}

func (r *EntContentNodeRepository) List(ctx context.Context, contentType domain.ContentType, skillID, conceptID string, difficulty domain.DifficultyLevel) ([]domain.ContentNode, error) {
	query := r.client.ContentNode.Query().WithLanguages().WithSkills().WithConcepts()
	if contentType != "" {
		query = query.Where(contentnode.ContentTypeEQ(contentnode.ContentType(contentType)))
	}
	if skillID != "" {
		parsed, err := uuid.Parse(skillID)
		if err != nil {
			return nil, err
		}
		query = query.Where(contentnode.HasSkillsWith(skill.ID(parsed)))
	}
	if conceptID != "" {
		parsed, err := uuid.Parse(conceptID)
		if err != nil {
			return nil, err
		}
		query = query.Where(contentnode.HasConceptsWith(concept.ID(parsed)))
	}
	if difficulty != "" {
		query = query.Where(contentnode.DifficultyLevelEQ(contentnode.DifficultyLevel(difficulty)))
	}

	rows, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.ContentNode, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainContentNode(row))
	}
	return result, nil
}

func (r *EntContentNodeRepository) Update(ctx context.Context, node domain.ContentNode) error {
	id, err := uuid.Parse(node.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	skillIDs, err := parseUUIDs(node.Classification.SkillIDs())
	if err != nil {
		return err
	}
	conceptIDs, err := parseUUIDs(node.Classification.ConceptIDs())
	if err != nil {
		return err
	}
	_, err = r.client.ContentNode.UpdateOneID(id).
		SetTitle(node.Title).
		SetDifficultyLevel(contentnode.DifficultyLevel(node.Classification.DifficultyLevel)).
		ClearSkills().
		AddSkillIDs(skillIDs...).
		ClearConcepts().
		AddConceptIDs(conceptIDs...).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

func toDomainContentNode(row *ent.ContentNode) domain.ContentNode {
	languages := make([]domain.Language, len(row.Edges.Languages))
	for i, lang := range row.Edges.Languages {
		languages[i] = domain.Language{Code: lang.Code, Name: lang.Name}
	}
	return domain.ContentNode{
		ID:          row.ID.String(),
		TeacherID:   row.TeacherID.String(),
		Title:       row.Title,
		ContentType: domain.ContentType(row.ContentType),
		Classification: domain.Classification{
			Skills:          domainSkillsFromEdges(row.Edges.Skills),
			Concepts:        domainConceptsFromEdges(row.Edges.Concepts),
			DifficultyLevel: domain.DifficultyLevel(row.DifficultyLevel),
			ReviewState:     domain.ReviewState(row.ReviewState),
		},
		Languages: languages,
		CreatedAt: row.CreatedAt,
	}
}

// languageIDsByCode resolves each of langs' codes to its Language row id. A
// code that matches no row is silently skipped — the same "not found, simply
// absent" convention ContentNodeRepository.GetByIDs documents — since codes
// are already validated non-empty by the domain layer before reaching here.
func languageIDsByCode(ctx context.Context, languageClient *ent.LanguageClient, langs []domain.Language) ([]uuid.UUID, error) {
	if len(langs) == 0 {
		return nil, nil
	}
	codes := make([]string, len(langs))
	for i, lang := range langs {
		codes[i] = lang.Code
	}
	rows, err := languageClient.Query().Where(language.CodeIn(codes...)).All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids, nil
}
