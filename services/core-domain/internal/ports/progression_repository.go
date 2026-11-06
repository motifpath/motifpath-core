package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// DrillProgressions reads the progressions the drills use. Drills depend on this
// and not on ProgressionRepository, so a progression for song charts only can't
// reach a learner by mistake.
type DrillProgressions interface {
	// ListInDrills returns the progressions the drills use, by key.
	ListInDrills(ctx context.Context) ([]domain.Progression, error)
}

// ProgressionRepository keeps every progression, in the drills or for song
// charts only.
type ProgressionRepository interface {
	DrillProgressions
	// List returns every progression, by key.
	List(ctx context.Context) ([]domain.Progression, error)
}
