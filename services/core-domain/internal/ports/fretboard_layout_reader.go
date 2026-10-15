package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// FretboardLayoutReader finds the fretboard layouts of the practice drill
// catalog an instrument plays on: the layout instruments it shares its
// geometry with.
type FretboardLayoutReader interface {
	// FretboardLayouts lists the layouts instrumentID shares its geometry
	// with; none for an instrument with no fretboard layout, or one that
	// doesn't exist.
	FretboardLayouts(ctx context.Context, instrumentID string) ([]domain.Instrument, error)
}
