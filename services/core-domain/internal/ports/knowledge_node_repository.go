package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// KnowledgeNodeFilter narrows KnowledgeNodeRepository.List. A nil Kind
// matches both kinds; an empty InstrumentIDs matches every node, otherwise a
// node matches when it is for every instrument or lists any of them.
type KnowledgeNodeFilter struct {
	Kind          *domain.KnowledgeNodeKind
	InstrumentIDs []string
}

// KnowledgeNodeUsage counts what still refers to a knowledge node — every
// reason it can't be deleted.
type KnowledgeNodeUsage struct {
	Children     int
	Edges        int
	ContentNodes int
	Exercises    int
	Diagrams     int
	Challenges   int
}

// KnowledgeNodeRepository persists the skills and concepts of the
// knowledge graph.
type KnowledgeNodeRepository interface {
	// Create returns domain.ErrAlreadyExists if another node has node.Key.
	Create(ctx context.Context, node domain.KnowledgeNode) error

	// GetByID returns domain.ErrNotFound if no node exists with id.
	GetByID(ctx context.Context, id string) (domain.KnowledgeNode, error)

	// GetByIDs returns the nodes matching ids, keyed by ID. An id with no
	// matching node is simply absent from the result.
	GetByIDs(ctx context.Context, ids []string) (map[string]domain.KnowledgeNode, error)

	// GetByKeys returns the nodes matching keys, keyed by Key. A key with no
	// matching node is simply absent from the result.
	GetByKeys(ctx context.Context, keys []string) (map[string]domain.KnowledgeNode, error)

	// List returns the nodes matching filter, sorted by key.
	List(ctx context.Context, filter KnowledgeNodeFilter) ([]domain.KnowledgeNode, error)

	// Update replaces the node's names, descriptions, parent and
	// instruments; kind and key never change. Returns domain.ErrNotFound
	// if no node exists with node.ID.
	Update(ctx context.Context, node domain.KnowledgeNode) error

	// Delete returns domain.ErrNotFound if no node exists with id.
	Delete(ctx context.Context, id string) error

	// Children returns the nodes whose parent is the node with id.
	Children(ctx context.Context, id string) ([]domain.KnowledgeNode, error)

	// InSubtree reports whether candidateID is rootID or one of its
	// descendants.
	InSubtree(ctx context.Context, rootID, candidateID string) (bool, error)

	// Usage counts what still refers to the node with id.
	Usage(ctx context.Context, id string) (KnowledgeNodeUsage, error)

	// ClassifiedInstrumentSets returns the instruments of every content node
	// and diagram classified under the node with id, one set per item; an
	// empty set is an item for every instrument.
	ClassifiedInstrumentSets(ctx context.Context, id string) ([][]string, error)
}
