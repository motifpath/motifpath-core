//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/domain"
)

// fakeFeltRatings is an in-memory ports.FeltRatingReader holding the
// felt-rated sessions of each drill template, across all students.
type fakeFeltRatings struct {
	mu       sync.Mutex
	sessions map[string]int
}

func newFakeFeltRatings() *fakeFeltRatings {
	return &fakeFeltRatings{sessions: map[string]int{}}
}

func (f *fakeFeltRatings) FeltRatedSessions(_ context.Context, templateKeys []string) (map[string]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := map[string]int{}
	for _, key := range templateKeys {
		if n, ok := f.sessions[key]; ok {
			counts[key] = n
		}
	}
	return counts, nil
}

func registerFeltQuestionSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)"'s next session will practise "fretboard_cell:name_the_note", "fretboard_cell:find_the_note" and "exercise:text_response"$`, w.nextSessionPractisesNameFindAndText)
	sc.Step(`^"([^"]+)" and "([^"]+)" have the fewest felt-rated sessions$`, w.haveFewestFeltRatedSessions)
	sc.Step(`^"([^"]+)"'s next session will practise only play-alongs$`, w.nextSessionPractisesOnlyPlayAlongs)

	sc.Step(`^the plan asks how "([^"]+)" and "([^"]+)" felt$`, w.planAsksHowFelt)
	sc.Step(`^the plan doesn't ask about "([^"]+)"$`, w.planDoesNotAskAbout)
	sc.Step(`^the plan asks no felt questions$`, w.planAsksNoFeltQuestions)
}

// nextSessionPractisesNameFindAndText adds, on top of the Background's new
// guitar cells (asked by name), a due guitar cell named right more often than
// found, so it is asked by finding, and a due text exercise.
func (w *world) nextSessionPractisesNameFindAndText(name string) error {
	w.cellDue(name, "guitar", 6, 5, 4, map[string]int{
		string(domain.FretboardDrillNameTheNote): 3, string(domain.FretboardDrillFindTheNote): 1,
	})
	w.dueItems(name, w.putPracticeExercise("felt-text", practiceSkill))
	return nil
}

// haveFewestFeltRatedSessions gives the two templates fewer felt-rated
// sessions than naming a note, fewest first.
func (w *world) haveFewestFeltRatedSessions(first, second string) error {
	w.feltRatings.mu.Lock()
	defer w.feltRatings.mu.Unlock()
	w.feltRatings.sessions[first] = 2
	w.feltRatings.sessions[second] = 5
	w.feltRatings.sessions["fretboard_cell:name_the_note"] = 30
	return nil
}

// nextSessionPractisesOnlyPlayAlongs drops the Background's fretboard cells,
// leaving its play-alongs.
func (w *world) nextSessionPractisesOnlyPlayAlongs(string) error {
	clear(w.practiceItems.cells)
	return nil
}

// planTemplates lists the timed drill templates the composed plan practises.
func (w *world) planTemplates() ([]string, error) {
	plan, err := w.composedPlan()
	if err != nil {
		return nil, err
	}
	var templates []string
	for _, item := range plan.Items {
		switch {
		case item.FretboardCell != nil:
			templates = append(templates, "fretboard_cell:"+string(item.FretboardCell.Drill))
		case item.Exercise != nil:
			templates = append(templates, "exercise:"+string(item.Exercise.ExerciseType))
		}
	}
	return templates, nil
}

func (w *world) planAsksHowFelt(first, second string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if want := []string{first, second}; !slices.Equal(plan.FeltQuestions, want) {
		return fmt.Errorf("expected felt questions %v, got %v", want, plan.FeltQuestions)
	}
	return nil
}

// planDoesNotAskAbout also checks the plan practises template, so leaving
// it out was a choice.
func (w *world) planDoesNotAskAbout(template string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	templates, err := w.planTemplates()
	if err != nil {
		return err
	}
	if !slices.Contains(templates, template) {
		return fmt.Errorf("expected the plan to practise %s, got %v", template, templates)
	}
	if slices.Contains(plan.FeltQuestions, template) {
		return fmt.Errorf("expected no felt question about %s, got %v", template, plan.FeltQuestions)
	}
	return nil
}

func (w *world) planAsksNoFeltQuestions() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if templates, err := w.planTemplates(); err != nil || len(templates) > 0 {
		return fmt.Errorf("expected a plan of play-alongs only, got timed drills %v (%v)", templates, err)
	}
	if plan.FeltQuestions == nil || len(plan.FeltQuestions) > 0 {
		return fmt.Errorf("expected an empty felt questions list, got %v", plan.FeltQuestions)
	}
	return nil
}
