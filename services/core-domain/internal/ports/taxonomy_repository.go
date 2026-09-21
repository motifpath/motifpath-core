package ports

import (
	"context"
)

// TaxonomyRepository persists a Skill or Concept tree — see
// domain.TaxonomyNode's doc comment for the shared shape. There is no
// update/delete method: neither resource has an update/delete endpoint yet.
//
// T's constraint is `any` because this interface only ever moves a T value
// through, never inspects or operates on it — a generic type parameter is a
// compile-time stand-in for a concrete, statically known type (here,
// domain.Skill or domain.Concept, fixed by TaxonomyRepository[T]'s two
// instantiations in skill_repository.go/concept_repository.go), not the
// untyped, runtime-checked "any value at all" this repo's `any`/
// interface{} ban targets.
type TaxonomyRepository[T any] interface {
	Create(ctx context.Context, node T) error

	// GetByID returns domain.ErrNotFound if no node exists with the given id.
	GetByID(ctx context.Context, id string) (T, error)

	// GetByIDs returns the nodes matching ids, keyed by ID. An id with no
	// matching node is simply absent from the result — the same
	// "not found, simply absent" convention ContentNodeRepository.GetByIDs
	// documents, letting a caller detect a bad id by checking for the key.
	GetByIDs(ctx context.Context, ids []string) (map[string]T, error)

	// List returns every known node, sorted by name within each level.
	List(ctx context.Context) ([]T, error)

	// ExistsSibling reports whether a node named name already exists under
	// parentID (nil meaning a root node) — the sibling-uniqueness rule a
	// plain unique index on a nullable parent_id column can't enforce
	// (Postgres treats NULL as distinct from NULL).
	ExistsSibling(ctx context.Context, parentID *string, name string) (bool, error)
}
