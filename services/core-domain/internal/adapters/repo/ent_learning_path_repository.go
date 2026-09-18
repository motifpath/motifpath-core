package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentnode"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/learningpathitem"
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

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.LearningPath.Create().
		SetID(id).
		SetTeacherID(teacherID).
		SetTitle(path.Title).
		SetCreatedAt(path.CreatedAt).
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

	pathRow, err := r.client.LearningPath.Get(ctx, parsed)
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

	nodeIDs := make([]uuid.UUID, len(itemRows))
	for i, item := range itemRows {
		nodeIDs[i] = item.ContentNodeID
	}
	nodeRows, err := r.client.ContentNode.Query().Where(contentnode.IDIn(nodeIDs...)).All(ctx)
	if err != nil {
		return domain.LearningPath{}, err
	}
	nodesByID := make(map[uuid.UUID]*ent.ContentNode, len(nodeRows))
	for _, node := range nodeRows {
		nodesByID[node.ID] = node
	}

	items := make([]domain.LearningPathItem, len(itemRows))
	for i, item := range itemRows {
		node, ok := nodesByID[item.ContentNodeID]
		if !ok {
			return domain.LearningPath{}, fmt.Errorf("learning path item %s references missing content node %s", item.ID, item.ContentNodeID)
		}
		items[i] = domain.LearningPathItem{
			Position:      item.Position,
			ContentNodeID: item.ContentNodeID.String(),
			Title:         node.Title,
			ContentType:   domain.ContentType(node.ContentType),
			SectionLabel:  item.SectionLabel,
		}
	}

	return domain.LearningPath{
		ID:        pathRow.ID.String(),
		TeacherID: pathRow.TeacherID.String(),
		Title:     pathRow.Title,
		Items:     items,
		CreatedAt: pathRow.CreatedAt,
	}, nil
}

func (r *EntLearningPathRepository) List(ctx context.Context) ([]domain.LearningPath, error) {
	pathRows, err := r.client.LearningPath.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]domain.LearningPath, 0, len(pathRows))
	for _, pathRow := range pathRows {
		path, err := r.GetByID(ctx, pathRow.ID.String())
		if err != nil {
			return nil, err
		}
		result = append(result, path)
	}
	return result, nil
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

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.LearningPath.UpdateOneID(id).
		SetTitle(path.Title).
		Save(ctx); err != nil {
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

// rollback rolls tx back and folds any rollback failure into the original
// error rather than discarding it silently.
func rollback(tx *ent.Tx, err error) error {
	if rbErr := tx.Rollback(); rbErr != nil {
		return fmt.Errorf("%w (rollback also failed: %v)", err, rbErr)
	}
	return err
}
