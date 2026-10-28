//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/domain"
)

func registerPracticeLanguageSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)"'s path skill "([^"]+)" has the exercise "([^"]+)" (in English only|in Portuguese|for any language)$`, w.pathSkillHasExerciseIn)
	sc.Step(`^the review of "([^"]+)" is due for "([^"]+)"$`, w.reviewIsDueFor)
	sc.Step(`^the review of the English-only exercise "([^"]+)" of "([^"]+)" is due for "([^"]+)"$`, w.reviewOfEnglishExerciseIsDue)
	sc.Step(`^the skill "([^"]+)" has only English-only exercises on guitar$`, w.skillHasOnlyEnglishExercises)
	sc.Step(`^"([^"]+)" is not in the session$`, w.notInSession)
}

// exerciseLanguages maps how a scenario says an exercise's languages to
// their codes.
var exerciseLanguages = map[string][]string{
	"in English only":  {"en"},
	"in Portuguese":    {"pt_BR"},
	"for any language": {domain.LanguageCodeAny},
}

// putExerciseIn seeds a practice exercise for every instrument under skill,
// available in codes, and returns its item key.
func (w *world) putExerciseIn(slug, skill string, codes ...string) string {
	seconds := practiceExerciseSeconds
	languages := make([]domain.Language, len(codes))
	for i, code := range codes {
		languages[i] = domain.Language{Code: code}
	}
	id := exerciseID(slug).String()
	w.exercises.put(domain.Exercise{
		ID: id, Title: slug, ExerciseType: domain.ExerciseTypeTextResponse,
		Skills:                   []domain.KnowledgeNode{{ID: w.skillIDFor(skill).String()}},
		EstimatedDurationSeconds: &seconds,
		Languages:                languages,
		CreatedAt:                fixedNow,
	})
	return domain.ExerciseItemKey(id)
}

func (w *world) pathSkillHasExerciseIn(name, skill, slug, languages string) error {
	w.addPathSkill(name, skill)
	w.putExerciseIn(slug, skill, exerciseLanguages[languages]...)
	return nil
}

func (w *world) reviewIsDueFor(slug, name string) error {
	w.dueItems(name, w.itemKeyFor(slug))
	return nil
}

func (w *world) reviewOfEnglishExerciseIsDue(slug, skill, name string) error {
	w.dueItems(name, w.putExerciseIn(slug, skill, "en"))
	return nil
}

func (w *world) skillHasOnlyEnglishExercises(skill string) error {
	for i := range 3 {
		w.putExerciseIn(fmt.Sprintf("%s-en-%02d", skill, i), skill, "en")
	}
	return nil
}

func (w *world) notInSession(slug string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	key := w.itemKeyFor(slug)
	for _, item := range plan.Items {
		if item.ItemKey == key {
			return fmt.Errorf("expected %q not in the session, got it with the reason %s", slug, item.Reason)
		}
	}
	return nil
}
