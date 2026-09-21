package application

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// checkSkillsAndConceptsExist reports a domain.ValidationError under
// "skill_ids"/"concept_ids" if any id in skillIDs/conceptIDs does not
// reference an existing Skill/Concept. The two checks hit independent
// tables, so they run concurrently rather than as two sequential round
// trips. A non-ValidationError (e.g. a DB failure) from either check is
// returned as-is; if both checks report a ValidationError, skill_ids takes
// precedence — a fixed order, rather than whichever goroutine happens to
// finish first, so the same request always reports the same field.
func checkSkillsAndConceptsExist(ctx context.Context, skills ports.SkillRepository, concepts ports.ConceptRepository, skillIDs, conceptIDs []string) error {
	g, gCtx := errgroup.WithContext(ctx)
	var skillErr, conceptErr error
	g.Go(func() error {
		err := checkSkillIDsExist(gCtx, skills, skillIDs)
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			skillErr = err
			return nil
		}
		return err
	})
	g.Go(func() error {
		err := checkConceptIDsExist(gCtx, concepts, conceptIDs)
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			conceptErr = err
			return nil
		}
		return err
	})
	if err := g.Wait(); err != nil {
		return err
	}
	if skillErr != nil {
		return skillErr
	}
	return conceptErr
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
