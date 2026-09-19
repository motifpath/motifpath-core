package repo

import (
	"context"
	"encoding/json"

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
	richContentJSON, err := marshalRichContent(item.RichContent)
	if err != nil {
		return err
	}

	builder := r.client.ExpandedContent.Create().
		SetID(id).
		SetContentNodeID(contentNodeID).
		SetContentType(expandedcontent.ContentType(item.ContentType)).
		SetNillableMediaURL(item.MediaURL).
		SetNillableRichContent(richContentJSON).
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

func (r *EntExpandedContentRepository) Update(ctx context.Context, item domain.ExpandedContent) error {
	id, err := uuid.Parse(item.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	richContentJSON, err := marshalRichContent(item.RichContent)
	if err != nil {
		return err
	}

	builder := r.client.ExpandedContent.UpdateOneID(id).
		SetContentType(expandedcontent.ContentType(item.ContentType)).
		SetNillableTriggerAtSeconds(item.TriggerAtSeconds).
		SetNillableHideAtSeconds(item.HideAtSeconds).
		SetNillableTriggerAtParagraph(item.TriggerAtParagraph).
		SetNillableDurationMs(item.DurationMS).
		SetNillableCaption(item.Caption)

	if item.MediaURL != nil {
		builder = builder.SetMediaURL(*item.MediaURL)
	} else {
		builder = builder.ClearMediaURL()
	}
	if richContentJSON != nil {
		builder = builder.SetRichContent(*richContentJSON)
	} else {
		builder = builder.ClearRichContent()
	}
	if item.TriggerAtSeconds == nil {
		builder = builder.ClearTriggerAtSeconds()
	}
	if item.HideAtSeconds == nil {
		builder = builder.ClearHideAtSeconds()
	}
	if item.TriggerAtParagraph == nil {
		builder = builder.ClearTriggerAtParagraph()
	}
	if item.DurationMS == nil {
		builder = builder.ClearDurationMs()
	}
	if item.Caption == nil {
		builder = builder.ClearCaption()
	}

	_, err = builder.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *EntExpandedContentRepository) Delete(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrNotFound
	}
	err = r.client.ExpandedContent.DeleteOneID(parsed).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

func toDomainExpandedContent(row *ent.ExpandedContent) domain.ExpandedContent {
	return domain.ExpandedContent{
		ID:                 row.ID.String(),
		ContentNodeID:      row.ContentNodeID.String(),
		ContentType:        domain.ExpandedContentType(row.ContentType),
		MediaURL:           row.MediaURL,
		RichContent:        unmarshalRichContent(row.RichContent),
		TriggerAtSeconds:   row.TriggerAtSeconds,
		HideAtSeconds:      row.HideAtSeconds,
		TriggerAtParagraph: row.TriggerAtParagraph,
		DurationMS:         row.DurationMs,
		Caption:            row.Caption,
		CreatedAt:          row.CreatedAt,
	}
}

// marshalRichContent serializes content to the JSON text stored in the
// expanded_contents table's rich_content column. A nil content marshals to
// nil (column left unset) — the column is only ever populated for rich_text
// items.
func marshalRichContent(content *domain.PromptDocument) (*string, error) {
	if content == nil {
		return nil, nil
	}
	data, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	s := string(data)
	return &s, nil
}

// unmarshalRichContent parses the expanded_contents table's rich_content
// column back into a *domain.PromptDocument. A nil column (image/gif items
// never populate it) or malformed JSON both yield nil rather than an error.
func unmarshalRichContent(stored *string) *domain.PromptDocument {
	if stored == nil {
		return nil
	}
	var content domain.PromptDocument
	if err := json.Unmarshal([]byte(*stored), &content); err != nil {
		return nil
	}
	return &content
}
