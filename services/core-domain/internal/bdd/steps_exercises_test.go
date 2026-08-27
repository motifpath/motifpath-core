//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerExerciseSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^an exercise "([^"]+)" exists within "([^"]+)"$`, w.putExercise)

	sc.Step(`^"([^"]+)" creates a fretboard_region exercise for "([^"]+)"\s+with prompt "([^"]+)"$`, w.createsExercise)
	sc.Step(`^"([^"]+)" creates three fretboard_region exercises for "([^"]+)"$`, w.createsThreeExercises)
	sc.Step(`^"([^"]+)" retrieves the exercise "([^"]+)"$`, w.retrievesExercise)
	sc.Step(`^"([^"]+)" submits a create exercise request with the exercise_type field omitted$`, w.submitsExerciseMissingType)
	sc.Step(`^"([^"]+)" submits a create exercise request with the prompt field omitted$`, w.submitsExerciseMissingPrompt)
	sc.Step(`^"([^"]+)" submits a create exercise request with exercise_type "([^"]+)"$`, w.submitsExerciseWithType)
	sc.Step(`^"([^"]+)" creates an exercise for a challenge ID that does not exist$`, w.createsExerciseForMissingChallenge)
	sc.Step(`^"([^"]+)" retrieves an exercise with an ID that does not exist$`, w.retrievesMissingExercise)
	sc.Step(`^"([^"]+)" attempts to create an exercise for "([^"]+)"$`, w.attemptsCreateExercise)
	sc.Step(`^an unauthenticated request attempts to create an exercise$`, w.unauthCreatesExercise)

	sc.Step(`^the exercise is created and assigned a stable identifier$`, w.exerciseCreated)
	sc.Step(`^the exercise records "([^"]+)" as its parent challenge$`, w.exerciseRecordsParent)
	sc.Step(`^the response returns the exercise's type, prompt, and parent challenge$`, w.exerciseResponseComplete)
}

func (w *world) putExercise(slug, challengeSlug string) error {
	w.exercises.put(domain.Exercise{
		ID:           exerciseID(slug).String(),
		ChallengeID:  challengeID(challengeSlug).String(),
		ExerciseType: domain.ExerciseTypeFretboardRegion,
		Prompt:       "prompt-" + slug,
		CreatedAt:    fixedNow,
	})
	return nil
}

func (w *world) createsExercise(name, challengeSlug, prompt string) error {
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body:        &generated.CreateExerciseRequest{ExerciseType: generated.CreateExerciseRequestExerciseTypeFretboardRegion, Prompt: prompt},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsThreeExercises(name, challengeSlug string) error {
	w.multiCreateIDs = nil
	for i := 0; i < 3; i++ {
		resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
			ChallengeId: challengeID(challengeSlug),
			Body: &generated.CreateExerciseRequest{
				ExerciseType: generated.CreateExerciseRequestExerciseTypeFretboardRegion,
				Prompt:       fmt.Sprintf("prompt-%d", i),
			},
		})
		w.lastResp, w.lastErr = resp, err
		if err != nil {
			return err
		}
		created, ok := resp.(generated.CreateExercise201JSONResponse)
		if !ok {
			return fmt.Errorf("expected a 201 response, got %#v", resp)
		}
		w.multiCreateIDs = append(w.multiCreateIDs, created.ExerciseId)
	}
	return nil
}

func (w *world) retrievesExercise(name, slug string) error {
	resp, err := w.handler.GetExercise(w.ctx(), generated.GetExerciseRequestObject{ExerciseId: exerciseID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseMissingType(string) error {
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		ChallengeId: challengeID("triad-challenge"),
		Body:        &generated.CreateExerciseRequest{Prompt: "prompt"},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseMissingPrompt(string) error {
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		ChallengeId: challengeID("triad-challenge"),
		Body:        &generated.CreateExerciseRequest{ExerciseType: generated.CreateExerciseRequestExerciseTypeFretboardRegion},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseWithType(name, exerciseType string) error {
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		ChallengeId: challengeID("triad-challenge"),
		Body:        &generated.CreateExerciseRequest{ExerciseType: generated.CreateExerciseRequestExerciseType(exerciseType), Prompt: "prompt"},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsExerciseForMissingChallenge(string) error {
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		ChallengeId: deterministicUUID("challenge", "does-not-exist"),
		Body:        &generated.CreateExerciseRequest{ExerciseType: generated.CreateExerciseRequestExerciseTypeFretboardRegion, Prompt: "prompt"},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesMissingExercise(string) error {
	resp, err := w.handler.GetExercise(w.ctx(), generated.GetExerciseRequestObject{ExerciseId: deterministicUUID("exercise", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsCreateExercise(name, challengeSlug string) error {
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		ChallengeId: challengeID(challengeSlug),
		Body:        &generated.CreateExerciseRequest{ExerciseType: generated.CreateExerciseRequestExerciseTypeFretboardRegion, Prompt: "prompt"},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthCreatesExercise() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateExercise("", "triad-challenge")
}

func (w *world) exerciseCreated() error {
	if _, ok := w.lastResp.(generated.CreateExercise201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) exerciseRecordsParent(challengeSlug string) error {
	resp, ok := w.lastResp.(generated.CreateExercise201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.ChallengeId != challengeID(challengeSlug) {
		return fmt.Errorf("expected challenge_id %s, got %s", challengeID(challengeSlug), resp.ChallengeId)
	}
	return nil
}

func (w *world) exerciseResponseComplete() error {
	resp, ok := w.lastResp.(generated.GetExercise200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.ExerciseType == "" || resp.Prompt == "" || resp.ChallengeId.String() == "" {
		return fmt.Errorf("expected a fully populated exercise, got %+v", resp)
	}
	return nil
}
