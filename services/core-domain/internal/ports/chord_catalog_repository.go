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
	// GetChords returns the chords with these ids, keyed by id; an id with
	// no chord is left out.
	GetChords(ctx context.Context, ids []string) (map[string]domain.ChordDefinition, error)
	// GetVoicings returns the voicings with these ids, withdrawn ones
	// included, keyed by id; an id with no voicing is left out.
	GetVoicings(ctx context.Context, ids []string) (map[string]domain.ChordVoicing, error)
	// GetVoicingByDiagramID returns the voicing, withdrawn or not, whose
	// fingering is the diagram with diagramID, or domain.ErrNotFound. A
	// diagram is the fingering of at most one voicing.
	GetVoicingByDiagramID(ctx context.Context, diagramID string) (domain.ChordVoicing, error)
}
