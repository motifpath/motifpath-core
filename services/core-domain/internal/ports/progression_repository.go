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

// ProgressionLister reads every progression, in the drills or for song charts
// only.
type ProgressionLister interface {
	// List returns every progression, by key.
	List(ctx context.Context) ([]domain.Progression, error)
}

// ProgressionRepository keeps every progression.
type ProgressionRepository interface {
	DrillProgressions
	ProgressionLister
	// GetByID returns the progression with id, or domain.ErrNotFound.
	GetByID(ctx context.Context, id string) (domain.Progression, error)
	// FindByNumerals returns the progression with this mode and these bars, or
	// domain.ErrNotFound.
	FindByNumerals(ctx context.Context, mode string, bars []string) (domain.Progression, error)
	// Create stores a new progression, or returns domain.ErrConflict when its
	// mode and bars or its key are taken.
	Create(ctx context.Context, progression domain.Progression) error
	// SaveNames replaces a progression's nicknames and names.
	SaveNames(ctx context.Context, progression domain.Progression) error
	// Delete removes the progression with id.
	Delete(ctx context.Context, id string) error
}

// ProgressionUseReader finds the song chart sections that tag progressions.
type ProgressionUseReader interface {
	// ProgressionUses returns every section tagged in a chart's draft or latest
	// published revision, in no set order.
	ProgressionUses(ctx context.Context) ([]domain.ProgressionUse, error)
}
