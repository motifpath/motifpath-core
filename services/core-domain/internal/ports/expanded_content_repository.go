package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ExpandedContentRepository persists ExpandedContent records.
type ExpandedContentRepository interface {
	Create(ctx context.Context, item domain.ExpandedContent) error

	// GetByID returns domain.ErrNotFound if no item exists with the given id.
	GetByID(ctx context.Context, id string) (domain.ExpandedContent, error)

	// ListByContentNode returns all items attached to contentNodeID,
	// ordered by trigger position ascending (trigger_at_seconds for video
	// nodes, trigger_at_paragraph for article nodes).
	ListByContentNode(ctx context.Context, contentNodeID string) ([]domain.ExpandedContent, error)
}
