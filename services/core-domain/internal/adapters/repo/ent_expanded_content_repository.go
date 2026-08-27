package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/expandedcontent"
	"github.com/motifpath/core-domain/internal/domain"
)

// EntExpandedContentRepository persists ExpandedContent records via
// ent/Postgres.
type EntExpandedContentRepository struct {
	client *ent.Client
}

func NewEntExpandedContentRepository(client *ent.Client) *EntExpandedContentRepository {
	return &EntExpandedContentRepository{client: client}
}

func (r *EntExpandedContentRepository) Create(ctx context.Context, item domain.ExpandedContent) error {
	id, err := uuid.Parse(item.ID)
	if err != nil {
		return err
	}
	contentNodeID, err := uuid.Parse(item.ContentNodeID)
	if err != nil {
		return err
	}

	builder := r.client.ExpandedContent.Create().
		SetID(id).
		SetContentNodeID(contentNodeID).
		SetContentType(expandedcontent.ContentType(item.ContentType)).
		SetMediaURL(item.MediaURL).
		SetNillableTriggerAtSeconds(item.TriggerAtSeconds).
		SetNillableHideAtSeconds(item.HideAtSeconds).
		SetNillableTriggerAtParagraph(item.TriggerAtParagraph).
		SetNillableDurationMs(item.DurationMS).
		SetNillableCaption(item.Caption).
		SetCreatedAt(item.CreatedAt)

	_, err = builder.Save(ctx)
	return err
}

func (r *EntExpandedContentRepository) GetByID(ctx context.Context, id string) (domain.ExpandedContent, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ExpandedContent{}, domain.ErrNotFound
	}
	row, err := r.client.ExpandedContent.Get(ctx, parsed)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ExpandedContent{}, domain.ErrNotFound
		}
		return domain.ExpandedContent{}, err
	}
	return toDomainExpandedContent(row), nil
}

// ListByContentNode orders by trigger position ascending. Every row for a
// given content node shares the same parent type, so exactly one of
// trigger_at_seconds/trigger_at_paragraph is ever populated per row — never
// both, never mixed within one node's items (enforced by
// domain.NewExpandedContent's video/article XOR). Ordering by both
// ascending, with Postgres's default NULLS LAST for ascending order, sorts
// correctly for either parent type without needing to know which one it is.
func (r *EntExpandedContentRepository) ListByContentNode(ctx context.Context, contentNodeID string) ([]domain.ExpandedContent, error) {
	parsed, err := uuid.Parse(contentNodeID)
	if err != nil {
		return nil, nil
	}
	rows, err := r.client.ExpandedContent.Query().
		Where(expandedcontent.ContentNodeID(parsed)).
		Order(expandedcontent.ByTriggerAtSeconds(), expandedcontent.ByTriggerAtParagraph()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.ExpandedContent, len(rows))
	for i, row := range rows {
		items[i] = toDomainExpandedContent(row)
	}
	return items, nil
}

func toDomainExpandedContent(row *ent.ExpandedContent) domain.ExpandedContent {
	return domain.ExpandedContent{
		ID:                 row.ID.String(),
		ContentNodeID:      row.ContentNodeID.String(),
		ContentType:        domain.ExpandedContentType(row.ContentType),
		MediaURL:           row.MediaURL,
		TriggerAtSeconds:   row.TriggerAtSeconds,
		HideAtSeconds:      row.HideAtSeconds,
		TriggerAtParagraph: row.TriggerAtParagraph,
		DurationMS:         row.DurationMs,
		Caption:            row.Caption,
		CreatedAt:          row.CreatedAt,
	}
}
