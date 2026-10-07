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

	// GetByIDs returns the diagrams with these ids, as GetByID would, keyed
	// by id; an id with no diagram is left out.
	GetByIDs(ctx context.Context, ids []string) (map[string]domain.Diagram, error)

	// List returns one page of the diagrams matching filter, ordered by
	// name then id, with the count of all matches across pages.
	List(ctx context.Context, filter domain.DiagramListFilter, page domain.PageRequest) (domain.Page[domain.Diagram], error)

	// ListCreatorIDs returns the distinct creator user ids of every diagram
	// matching filter, using the same predicates as List, in no set order.
	ListCreatorIDs(ctx context.Context, filter domain.DiagramListFilter) ([]string, error)

	// Update replaces the diagram's name, positions and classification. It
	// returns domain.ErrNotFound if no diagram exists with the given id.
	// InstrumentID, Kind and CreatedBy are never changed.
	Update(ctx context.Context, diagram domain.Diagram) error
}
