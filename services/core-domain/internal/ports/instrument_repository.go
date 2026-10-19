package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// InstrumentRepository persists Instrument records.
type InstrumentRepository interface {
	Create(ctx context.Context, instrument domain.Instrument) error

	// GetByID returns domain.ErrNotFound if no instrument exists with the
	// given id.
	GetByID(ctx context.Context, id string) (domain.Instrument, error)

	// List returns every known instrument in a stable id order.
	List(ctx context.Context) ([]domain.Instrument, error)

	// Update replaces the names and default voice of the instrument with
	// instrument's id — the only parts of an instrument that can change.
	// Returns domain.ErrNotFound if no instrument exists with that id.
	Update(ctx context.Context, instrument domain.Instrument) error
}

// VoiceRepository reads the platform's Voices, which are provided with the
// platform and never written through the service.
type VoiceRepository interface {
	// GetByID returns domain.ErrNotFound if no voice exists with the given
	// id.
	GetByID(ctx context.Context, id string) (domain.Voice, error)

	// List returns every voice in id order.
	List(ctx context.Context) ([]domain.Voice, error)
}
