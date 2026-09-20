package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// checkSkillsAndConceptsExist reports a domain.ValidationError under
// "skill_ids"/"concept_ids" if any id in skillIDs/conceptIDs does not
// reference an existing Skill/Concept. Looks up every id in a single
// batched GetByIDs call per dimension rather than one GetByID per id.
func checkSkillsAndConceptsExist(ctx context.Context, skills ports.SkillRepository, concepts ports.ConceptRepository, skillIDs, conceptIDs []string) error {
	if len(skillIDs) > 0 {
		found, err := skills.GetByIDs(ctx, skillIDs)
		if err != nil {
			return err
		}
		for _, id := range skillIDs {
			if _, ok := found[id]; !ok {
				return domain.NewValidationError("skill_ids", "references a skill that does not exist: "+id)
			}
		}
	}
	if len(conceptIDs) > 0 {
		found, err := concepts.GetByIDs(ctx, conceptIDs)
		if err != nil {
			return err
		}
		for _, id := range conceptIDs {
			if _, ok := found[id]; !ok {
				return domain.NewValidationError("concept_ids", "references a concept that does not exist: "+id)
			}
		}
	}
	return nil
}
