package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ContentNodeRepository persists ContentNode records.
type ContentNodeRepository interface {
	Create(ctx context.Context, node domain.ContentNode) error

	// GetByID returns domain.ErrNotFound if no node exists with the given id.
	GetByID(ctx context.Context, id string) (domain.ContentNode, error)

	// GetByIDs returns the content nodes matching ids, keyed by ID. An id
	// with no matching node is simply absent from the result — callers
	// detect "doesn't exist" by checking for the key, which is exactly the
	// information CreateLearningPath's 400 response needs to name the
	// missing content_node_id.
	GetByIDs(ctx context.Context, ids []string) (map[string]domain.ContentNode, error)

	// List returns one page of the content nodes matching filter, in
	// filter.Sort's order, with the count of all matches across pages.
	List(ctx context.Context, filter domain.ContentNodeFilter, page domain.PageRequest) (domain.Page[domain.ContentNode], error)

	// ListCreatorCounts returns each distinct creator of a content node
	// with how many nodes they created, in no particular order.
	ListCreatorCounts(ctx context.Context) ([]domain.CreatorCount, error)

	// Update persists every field of node, including UpdatedAt and its
	// publishing state. Returns domain.ErrNotFound if no node exists with
	// the given id.
	Update(ctx context.Context, node domain.ContentNode) error

	// MarkPublished records that version is now the node's latest published
	// version and that the node matches it, leaving every other field as it
	// is. Returns domain.ErrNotFound if no node exists with the given id.
	MarkPublished(ctx context.Context, id string, version int) error
	// LinkedExerciseInstrumentSets returns the instruments of every exercise
	// linked to the node, as a path exercise or through one of its
	// challenges — one set per link, empty meaning every instrument.
	LinkedExerciseInstrumentSets(ctx context.Context, id string) ([][]string, error)
}
