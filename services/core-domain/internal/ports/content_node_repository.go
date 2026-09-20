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

	// List returns content nodes matching the given filters. An empty
	// filter value means "no filter" on that dimension. skillID/conceptID
	// match a content node whose linked skill_ids/concept_ids contain that
	// exact id — not its ancestors or descendants.
	List(ctx context.Context, contentType domain.ContentType, skillID, conceptID string, difficulty domain.DifficultyLevel) ([]domain.ContentNode, error)

	// Update returns domain.ErrNotFound if no node exists with the given id.
	Update(ctx context.Context, node domain.ContentNode) error
}
