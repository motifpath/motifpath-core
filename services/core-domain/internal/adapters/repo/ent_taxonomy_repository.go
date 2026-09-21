package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/domain"
)

// entTaxonomyRow is the raw (id, name, parent_id) shape common to ent's
// generated Skill and Concept rows. entTaxonomyRepository moves data across
// its closure boundary in this shape rather than as T directly, because Go
// generics can't read T's struct fields without a method-set constraint,
// and adding one to domain.Skill/Concept just to satisfy this repository
// would leak an adapter-layer concern into the domain layer.
type entTaxonomyRow struct {
	ID       uuid.UUID
	Name     string
	ParentID *uuid.UUID
}

// entTaxonomyRepository implements the Create/GetByID/GetByIDs/List/
// ExistsSibling shape ports.TaxonomyRepository[T] needs, via a small set of
// closures wrapping the entity-specific ent client calls — ent generates a
// distinct query-builder package per entity (skill vs concept), so there is
// no single type this shared body can call through directly.
// EntSkillRepository and EntConceptRepository are both thin instantiations
// — see either constructor for the entity-specific wiring. See
// ports.TaxonomyRepository's doc comment for why T's `any` constraint here
// doesn't conflict with this repo's ban on interface{}/any.
type entTaxonomyRepository[T any] struct {
	create        func(ctx context.Context, row entTaxonomyRow) error
	getByID       func(ctx context.Context, id uuid.UUID) (entTaxonomyRow, error) // domain.ErrNotFound on miss
	getByIDs      func(ctx context.Context, ids []uuid.UUID) ([]entTaxonomyRow, error)
	list          func(ctx context.Context) ([]entTaxonomyRow, error)
	existsSibling func(ctx context.Context, parentID *uuid.UUID, name string) (bool, error)
	wrap          func(row entTaxonomyRow) T
	unwrap        func(node T) (entTaxonomyRow, error)
}

func (r *entTaxonomyRepository[T]) Create(ctx context.Context, node T) error {
	row, err := r.unwrap(node)
	if err != nil {
		return err
	}
	return r.create(ctx, row)
}

func (r *entTaxonomyRepository[T]) GetByID(ctx context.Context, id string) (T, error) {
	var zero T
	parsed, err := uuid.Parse(id)
	if err != nil {
		return zero, domain.ErrNotFound
	}
	row, err := r.getByID(ctx, parsed)
	if err != nil {
		return zero, err
	}
	return r.wrap(row), nil
}

func (r *entTaxonomyRepository[T]) GetByIDs(ctx context.Context, ids []string) (map[string]T, error) {
	result := map[string]T{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.getByIDs(ctx, parseUUIDsSkippingInvalid(ids))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID.String()] = r.wrap(row)
	}
	return result, nil
}

func (r *entTaxonomyRepository[T]) List(ctx context.Context) ([]T, error) {
	rows, err := r.list(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]T, len(rows))
	for i, row := range rows {
		result[i] = r.wrap(row)
	}
	return result, nil
}

func (r *entTaxonomyRepository[T]) ExistsSibling(ctx context.Context, parentID *string, name string) (bool, error) {
	parsed, err := parseOptionalUUID(parentID)
	if err != nil {
		return false, err
	}
	return r.existsSibling(ctx, parsed, name)
}
