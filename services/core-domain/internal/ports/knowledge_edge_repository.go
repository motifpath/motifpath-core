package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// KnowledgeEdgeFilter narrows KnowledgeEdgeRepository.List; a nil field
// matches every edge.
type KnowledgeEdgeFilter struct {
	Type   *domain.KnowledgeEdgeType
	FromID *string
	ToID   *string
}

// KnowledgeEdgeRepository persists the typed links between knowledge
// nodes.
type KnowledgeEdgeRepository interface {
	// Create returns domain.ErrAlreadyExists if an edge of edge.Type
	// already links edge.FromID to edge.ToID.
	Create(ctx context.Context, edge domain.KnowledgeEdge) error

	// GetByID returns domain.ErrNotFound if no edge exists with id.
	GetByID(ctx context.Context, id string) (domain.KnowledgeEdge, error)

	// List returns the edges matching filter, in a stable order.
	List(ctx context.Context, filter KnowledgeEdgeFilter) ([]domain.KnowledgeEdge, error)

	// UpdateLevel returns domain.ErrNotFound if no edge exists with
	// edge.ID.
	UpdateLevel(ctx context.Context, edge domain.KnowledgeEdge) error

	// Delete returns domain.ErrNotFound if no edge exists with id.
	Delete(ctx context.Context, id string) error

	// RequiresPathExists reports whether following requires edges from
	// fromID reaches toID.
	RequiresPathExists(ctx context.Context, fromID, toID string) (bool, error)
}
