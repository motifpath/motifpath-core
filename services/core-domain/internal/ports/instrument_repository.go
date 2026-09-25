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

	// UpdateNames replaces the names of the instrument with the given id.
	// Returns domain.ErrNotFound if no instrument exists with that id.
	UpdateNames(ctx context.Context, id string, names domain.LocalizedText) error
}
