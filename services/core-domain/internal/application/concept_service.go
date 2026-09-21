package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ConceptService manages Concept — see SkillService's doc comment for the
// shared shape and rationale.
type ConceptService struct {
	*TaxonomyService[domain.Concept]
}

func NewConceptService(concepts ports.ConceptRepository, newID func() string) *ConceptService {
	return &ConceptService{TaxonomyService: NewTaxonomyService(concepts, newID, domain.NewConcept, "concept")}
}

// CreateConcept creates a new concept, either as a root (parentID nil) or as
// a child of an existing concept. Only teachers and admins may create a
// concept — the tree is an authoring surface.
func (s *ConceptService) CreateConcept(ctx context.Context, caller domain.User, name string, parentID *string) (domain.Concept, error) {
	return s.Create(ctx, caller, name, parentID)
}

// ListConcepts returns every known concept. Any authenticated user may list
// concepts.
func (s *ConceptService) ListConcepts(ctx context.Context) ([]domain.Concept, error) {
	return s.List(ctx)
}
