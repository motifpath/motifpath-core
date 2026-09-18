package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnode"
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
	_, err = r.client.ContentNode.Create().
		SetID(id).
		SetTeacherID(teacherID).
		SetTitle(node.Title).
		SetContentType(contentnode.ContentType(node.ContentType)).
		SetSkill(node.Classification.Skill).
		SetConcept(node.Classification.Concept).
		SetDifficultyLevel(contentnode.DifficultyLevel(node.Classification.DifficultyLevel)).
		SetReviewState(contentnode.ReviewState(node.Classification.ReviewState)).
		SetCreatedAt(node.CreatedAt).
		Save(ctx)
	return err
}

func (r *EntContentNodeRepository) GetByID(ctx context.Context, id string) (domain.ContentNode, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ContentNode{}, domain.ErrNotFound
	}
	row, err := r.client.ContentNode.Get(ctx, parsed)
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

	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		u, err := uuid.Parse(id)
		if err != nil {
			continue // not a valid id, so it can never match — left absent from result
		}
		parsed = append(parsed, u)
	}

	rows, err := r.client.ContentNode.Query().Where(contentnode.IDIn(parsed...)).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID.String()] = toDomainContentNode(row)
	}
	return result, nil
}

func (r *EntContentNodeRepository) List(ctx context.Context, contentType domain.ContentType, skill string, difficulty domain.DifficultyLevel) ([]domain.ContentNode, error) {
	query := r.client.ContentNode.Query()
	if contentType != "" {
		query = query.Where(contentnode.ContentTypeEQ(contentnode.ContentType(contentType)))
	}
	if skill != "" {
		query = query.Where(contentnode.SkillEQ(skill))
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
	_, err = r.client.ContentNode.UpdateOneID(id).
		SetTitle(node.Title).
		SetSkill(node.Classification.Skill).
		SetConcept(node.Classification.Concept).
		SetDifficultyLevel(contentnode.DifficultyLevel(node.Classification.DifficultyLevel)).
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
	return domain.ContentNode{
		ID:          row.ID.String(),
		TeacherID:   row.TeacherID.String(),
		Title:       row.Title,
		ContentType: domain.ContentType(row.ContentType),
		Classification: domain.Classification{
			Skill:           row.Skill,
			Concept:         row.Concept,
			DifficultyLevel: domain.DifficultyLevel(row.DifficultyLevel),
			ReviewState:     domain.ReviewState(row.ReviewState),
		},
		CreatedAt: row.CreatedAt,
	}
}
