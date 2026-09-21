package application

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// TaxonomyService manages a Skill or Concept tree — see
// domain.TaxonomyNode's doc comment for the shared shape. There is
// deliberately no update/delete method, matching the OpenAPI surface (no
// PUT/DELETE /skills or /concepts endpoint yet).
type TaxonomyService[T any] struct {
	repo      ports.TaxonomyRepository[T]
	newID     func() string
	construct func(id, name string, parentID *string) (T, error)
	// noun names the resource in validation/forbidden messages ("skill" or
	// "concept") so both instantiations read the same as their pre-generic,
	// hand-written services did.
	noun string
}

func NewTaxonomyService[T any](
	repo ports.TaxonomyRepository[T],
	newID func() string,
	construct func(id, name string, parentID *string) (T, error),
	noun string,
) *TaxonomyService[T] {
	return &TaxonomyService[T]{repo: repo, newID: newID, construct: construct, noun: noun}
}

// Create creates a new node, either as a root (parentID nil) or as a child
// of an existing node. Only teachers and admins may create one — the tree
// is an authoring surface.
func (s *TaxonomyService[T]) Create(ctx context.Context, caller domain.User, name string, parentID *string) (T, error) {
	var zero T
	if !canManageContent(caller.Role) {
		return zero, domain.ErrForbidden
	}

	node, err := s.construct(s.newID(), name, parentID)
	if err != nil {
		return zero, err
	}

	if err := s.checkParentAndSibling(ctx, parentID, name); err != nil {
		return zero, err
	}

	if err := s.repo.Create(ctx, node); err != nil {
		return zero, err
	}
	return node, nil
}

// checkParentAndSibling reports a domain.ValidationError if parentID is set
// but doesn't reference an existing node, or if name already exists among
// the siblings under parentID. The two checks are independent lookups
// against the same table, run concurrently rather than as two sequential
// round trips. On a race between the two, parent-not-found takes precedence
// — the same order a sequential check would report first.
func (s *TaxonomyService[T]) checkParentAndSibling(ctx context.Context, parentID *string, name string) error {
	g, gCtx := errgroup.WithContext(ctx)
	var parentErr, siblingErr error
	g.Go(func() error {
		if parentID == nil || *parentID == "" {
			return nil
		}
		if _, err := s.repo.GetByID(gCtx, *parentID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				parentErr = domain.NewValidationError("parent_id", "does not reference an existing "+s.noun)
				return nil
			}
			return err
		}
		return nil
	})
	g.Go(func() error {
		exists, err := s.repo.ExistsSibling(gCtx, parentID, name)
		if err != nil {
			return err
		}
		if exists {
			siblingErr = domain.NewValidationError("name", "already exists among the siblings under this parent")
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return err
	}
	if parentErr != nil {
		return parentErr
	}
	return siblingErr
}

// List returns every known node. Any authenticated user may list them.
func (s *TaxonomyService[T]) List(ctx context.Context) ([]T, error) {
	return s.repo.List(ctx)
}
