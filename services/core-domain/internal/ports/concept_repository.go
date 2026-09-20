package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ConceptRepository persists Concept records. See SkillRepository's doc
// comment for the shared shape and rationale — Concept and Skill are
// otherwise independent trees.
type ConceptRepository interface {
	Create(ctx context.Context, concept domain.Concept) error

	// GetByID returns domain.ErrNotFound if no concept exists with the given id.
	GetByID(ctx context.Context, id string) (domain.Concept, error)

	// GetByIDs returns the concepts matching ids, keyed by ID. An id with no
	// matching concept is simply absent from the result.
	GetByIDs(ctx context.Context, ids []string) (map[string]domain.Concept, error)

	// List returns every known concept, sorted by name within each level.
	List(ctx context.Context) ([]domain.Concept, error)

	// ExistsSibling reports whether a concept named name already exists
	// under parentID (nil meaning a root concept).
	ExistsSibling(ctx context.Context, parentID *string, name string) (bool, error)
}
