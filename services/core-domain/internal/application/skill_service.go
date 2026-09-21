package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// SkillService manages Skill — a self-referential tree node ContentNode and
// Exercise classify against. It's a thin, name-preserving wrapper over the
// generic TaxonomyService, which holds the actual create/list logic shared
// with ConceptService.
type SkillService struct {
	*TaxonomyService[domain.Skill]
}

func NewSkillService(skills ports.SkillRepository, newID func() string) *SkillService {
	return &SkillService{TaxonomyService: NewTaxonomyService(skills, newID, domain.NewSkill, "skill")}
}

// CreateSkill creates a new skill, either as a root (parentID nil) or as a
// child of an existing skill. Only teachers and admins may create a skill —
// the tree is an authoring surface.
func (s *SkillService) CreateSkill(ctx context.Context, caller domain.User, name string, parentID *string) (domain.Skill, error) {
	return s.Create(ctx, caller, name, parentID)
}

// ListSkills returns every known skill. Any authenticated user may list
// skills.
func (s *SkillService) ListSkills(ctx context.Context) ([]domain.Skill, error) {
	return s.List(ctx)
}
