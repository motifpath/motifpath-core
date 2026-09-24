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

	// List returns one page of the diagrams matching filter, ordered by
	// name then id, with the count of all matches across pages.
	List(ctx context.Context, filter domain.DiagramListFilter, page domain.PageRequest) (domain.Page[domain.Diagram], error)

	// Update replaces the diagram's name, positions and classification. It
	// returns domain.ErrNotFound if no diagram exists with the given id.
	// InstrumentID, Kind and CreatedBy are never changed.
	Update(ctx context.Context, diagram domain.Diagram) error
}
