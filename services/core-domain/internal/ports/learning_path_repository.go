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
}
