package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ContentNodeVersionRepository persists ContentNodeVersion snapshots —
// each an immutable record of a content node's published state at a point
// in time.
type ContentNodeVersionRepository interface {
	Create(ctx context.Context, version domain.ContentNodeVersion) error

	// GetLatestByContentNodeID returns the highest-version_number
	// ContentNodeVersion for contentNodeID. Returns domain.ErrNotFound if
	// the node has never been published.
	GetLatestByContentNodeID(ctx context.Context, contentNodeID string) (domain.ContentNodeVersion, error)

	// ListByContentNodeID returns every version of contentNodeID, newest
	// first, or an empty slice if it has never been published. Does not
	// itself verify the content node exists.
	ListByContentNodeID(ctx context.Context, contentNodeID string) ([]domain.ContentNodeVersion, error)
}
