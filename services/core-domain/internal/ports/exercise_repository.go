package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ExerciseRepository persists Exercise records.
type ExerciseRepository interface {
	Create(ctx context.Context, exercise domain.Exercise) error

	// GetByID returns domain.ErrNotFound if no exercise exists with the
	// given id.
	GetByID(ctx context.Context, id string) (domain.Exercise, error)
}
