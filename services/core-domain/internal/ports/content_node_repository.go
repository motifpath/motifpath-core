package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ContentNodeRepository persists ContentNode records.
type ContentNodeRepository interface {
	Create(ctx context.Context, node domain.ContentNode) error

	// GetByID returns domain.ErrNotFound if no node exists with the given id.
	GetByID(ctx context.Context, id string) (domain.ContentNode, error)

	// GetByIDs returns the content nodes matching ids, keyed by ID. An id
	// with no matching node is simply absent from the result — callers
	// detect "doesn't exist" by checking for the key, which is exactly the
	// information CreateLearningPath's 400 response needs to name the
	// missing content_node_id.
	GetByIDs(ctx context.Context, ids []string) (map[string]domain.ContentNode, error)
}
