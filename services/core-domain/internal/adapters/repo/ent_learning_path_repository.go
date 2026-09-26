package repo

import (
	"context"
	"entgo.io/ent/dialect/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnode"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/instrument"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/learningpath"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/learningpathinstrument"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/learningpathitem"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/predicate"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntLearningPathRepository persists LearningPath and LearningPathItem
// records via ent/Postgres. Item titles and content types are not stored
// redundantly on learning_path_items — GetByID joins back to content_nodes
// at read time to denormalise them for the response.
type EntLearningPathRepository struct {
	client *ent.Client
}

func NewEntLearningPathRepository(client *ent.Client) *EntLearningPathRepository {
	return &EntLearningPathRepository{client: client}
}

func (r *EntLearningPathRepository) Create(ctx context.Context, path domain.LearningPath) error {
	id, err := uuid.Parse(path.ID)
	if err != nil {
		return err
	}
	teacherID, err := uuid.Parse(path.TeacherID)
	if err != nil {
		return err
	}
	instrumentIDs, err := parseUUIDs(path.InstrumentIDs)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.LearningPath.Create().
		SetID(id).
		SetTeacherID(teacherID).
		SetTitle(path.Title).
		SetNillableLevel(entLevel(path.Level)).
		SetCreatedAt(path.CreatedAt).
		SetUpdatedAt(path.UpdatedAt).
		SetNillableThumbnailURL(path.ThumbnailURL).
		AddInstrumentIDs(instrumentIDs...).
		Save(ctx); err != nil {
		return rollback(tx, err)
	}

	itemBuilders := make([]*ent.LearningPathItemCreate, len(path.Items))
	for i, item := range path.Items {
		contentNodeID, err := uuid.Parse(item.ContentNodeID)
		if err != nil {
			return rollback(tx, err)
		}
		itemBuilders[i] = tx.LearningPathItem.Create().
			SetID(uuid.New()).
			SetLearningPathID(id).
			SetContentNodeID(contentNodeID).
			SetPosition(item.Position).
			SetNillableSectionLabel(item.SectionLabel)
	}
	if _, err := tx.LearningPathItem.CreateBulk(itemBuilders...).Save(ctx); err != nil {
		return rollback(tx, err)
	}

	return tx.Commit()
}

func (r *EntLearningPathRepository) GetByID(ctx context.Context, id string) (domain.LearningPath, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.LearningPath{}, domain.ErrNotFound
	}

	pathRow, err := r.client.LearningPath.Query().Where(learningpath.ID(parsed)).WithInstruments().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.LearningPath{}, domain.ErrNotFound
		}
		return domain.LearningPath{}, err
	}

	itemRows, err := r.client.LearningPathItem.Query().
		Where(learningpathitem.LearningPathID(parsed)).
		Order(learningpathitem.ByPosition()).
		All(ctx)
	if err != nil {
		return domain.LearningPath{}, err
	}

	nodesByID, err := r.contentNodesForItems(ctx, itemRows)
	if err != nil {
		return domain.LearningPath{}, err
	}

	items, err := buildLearningPathItems(itemRows, nodesByID)
	if err != nil {
		return domain.LearningPath{}, err
	}

	return domain.LearningPath{
		ID:            pathRow.ID.String(),
		TeacherID:     pathRow.TeacherID.String(),
		Title:         pathRow.Title,
		Level:         domainLevel(pathRow.Level),
		InstrumentIDs: instrumentIDsOf(pathRow.Edges.Instruments),
		ThumbnailURL:  pathRow.ThumbnailURL,
		Items:         items,
		CreatedAt:     pathRow.CreatedAt,
		UpdatedAt:     pathRow.UpdatedAt,
	}, nil
}

// List returns every learning path with its items, batching the item and
// content-node lookups into one query each across all paths — instead of
// GetByID's per-path 1 (items) + 1 (nodes) round trips repeated per path.
func (r *EntLearningPathRepository) List(ctx context.Context, filter domain.LearningPathFilter, page domain.PageRequest) (domain.Page[domain.LearningPath], error) {
	predicates, err := learningPathListPredicates(filter)
	if err != nil {
		return domain.Page[domain.LearningPath]{}, err
	}
	query := r.client.LearningPath.Query().Where(predicates...).WithInstruments()
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return domain.Page[domain.LearningPath]{}, err
	}
	pathRows, err := query.
		Order(learningPathListOrder(filter.Sort)...).
		Limit(page.Limit).
		Offset(page.Offset).
		All(ctx)
	if err != nil {
		return domain.Page[domain.LearningPath]{}, err
	}
	if len(pathRows) == 0 {
		return domain.Page[domain.LearningPath]{Items: []domain.LearningPath{}, Total: total}, nil
	}

	pathIDs := make([]uuid.UUID, len(pathRows))
	for i, pathRow := range pathRows {
		pathIDs[i] = pathRow.ID
	}
	itemRows, err := r.client.LearningPathItem.Query().
		Where(learningpathitem.LearningPathIDIn(pathIDs...)).
		Order(learningpathitem.ByPosition()).
		All(ctx)
	if err != nil {
		return domain.Page[domain.LearningPath]{}, err
	}

	nodesByID, err := r.contentNodesForItems(ctx, itemRows)
	if err != nil {
		return domain.Page[domain.LearningPath]{}, err
	}

	// itemRows is sorted by position across all paths; bucketing by
	// learning_path_id below preserves that relative order within each
	// bucket, so no per-path re-sort is needed.
	itemsByPathID := make(map[uuid.UUID][]*ent.LearningPathItem, len(pathRows))
	for _, item := range itemRows {
		itemsByPathID[item.LearningPathID] = append(itemsByPathID[item.LearningPathID], item)
	}

	result := make([]domain.LearningPath, len(pathRows))
	for i, pathRow := range pathRows {
		items, err := buildLearningPathItems(itemsByPathID[pathRow.ID], nodesByID)
		if err != nil {
			return domain.Page[domain.LearningPath]{}, err
		}
		result[i] = domain.LearningPath{
			ID:            pathRow.ID.String(),
			TeacherID:     pathRow.TeacherID.String(),
			Title:         pathRow.Title,
			Level:         domainLevel(pathRow.Level),
			InstrumentIDs: instrumentIDsOf(pathRow.Edges.Instruments),
			ThumbnailURL:  pathRow.ThumbnailURL,
			Items:         items,
			CreatedAt:     pathRow.CreatedAt,
			UpdatedAt:     pathRow.UpdatedAt,
		}
	}
	return domain.Page[domain.LearningPath]{Items: result, Total: total}, nil
}

// contentNodesForItems batch-fetches the content nodes referenced by
// itemRows, keyed by id.
func (r *EntLearningPathRepository) contentNodesForItems(ctx context.Context, itemRows []*ent.LearningPathItem) (map[uuid.UUID]*ent.ContentNode, error) {
	nodeIDs := make([]uuid.UUID, len(itemRows))
	for i, item := range itemRows {
		nodeIDs[i] = item.ContentNodeID
	}
	nodeRows, err := r.client.ContentNode.Query().Where(contentnode.IDIn(nodeIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	nodesByID := make(map[uuid.UUID]*ent.ContentNode, len(nodeRows))
	for _, node := range nodeRows {
		nodesByID[node.ID] = node
	}
	return nodesByID, nil
}

// buildLearningPathItems denormalises Title/ContentType from nodesByID onto
// each of itemRows, shared by GetByID and List.
func buildLearningPathItems(itemRows []*ent.LearningPathItem, nodesByID map[uuid.UUID]*ent.ContentNode) ([]domain.LearningPathItem, error) {
	items := make([]domain.LearningPathItem, len(itemRows))
	for i, item := range itemRows {
		node, ok := nodesByID[item.ContentNodeID]
		if !ok {
			return nil, fmt.Errorf("learning path item %s references missing content node %s", item.ID, item.ContentNodeID)
		}
		items[i] = domain.LearningPathItem{
			Position:      item.Position,
			ContentNodeID: item.ContentNodeID.String(),
			Title:         node.Title,
			ContentType:   domain.ContentType(node.ContentType),
			SectionLabel:  item.SectionLabel,
		}
	}
	return items, nil
}

// Replace deletes path's current items and inserts path.Items in their
// place, in one transaction, then updates the path's own title. Items are
// immutable once created (see LearningPathItem's schema), so a replace is
// expressed as delete-then-recreate rather than a per-item update.
func (r *EntLearningPathRepository) Replace(ctx context.Context, path domain.LearningPath) error {
	id, err := uuid.Parse(path.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	instrumentIDs, err := parseUUIDs(path.InstrumentIDs)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	update := tx.LearningPath.UpdateOneID(id).
		SetTitle(path.Title).
		SetNillableLevel(entLevel(path.Level)).
		SetUpdatedAt(path.UpdatedAt).
		SetNillableThumbnailURL(path.ThumbnailURL).
		ClearInstruments().
		AddInstrumentIDs(instrumentIDs...)
	// A replace without a thumbnail removes the stored one.
	if path.ThumbnailURL == nil {
		update = update.ClearThumbnailURL()
	}
	if _, err := update.Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return rollback(tx, domain.ErrNotFound)
		}
		return rollback(tx, err)
	}

	if _, err := tx.LearningPathItem.Delete().
		Where(learningpathitem.LearningPathID(id)).
		Exec(ctx); err != nil {
		return rollback(tx, err)
	}

	itemBuilders := make([]*ent.LearningPathItemCreate, len(path.Items))
	for i, item := range path.Items {
		contentNodeID, err := uuid.Parse(item.ContentNodeID)
		if err != nil {
			return rollback(tx, err)
		}
		itemBuilders[i] = tx.LearningPathItem.Create().
			SetID(uuid.New()).
			SetLearningPathID(id).
			SetContentNodeID(contentNodeID).
			SetPosition(item.Position).
			SetNillableSectionLabel(item.SectionLabel)
	}
	if len(itemBuilders) > 0 {
		if _, err := tx.LearningPathItem.CreateBulk(itemBuilders...).Save(ctx); err != nil {
			return rollback(tx, err)
		}
	}

	return tx.Commit()
}

// Delete removes id's LearningPath row together with all of its
// LearningPathItem rows, in one transaction. Returns domain.ErrNotFound if
// no path exists with the given id. Neither table carries an ent edge back
// from StudentPath/StudentPathItem — those copy a template's items at
// assign time and never reference learning_paths again — so this never
// needs to cascade or touch student progress data.
func (r *EntLearningPathRepository) Delete(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.LearningPathItem.Delete().
		Where(learningpathitem.LearningPathID(parsed)).
		Exec(ctx); err != nil {
		return rollback(tx, err)
	}
	if _, err := tx.LearningPathInstrument.Delete().
		Where(learningpathinstrument.LearningPathID(parsed)).
		Exec(ctx); err != nil {
		return rollback(tx, err)
	}

	if err := tx.LearningPath.DeleteOneID(parsed).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return rollback(tx, domain.ErrNotFound)
		}
		return rollback(tx, err)
	}

	return tx.Commit()
}

// rollback rolls tx back and folds any rollback failure into the original
// error rather than discarding it silently.
func rollback(tx *ent.Tx, err error) error {
	if rbErr := tx.Rollback(); rbErr != nil {
		return fmt.Errorf("%w (rollback also failed: %v)", err, rbErr)
	}
	return err
}

// domainLevel is a stored path level as the domain's, nil when none is
// recorded.
func domainLevel(level *learningpath.Level) *domain.DifficultyLevel {
	if level == nil {
		return nil
	}
	l := domain.DifficultyLevel(*level)
	return &l
}

// entLevel is a domain path level as ent's, nil when none is recorded.
func entLevel(level *domain.DifficultyLevel) *learningpath.Level {
	if level == nil {
		return nil
	}
	l := learningpath.Level(*level)
	return &l
}

// learningPathListPredicates translates filter into ent predicates, one per
// set field.
func learningPathListPredicates(filter domain.LearningPathFilter) ([]predicate.LearningPath, error) {
	var predicates []predicate.LearningPath
	if filter.Query != "" {
		predicates = append(predicates, learningpath.TitleContainsFold(filter.Query))
	}
	if filter.CreatedBy != "" {
		teacher, err := uuid.Parse(filter.CreatedBy)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, learningpath.TeacherIDEQ(teacher))
	}
	if len(filter.Levels) > 0 {
		levels := make([]learningpath.Level, len(filter.Levels))
		for i, l := range filter.Levels {
			levels[i] = learningpath.Level(l)
		}
		predicates = append(predicates, learningpath.LevelIn(levels...))
	}
	if filter.InstrumentID != "" {
		forInstrument, err := pathForInstrument(filter.InstrumentID)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, forInstrument)
	}
	if len(filter.SkillIDs) > 0 || len(filter.ConceptIDs) > 0 {
		classified, err := pathClassifiedWith(filter.SkillIDs, filter.ConceptIDs)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, classified)
	}
	return predicates, nil
}

// pathForInstrument matches a path for instrumentID, or for every
// instrument (no instruments listed).
func pathForInstrument(instrumentID string) (predicate.LearningPath, error) {
	id, err := uuid.Parse(instrumentID)
	if err != nil {
		return nil, err
	}
	return learningpath.Or(
		learningpath.HasInstrumentsWith(instrument.ID(id)),
		learningpath.Not(learningpath.HasInstruments()),
	), nil
}

// pathClassifiedWith parses the skill and concept ids and matches paths
// through classifiedItemMatches.
func pathClassifiedWith(skills, concepts []string) (predicate.LearningPath, error) {
	skillIDs, err := parseUUIDs(skills)
	if err != nil {
		return nil, err
	}
	conceptIDs, err := parseUUIDs(concepts)
	if err != nil {
		return nil, err
	}
	return classifiedItemMatches(skillIDs, conceptIDs), nil
}

// classifiedItemMatches matches a path when one of its items' content nodes
// is linked to any of skillIDs and, where both are given, also to any of
// conceptIDs, the same rule the course filters apply to checkpoints.
func classifiedItemMatches(skillIDs, conceptIDs []uuid.UUID) predicate.LearningPath {
	return predicate.LearningPath(func(s *sql.Selector) {
		s.Where(sql.P(func(b *sql.Builder) {
			b.WriteString("EXISTS (SELECT 1 FROM learning_path_items lpi WHERE lpi.learning_path_id = " + s.C(learningpath.FieldID))
			writeNodeLinkedToAny(b, "content_node_skills", "skill_id", skillIDs)
			writeNodeLinkedToAny(b, "content_node_concepts", "concept_id", conceptIDs)
			b.WriteString(")")
		}))
	})
}

// learningPathListOrder orders by title, or by most recent update first when
// sort asks for it; id breaks ties either way.
func learningPathListOrder(sort domain.LearningPathSort) []learningpath.OrderOption {
	if sort == domain.LearningPathSortUpdated {
		return []learningpath.OrderOption{learningpath.ByUpdatedAt(sql.OrderDesc()), learningpath.ByID()}
	}
	return []learningpath.OrderOption{learningpath.ByTitle(), learningpath.ByID()}
}
