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

	// List returns one page of the learning paths matching filter, ordered
	// by title then id, with the count of all matches across pages.
	List(ctx context.Context, filter domain.LearningPathFilter, page domain.PageRequest) (domain.Page[domain.LearningPath], error)

	// Replace replaces path's title and items wholesale — its current items
	// are deleted and path.Items inserted in their place, in one
	// transaction. Returns domain.ErrNotFound if no path exists with the
	// given id.
	Replace(ctx context.Context, path domain.LearningPath) error

	// Delete permanently removes the learning path with the given id and
	// all of its items, in one transaction. Never touches any StudentPath
	// already copied from it — those are independent snapshots. Returns
	// domain.ErrNotFound if no path exists with the given id.
	Delete(ctx context.Context, id string) error
}
