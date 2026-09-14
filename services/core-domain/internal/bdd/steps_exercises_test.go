//go:build integration

package bdd

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerExerciseSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^an exercise "([^"]+)" exists$`, w.putExercise)

	sc.Step(`^"([^"]+)" creates a(?:n)? (\S+) exercise titled "([^"]+)" with prompt "([^"]+)" and one correct option$`, w.createsExercise)
	sc.Step(`^"([^"]+)" creates a(?:n)? (\S+) exercise titled "([^"]+)" with prompt "([^"]+)" and one correct option and skill tags "([^"]+)"$`, w.createsExerciseWithSkillTags)
	sc.Step(`^"([^"]+)" retrieves the exercise "([^"]+)"$`, w.retrievesExercise)
	sc.Step(`^"([^"]+)" submits a create exercise request with the title field omitted$`, w.submitsExerciseMissingTitle)
	sc.Step(`^"([^"]+)" submits a create exercise request with the prompt field omitted$`, w.submitsExerciseMissingPrompt)
	sc.Step(`^"([^"]+)" submits a create exercise request with the exercise_type field omitted$`, w.submitsExerciseMissingType)
	sc.Step(`^"([^"]+)" submits a create exercise request with exercise_type "([^"]+)"$`, w.submitsExerciseWithType)
	sc.Step(`^"([^"]+)" submits a create exercise request whose options have no option marked correct$`, w.submitsExerciseNoCorrectOption)
	sc.Step(`^"([^"]+)" submits a create exercise request with an empty-string skill tag$`, w.submitsExerciseEmptySkillTag)
	sc.Step(`^"([^"]+)" retrieves an exercise with an ID that does not exist$`, w.retrievesMissingExercise)
	sc.Step(`^"([^"]+)" attempts to create an exercise$`, w.attemptsCreateExercise)
	sc.Step(`^an unauthenticated request attempts to create an exercise$`, w.unauthCreatesExercise)

	sc.Step(`^"([^"]+)" links exercise "([^"]+)" to "([^"]+)"$`, w.linksExerciseToChallenge)
	sc.Step(`^"([^"]+)" has linked exercise "([^"]+)" to "([^"]+)"$`, w.hasLinkedExerciseToChallenge)
	sc.Step(`^"([^"]+)" unlinks exercise "([^"]+)" from "([^"]+)"$`, w.unlinksExerciseFromChallenge)
	sc.Step(`^"([^"]+)" links an exercise ID that does not exist to "([^"]+)"$`, w.linksMissingExerciseToChallenge)
	sc.Step(`^"([^"]+)" links exercise "([^"]+)" to a challenge ID that does not exist$`, w.linksExerciseToMissingChallenge)
	sc.Step(`^"([^"]+)" attempts to link exercise "([^"]+)" to "([^"]+)"$`, w.linksExerciseToChallenge)
	sc.Step(`^"([^"]+)" attempts to unlink exercise "([^"]+)" from "([^"]+)"$`, w.unlinksExerciseFromChallenge)

	sc.Step(`^the exercise is created and assigned a stable identifier$`, w.exerciseCreated)
	sc.Step(`^the exercise's type is recorded as (\S+)$`, w.exerciseTypeRecorded)
	sc.Step(`^the exercise carries skill tags "([^"]+)"$`, w.exerciseCarriesSkillTags)
	sc.Step(`^the exercise is not linked to any challenge$`, w.exerciseNotLinked)
	sc.Step(`^the response returns the exercise's title, prompt, type, options, and linked challenges$`, w.exerciseResponseComplete)
	sc.Step(`^the exercise records "([^"]+)" among its linked challenges$`, w.exerciseRecordsLinkedChallenge)
	sc.Step(`^the exercise records both "([^"]+)" and "([^"]+)" among its linked challenges$`, w.exerciseRecordsBothLinkedChallenges)
	sc.Step(`^exactly one exercise "([^"]+)" exists in the system$`, w.exactlyOneExerciseExists)
	sc.Step(`^the exercise no longer records "([^"]+)" among its linked challenges$`, w.exerciseNoLongerRecordsLinkedChallenge)
}

func (w *world) putExercise(slug string) error {
	label := "option-" + slug
	w.exercises.put(domain.Exercise{
		ID:           exerciseID(slug).String(),
		Title:        "title-" + slug,
		Prompt:       "prompt-" + slug,
		ExerciseType: domain.ExerciseTypeTextResponse,
		Options:      []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}},
		ChallengeIDs: []string{},
		CreatedAt:    fixedNow,
	})
	return nil
}

func optionsFor(exerciseType generated.CreateExerciseRequestExerciseType) []generated.Option {
	label := "correct option"
	switch exerciseType {
	case generated.CreateExerciseRequestExerciseTypeImageRecognition:
		return []generated.Option{{OptionId: uuid.New(), IsCorrect: true, Region: &generated.OptionRegion{X: 0.2, Y: 0.3, Width: 0.1, Height: 0.1, Shape: generated.Rectangle}}}
	case generated.CreateExerciseRequestExerciseTypeImageChoice:
		imageURL := "https://cdn.example.com/library/option.png"
		return []generated.Option{{OptionId: uuid.New(), IsCorrect: true, ImageUrl: &imageURL}}
	default:
		return []generated.Option{{OptionId: uuid.New(), IsCorrect: true, Label: &label}}
	}
}

func mediaFieldsFor(exerciseType generated.CreateExerciseRequestExerciseType) (imageURL, audioURL *string) {
	switch exerciseType {
	case generated.CreateExerciseRequestExerciseTypeImageRecognition:
		url := "https://cdn.example.com/fretboard/prompt.png"
		return &url, nil
	case generated.CreateExerciseRequestExerciseTypeAudioRecognition:
		url := "https://cdn.example.com/audio/prompt.mp3"
		return nil, &url
	default:
		return nil, nil
	}
}

func (w *world) createsExercise(name, exerciseType, title, prompt string) error {
	et := generated.CreateExerciseRequestExerciseType(exerciseType)
	imageURL, audioURL := mediaFieldsFor(et)
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			Title: title, Prompt: prompt, ExerciseType: et,
			ImageUrl: imageURL, AudioUrl: audioURL,
			Options: optionsFor(et),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsExerciseWithSkillTags(name, exerciseType, title, prompt, tagsCSV string) error {
	et := generated.CreateExerciseRequestExerciseType(exerciseType)
	imageURL, audioURL := mediaFieldsFor(et)
	tags := splitCSV(tagsCSV)
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			Title: title, Prompt: prompt, ExerciseType: et,
			ImageUrl: imageURL, AudioUrl: audioURL,
			SkillTags: &tags,
			Options:   optionsFor(et),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func splitCSV(csv string) []string {
	parts := strings.Split(csv, ",")
	result := make([]string, len(parts))
	for i, p := range parts {
		result[i] = strings.TrimSpace(p)
	}
	return result
}

func (w *world) retrievesExercise(name, slug string) error {
	resp, err := w.handler.GetExercise(w.ctx(), generated.GetExerciseRequestObject{ExerciseId: exerciseID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseMissingTitle(string) error {
	label := "correct option"
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			Prompt: "prompt", ExerciseType: generated.CreateExerciseRequestExerciseTypeTextResponse,
			Options: []generated.Option{{OptionId: uuid.New(), IsCorrect: true, Label: &label}},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseMissingPrompt(string) error {
	label := "correct option"
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			Title: "title", ExerciseType: generated.CreateExerciseRequestExerciseTypeTextResponse,
			Options: []generated.Option{{OptionId: uuid.New(), IsCorrect: true, Label: &label}},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseMissingType(string) error {
	label := "correct option"
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			Title: "title", Prompt: "prompt",
			Options: []generated.Option{{OptionId: uuid.New(), IsCorrect: true, Label: &label}},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseWithType(name, exerciseType string) error {
	label := "correct option"
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			Title: "title", Prompt: "prompt", ExerciseType: generated.CreateExerciseRequestExerciseType(exerciseType),
			Options: []generated.Option{{OptionId: uuid.New(), IsCorrect: true, Label: &label}},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseNoCorrectOption(string) error {
	label := "an option"
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			Title: "title", Prompt: "prompt", ExerciseType: generated.CreateExerciseRequestExerciseTypeTextResponse,
			Options: []generated.Option{{OptionId: uuid.New(), IsCorrect: false, Label: &label}},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExerciseEmptySkillTag(string) error {
	label := "correct option"
	tags := []string{"technique", ""}
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			Title: "title", Prompt: "prompt", ExerciseType: generated.CreateExerciseRequestExerciseTypeTextResponse,
			SkillTags: &tags,
			Options:   []generated.Option{{OptionId: uuid.New(), IsCorrect: true, Label: &label}},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesMissingExercise(string) error {
	resp, err := w.handler.GetExercise(w.ctx(), generated.GetExerciseRequestObject{ExerciseId: deterministicUUID("exercise", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsCreateExercise(string) error {
	return w.createsExercise("", "text_response", "title", "prompt")
}

func (w *world) unauthCreatesExercise() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateExercise("")
}

func (w *world) linksExerciseToChallenge(name, exerciseSlug, challengeSlug string) error {
	resp, err := w.handler.LinkExerciseToChallenge(w.ctx(), generated.LinkExerciseToChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		ExerciseId:  exerciseID(exerciseSlug),
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) hasLinkedExerciseToChallenge(name, exerciseSlug, challengeSlug string) error {
	return w.linksExerciseToChallenge(name, exerciseSlug, challengeSlug)
}

func (w *world) unlinksExerciseFromChallenge(name, exerciseSlug, challengeSlug string) error {
	resp, err := w.handler.UnlinkExerciseFromChallenge(w.ctx(), generated.UnlinkExerciseFromChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		ExerciseId:  exerciseID(exerciseSlug),
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) linksMissingExerciseToChallenge(name, challengeSlug string) error {
	resp, err := w.handler.LinkExerciseToChallenge(w.ctx(), generated.LinkExerciseToChallengeRequestObject{
		ChallengeId: challengeID(challengeSlug),
		ExerciseId:  deterministicUUID("exercise", "does-not-exist"),
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) linksExerciseToMissingChallenge(name, exerciseSlug string) error {
	resp, err := w.handler.LinkExerciseToChallenge(w.ctx(), generated.LinkExerciseToChallengeRequestObject{
		ChallengeId: deterministicUUID("challenge", "does-not-exist"),
		ExerciseId:  exerciseID(exerciseSlug),
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) exerciseCreated() error {
	if _, ok := w.lastResp.(generated.CreateExercise201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) exerciseTypeRecorded(exerciseType string) error {
	resp, ok := w.lastResp.(generated.CreateExercise201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if string(resp.ExerciseType) != exerciseType {
		return fmt.Errorf("expected exercise_type %q, got %q", exerciseType, resp.ExerciseType)
	}
	return nil
}

func (w *world) exerciseCarriesSkillTags(tagsCSV string) error {
	resp, ok := w.lastResp.(generated.CreateExercise201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.SkillTags == nil {
		return fmt.Errorf("expected skill_tags to be set")
	}
	want := splitCSV(tagsCSV)
	got := *resp.SkillTags
	if len(want) != len(got) {
		return fmt.Errorf("expected skill_tags %v, got %v", want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			return fmt.Errorf("expected skill_tags %v, got %v", want, got)
		}
	}
	return nil
}

func (w *world) exerciseNotLinked() error {
	resp, ok := w.lastResp.(generated.CreateExercise201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if len(resp.ChallengeIds) != 0 {
		return fmt.Errorf("expected no linked challenges, got %v", resp.ChallengeIds)
	}
	return nil
}

func (w *world) exerciseResponseComplete() error {
	resp, ok := w.lastResp.(generated.GetExercise200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.Title == "" || resp.Prompt == "" || resp.ExerciseType == "" || resp.Options == nil || resp.ChallengeIds == nil {
		return fmt.Errorf("expected a fully populated exercise, got %+v", resp)
	}
	return nil
}

func (w *world) exerciseRecordsLinkedChallenge(challengeSlug string) error {
	resp, ok := w.lastResp.(generated.LinkExerciseToChallenge201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	for _, id := range resp.ChallengeIds {
		if id == challengeID(challengeSlug) {
			return nil
		}
	}
	return fmt.Errorf("expected challenge_ids to contain %s, got %v", challengeID(challengeSlug), resp.ChallengeIds)
}

func (w *world) exerciseRecordsBothLinkedChallenges(slugA, slugB string) error {
	if err := w.exerciseRecordsLinkedChallenge(slugA); err != nil {
		return err
	}
	return w.exerciseRecordsLinkedChallenge(slugB)
}

func (w *world) exactlyOneExerciseExists(slug string) error {
	if w.exercises.count() != 1 {
		return fmt.Errorf("expected exactly 1 exercise to exist, got %d", w.exercises.count())
	}
	return nil
}

func (w *world) exerciseNoLongerRecordsLinkedChallenge(challengeSlug string) error {
	if _, ok := w.lastResp.(generated.UnlinkExerciseFromChallenge204Response); !ok {
		return fmt.Errorf("expected a 204 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}
