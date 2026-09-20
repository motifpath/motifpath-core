package application

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ConceptService manages Concept — see SkillService's doc comment for the
// shared shape and rationale.
type ConceptService struct {
	concepts ports.ConceptRepository
	newID    func() string
}

func NewConceptService(concepts ports.ConceptRepository, newID func() string) *ConceptService {
	return &ConceptService{concepts: concepts, newID: newID}
}

// CreateConcept creates a new concept, either as a root (parentID nil) or as
// a child of an existing concept. Only teachers and admins may create a
// concept — the tree is an authoring surface.
func (s *ConceptService) CreateConcept(ctx context.Context, caller domain.User, name string, parentID *string) (domain.Concept, error) {
	if !canManageContent(caller.Role) {
		return domain.Concept{}, domain.ErrForbidden
	}

	concept, err := domain.NewConcept(s.newID(), name, parentID)
	if err != nil {
		return domain.Concept{}, err
	}

	if parentID != nil && *parentID != "" {
		if _, err := s.concepts.GetByID(ctx, *parentID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return domain.Concept{}, domain.NewValidationError("parent_id", "does not reference an existing concept")
			}
			return domain.Concept{}, err
		}
	}

	exists, err := s.concepts.ExistsSibling(ctx, parentID, name)
	if err != nil {
		return domain.Concept{}, err
	}
	if exists {
		return domain.Concept{}, domain.NewValidationError("name", "already exists among the siblings under this parent")
	}

	if err := s.concepts.Create(ctx, concept); err != nil {
		return domain.Concept{}, err
	}
	return concept, nil
}

// ListConcepts returns every known concept. Any authenticated user may list
// concepts.
func (s *ConceptService) ListConcepts(ctx context.Context) ([]domain.Concept, error) {
	return s.concepts.List(ctx)
}
