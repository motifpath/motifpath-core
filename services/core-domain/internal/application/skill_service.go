package application

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// SkillService manages Skill — a self-referential tree node ContentNode and
// Exercise classify against. There is deliberately no update/delete method,
// matching the OpenAPI surface (no PUT/DELETE /skills endpoint yet).
type SkillService struct {
	skills ports.SkillRepository
	newID  func() string
}

func NewSkillService(skills ports.SkillRepository, newID func() string) *SkillService {
	return &SkillService{skills: skills, newID: newID}
}

// CreateSkill creates a new skill, either as a root (parentID nil) or as a
// child of an existing skill. Only teachers and admins may create a skill —
// the tree is an authoring surface.
func (s *SkillService) CreateSkill(ctx context.Context, caller domain.User, name string, parentID *string) (domain.Skill, error) {
	if !canManageContent(caller.Role) {
		return domain.Skill{}, domain.ErrForbidden
	}

	skill, err := domain.NewSkill(s.newID(), name, parentID)
	if err != nil {
		return domain.Skill{}, err
	}

	if parentID != nil && *parentID != "" {
		if _, err := s.skills.GetByID(ctx, *parentID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return domain.Skill{}, domain.NewValidationError("parent_id", "does not reference an existing skill")
			}
			return domain.Skill{}, err
		}
	}

	exists, err := s.skills.ExistsSibling(ctx, parentID, name)
	if err != nil {
		return domain.Skill{}, err
	}
	if exists {
		return domain.Skill{}, domain.NewValidationError("name", "already exists among the siblings under this parent")
	}

	if err := s.skills.Create(ctx, skill); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

// ListSkills returns every known skill. Any authenticated user may list
// skills.
func (s *SkillService) ListSkills(ctx context.Context) ([]domain.Skill, error) {
	return s.skills.List(ctx)
}
