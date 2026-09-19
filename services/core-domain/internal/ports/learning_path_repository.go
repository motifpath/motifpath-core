package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// LearningPathRepository persists LearningPath records.
type LearningPathRepository interface {
	Create(ctx context.Context, path domain.LearningPath) error

	// GetByID returns domain.ErrNotFound if no path exists with the given id.
	GetByID(ctx context.Context, id string) (domain.LearningPath, error)

	// List returns every learning path in the library.
	List(ctx context.Context) ([]domain.LearningPath, error)

	// Replace replaces path's title and items wholesale — its current items
	// are deleted and path.Items inserted in their place, in one
	// transaction. Returns domain.ErrNotFound if no path exists with the
	// given id.
	Replace(ctx context.Context, path domain.LearningPath) error
}
