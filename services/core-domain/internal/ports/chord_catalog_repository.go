package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ChordCatalogRepository reads the chords and voicings the chord catalog's
// migrations install. Every chord it returns carries its active voicings,
// best first (by recommended rank, then id).
type ChordCatalogRepository interface {
	// GetChord returns the chord with id, or domain.ErrNotFound.
	GetChord(ctx context.Context, id string) (domain.ChordDefinition, error)
	// FindChord returns the chord with this root pitch class, quality and
	// bass pitch class (nil: no slash bass), or domain.ErrNotFound.
	FindChord(ctx context.Context, rootPitchClass int, quality domain.ChordQuality, bassPitchClass *int) (domain.ChordDefinition, error)
}
