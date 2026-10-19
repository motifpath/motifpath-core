package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ProgressionRepository reads the installed progression catalog.
type ProgressionRepository interface {
	// List returns every progression, by key.
	List(ctx context.Context) ([]domain.Progression, error)
}
