//go:build integration

package bdd

import (
	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// registerExerciseInstrumentSteps covers an exercise's instruments: its
// skills and concepts must suit them, the library filters by them, and a
// content node takes only exercises that suit its own instruments.
func registerExerciseInstrumentSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a root concept "([^"]+)" for instrument "([^"]+)" exists in the system$`, func(name, instrument string) error {
		w.putConcept(name, nil, instrumentID(instrument).String())
		return nil
	})
	sc.Step(`^"([^"]+)" creates an exercise titled "([^"]+)" for instruments ((?:"[^"]+"(?:, )?)+) classified under (skill|concept) "([^"]+)"$`, func(_, title, instruments, kind, name string) error {
		ids := generated.InstrumentIds(instrumentIDsNamed(quotedValues(instruments)))
		return w.createExerciseClassified(title, &ids, kind, name)
	})
	sc.Step(`^"([^"]+)" creates an exercise titled "([^"]+)" for every instrument classified under (skill|concept) "([^"]+)"$`, func(_, title, kind, name string) error {
		return w.createExerciseClassified(title, nil, kind, name)
	})
	sc.Step(`^an exercise "([^"]+)" for instruments ((?:"[^"]+"(?:, )?)+) classified under skill "([^"]+)" exists in the system$`, func(slug, instruments, skill string) error {
		if err := w.putExerciseFor(slug, instruments); err != nil {
			return err
		}
		exercise, err := w.exercises.GetByID(w.ctx(), exerciseID(slug).String())
		if err != nil {
			return err
		}
		exercise.Skills = []domain.KnowledgeNode{{ID: w.skillIDFor(skill).String()}}
		w.exercises.put(exercise)
		return nil
	})
	sc.Step(`^an exercise "([^"]+)" exists for every instrument$`, func(slug string) error {
		return w.putExerciseFor(slug, "")
	})
	sc.Step(`^"([^"]+)" updates exercise "([^"]+)" to be for instruments ((?:"[^"]+"(?:, )?)+)$`, w.updatesExerciseInstruments)
	sc.Step(`^"([^"]+)" lists exercises filtered by instruments ((?:"[^"]+"(?:, | and )?)+)$`, func(_, instruments string) error {
		ids := instrumentIDsNamed(quotedValues(instruments))
		return w.listExercises(generated.ListExercisesParams{InstrumentId: &ids})
	})
}

// createExerciseClassified creates a valid exercise for instruments (nil
// for every instrument) whose skill or concept, by kind, is name.
func (w *world) createExerciseClassified(title string, instruments *generated.InstrumentIds, kind, name string) error {
	body := w.exerciseBody(title)
	body.InstrumentIds = instruments
	if kind == string(domain.KnowledgeNodeKindSkill) {
		body.SkillIds = []uuid.UUID{w.skillIDFor(name)}
	} else {
		body.ConceptIds = []uuid.UUID{w.conceptIDFor(name)}
	}
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

// updatesExerciseInstruments replaces only the instruments of the exercise
// slug names, resending its current skills, concepts and content.
func (w *world) updatesExerciseInstruments(_, slug, instruments string) error {
	existing, err := w.exercises.GetByID(w.ctx(), exerciseID(slug).String())
	if err != nil {
		return err
	}
	label := "correct option"
	ids := generated.InstrumentIds(instrumentIDsNamed(quotedValues(instruments)))
	resp, err := w.handler.UpdateExercise(w.ctx(), generated.UpdateExerciseRequestObject{
		ExerciseId: exerciseID(slug),
		Body: &generated.UpdateExerciseRequest{
			SkillIds: idsOfSkills(existing.Skills), ConceptIds: idsOfConcepts(existing.Concepts),
			Title: existing.Title, Prompt: promptDocFor("prompt"),
			Options:       opts([]generated.Option{{OptionId: uuid.New(), IsCorrect: true, Label: &label}}),
			LanguageCodes: []string{"en"},
			InstrumentIds: &ids,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}
