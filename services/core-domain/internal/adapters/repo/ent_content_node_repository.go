package repo

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/challenge"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnode"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/exercise"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/instrument"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/knowledgenode"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/language"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/predicate"
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
	richContentJSON, err := marshalRichContent(node.RichContent)
	if err != nil {
		return err
	}
	instrumentIDs, err := parseUUIDs(node.InstrumentIDs)
	if err != nil {
		return err
	}
	_, err = r.client.ContentNode.Create().
		SetID(id).
		SetTeacherID(teacherID).
		SetTitle(node.Title).
		SetContentType(contentnode.ContentType(node.ContentType)).
		SetNillableMediaURL(node.MediaURL).
		SetNillableRichContent(richContentJSON).
		SetDifficultyLevel(contentnode.DifficultyLevel(node.Classification.DifficultyLevel)).
		SetReviewState(contentnode.ReviewState(node.Classification.ReviewState)).
		SetCreatedAt(node.CreatedAt).
		SetUpdatedAt(updatedAtOr(node.UpdatedAt, node.CreatedAt)).
		SetNillableLatestPublishedVersion(node.LatestPublishedVersion).
		SetHasUnpublishedChanges(node.HasUnpublishedChanges).
		AddLanguageIDs(langIDs...).
		AddSkillIDs(skillIDs...).
		AddConceptIDs(conceptIDs...).
		AddInstrumentIDs(instrumentIDs...).
		SetNillableThumbnailURL(node.ThumbnailURL).
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
		WithSkills(withNodeInstruments).
		WithConcepts(withNodeInstruments).
		WithInstruments().
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
		WithLanguages().WithSkills(withNodeInstruments).WithConcepts(withNodeInstruments).WithInstruments().All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID.String()] = toDomainContentNode(row)
	}
	return result, nil
}

func (r *EntContentNodeRepository) List(ctx context.Context, filter domain.ContentNodeFilter, page domain.PageRequest) (domain.Page[domain.ContentNode], error) {
	predicates, err := contentNodePredicates(filter)
	if err != nil {
		return domain.Page[domain.ContentNode]{}, err
	}
	query := r.client.ContentNode.Query().Where(predicates...)

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return domain.Page[domain.ContentNode]{}, err
	}
	rows, err := query.
		WithLanguages().WithSkills(withNodeInstruments).WithConcepts(withNodeInstruments).WithInstruments().
		Order(contentNodeOrder(filter.Sort)...).
		Limit(page.Limit).
		Offset(page.Offset).
		All(ctx)
	if err != nil {
		return domain.Page[domain.ContentNode]{}, err
	}
	items := make([]domain.ContentNode, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainContentNode(row))
	}
	return domain.Page[domain.ContentNode]{Items: items, Total: total}, nil
}

// contentNodePredicates translates filter into query predicates; a
// zero-valued field adds none.
func contentNodePredicates(filter domain.ContentNodeFilter) ([]predicate.ContentNode, error) {
	var predicates []predicate.ContentNode
	if filter.ContentType != "" {
		predicates = append(predicates, contentnode.ContentTypeEQ(contentnode.ContentType(filter.ContentType)))
	}
	if len(filter.SkillIDs) > 0 {
		predicates = append(predicates, contentnode.HasSkillsWith(knowledgenode.IDIn(parseUUIDsSkippingInvalid(filter.SkillIDs)...)))
	}
	if len(filter.ConceptIDs) > 0 {
		predicates = append(predicates, contentnode.HasConceptsWith(knowledgenode.IDIn(parseUUIDsSkippingInvalid(filter.ConceptIDs)...)))
	}
	if filter.CreatedBy != "" {
		parsed, err := uuid.Parse(filter.CreatedBy)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, contentnode.TeacherID(parsed))
	}
	if filter.Language != "" {
		predicates = append(predicates, contentnode.HasLanguagesWith(language.Code(filter.Language)))
	}
	switch filter.Status {
	case domain.ContentNodeStatusDraft:
		predicates = append(predicates, contentnode.LatestPublishedVersionIsNil())
	case domain.ContentNodeStatusPublished:
		predicates = append(predicates, contentnode.LatestPublishedVersionNotNil(), contentnode.HasUnpublishedChanges(false))
	case domain.ContentNodeStatusUnpublishedChanges:
		predicates = append(predicates, contentnode.LatestPublishedVersionNotNil(), contentnode.HasUnpublishedChanges(true))
	}
	if filter.Difficulty != "" {
		predicates = append(predicates, contentnode.DifficultyLevelEQ(contentnode.DifficultyLevel(filter.Difficulty)))
	}
	if filter.Query != "" {
		predicates = append(predicates, contentnode.TitleContainsFold(filter.Query))
	}
	if filter.InstrumentID != "" {
		parsed, err := uuid.Parse(filter.InstrumentID)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, contentnode.Or(
			contentnode.HasInstrumentsWith(instrument.ID(parsed)),
			contentnode.Not(contentnode.HasInstruments()),
		))
	}
	return predicates, nil
}

// contentNodeOrder is the order a listing in sort is returned in.
func contentNodeOrder(sort domain.ListSort) []contentnode.OrderOption {
	if sort == domain.ListSortUpdated {
		return []contentnode.OrderOption{contentnode.ByUpdatedAt(sql.OrderDesc()), contentnode.ByID()}
	}
	return []contentnode.OrderOption{contentnode.ByTitle(), contentnode.ByID()}
}

func (r *EntContentNodeRepository) ListCreatorCounts(ctx context.Context) ([]domain.CreatorCount, error) {
	var rows []struct {
		TeacherID uuid.UUID `json:"teacher_id"`
		Count     int       `json:"count"`
	}
	if err := r.client.ContentNode.Query().
		GroupBy(contentnode.FieldTeacherID).
		Aggregate(ent.Count()).
		Scan(ctx, &rows); err != nil {
		return nil, err
	}
	counts := make([]domain.CreatorCount, len(rows))
	for i, row := range rows {
		counts[i] = domain.CreatorCount{UserID: row.TeacherID.String(), Count: row.Count}
	}
	return counts, nil
}

func (r *EntContentNodeRepository) MarkPublished(ctx context.Context, id string, version int) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	err = r.client.ContentNode.UpdateOneID(parsed).
		SetLatestPublishedVersion(version).
		SetHasUnpublishedChanges(false).
		Exec(ctx)
	if ent.IsNotFound(err) {
		return domain.ErrNotFound
	}
	return err
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
	langIDs, err := languageIDsByCode(ctx, r.client.Language, node.Languages)
	if err != nil {
		return err
	}
	richContentJSON, err := marshalRichContent(node.RichContent)
	if err != nil {
		return err
	}
	instrumentIDs, err := parseUUIDs(node.InstrumentIDs)
	if err != nil {
		return err
	}
	update := r.client.ContentNode.UpdateOneID(id).
		SetTitle(node.Title).
		SetUpdatedAt(updatedAtOr(node.UpdatedAt, node.CreatedAt)).
		SetHasUnpublishedChanges(node.HasUnpublishedChanges).
		SetNillableMediaURL(node.MediaURL).
		SetNillableRichContent(richContentJSON).
		SetDifficultyLevel(contentnode.DifficultyLevel(node.Classification.DifficultyLevel)).
		ClearSkills().
		AddSkillIDs(skillIDs...).
		ClearConcepts().
		AddConceptIDs(conceptIDs...).
		ClearLanguages().
		AddLanguageIDs(langIDs...).
		ClearInstruments().
		AddInstrumentIDs(instrumentIDs...)
	// A nil body field means the caller cleared it (e.g. the other content
	// type's field), so the stored column must be cleared too — the Nillable
	// setters alone would leave a stale value in place.
	if node.MediaURL == nil {
		update = update.ClearMediaURL()
	}
	if richContentJSON == nil {
		update = update.ClearRichContent()
	}
	update = update.SetNillableThumbnailURL(node.ThumbnailURL)
	if node.ThumbnailURL == nil {
		update = update.ClearThumbnailURL()
	}
	_, err = update.Save(ctx)
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
			Skills:          domainKnowledgeNodesFromEdges(row.Edges.Skills),
			Concepts:        domainKnowledgeNodesFromEdges(row.Edges.Concepts),
			DifficultyLevel: domain.DifficultyLevel(row.DifficultyLevel),
			ReviewState:     domain.ReviewState(row.ReviewState),
		},
		MediaURL:      row.MediaURL,
		RichContent:   unmarshalRichContent(row.RichContent),
		Languages:     languages,
		InstrumentIDs: instrumentIDsOf(row.Edges.Instruments),
		ThumbnailURL:  row.ThumbnailURL,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,

		LatestPublishedVersion: row.LatestPublishedVersion,
		HasUnpublishedChanges:  row.HasUnpublishedChanges,
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

func (r *EntContentNodeRepository) LinkedExerciseInstrumentSets(ctx context.Context, id string) ([][]string, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	rows, err := r.client.Exercise.Query().
		Where(exercise.Or(
			exercise.HasContentNodesWith(contentnode.ID(parsed)),
			exercise.HasChallengesWith(challenge.ContentNodeID(parsed)),
		)).
		WithInstruments().
		All(ctx)
	if err != nil {
		return nil, err
	}
	sets := make([][]string, 0, len(rows))
	for _, row := range rows {
		sets = append(sets, instrumentIDsOf(row.Edges.Instruments))
	}
	return sets, nil
}
