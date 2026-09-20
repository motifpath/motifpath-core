package application

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// checkSkillsAndConceptsExist reports a domain.ValidationError under
// "skill_ids"/"concept_ids" if any id in skillIDs/conceptIDs does not
// reference an existing Skill/Concept.
func checkSkillsAndConceptsExist(ctx context.Context, skills ports.SkillRepository, concepts ports.ConceptRepository, skillIDs, conceptIDs []string) error {
	if err := checkSkillIDsExist(ctx, skills, skillIDs); err != nil {
		return err
	}
	return checkConceptIDsExist(ctx, concepts, conceptIDs)
}

// checkSkillIDsExist reports a domain.ValidationError under "skill_ids" if
// any id in skillIDs does not reference an existing Skill. Looks up every
// id in a single batched GetByIDs call rather than one GetByID per id.
func checkSkillIDsExist(ctx context.Context, skills ports.SkillRepository, skillIDs []string) error {
	if len(skillIDs) == 0 {
		return nil
	}
	found, err := skills.GetByIDs(ctx, skillIDs)
	if err != nil {
		return err
	}
	for _, id := range skillIDs {
		if _, ok := found[id]; !ok {
			return domain.NewValidationError("skill_ids", "references a skill that does not exist: "+id)
		}
	}
	return nil
}

// checkConceptIDsExist is checkSkillIDsExist's counterpart for concept_ids.
func checkConceptIDsExist(ctx context.Context, concepts ports.ConceptRepository, conceptIDs []string) error {
	if len(conceptIDs) == 0 {
		return nil
	}
	found, err := concepts.GetByIDs(ctx, conceptIDs)
	if err != nil {
		return err
	}
	for _, id := range conceptIDs {
		if _, ok := found[id]; !ok {
			return domain.NewValidationError("concept_ids", "references a concept that does not exist: "+id)
		}
	}
	return nil
}
