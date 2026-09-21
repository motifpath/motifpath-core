package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// DiagramRepository persists Diagram records together with their Positions.
type DiagramRepository interface {
	Create(ctx context.Context, diagram domain.Diagram) error

	// GetByID returns domain.ErrNotFound if no diagram exists with the given
	// id. Skills and Concepts come back fully populated, not id-only.
	GetByID(ctx context.Context, id string) (domain.Diagram, error)

	// List returns diagrams in a stable id order. An empty filter value
	// means "no filter" on that dimension. skillID/conceptID match a
	// diagram whose linked skills/concepts contain that exact id — not its
	// ancestors or descendants.
	List(ctx context.Context, instrumentID, skillID, conceptID string) ([]domain.Diagram, error)

	// Update replaces the diagram's name, positions and classification. It
	// returns domain.ErrNotFound if no diagram exists with the given id.
	// InstrumentID is never changed.
	Update(ctx context.Context, diagram domain.Diagram) error
}
