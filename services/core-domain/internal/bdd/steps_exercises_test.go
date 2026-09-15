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

	sc.Step(`^"([^"]+)" lists the exercises for challenge "([^"]+)"$`, w.listsChallengeExercises)
	sc.Step(`^"([^"]+)" lists the exercises for challenge "([^"]+)" twice$`, w.listsChallengeExercisesTwice)
	sc.Step(`^"([^"]+)" lists the exercises for a challenge ID that does not exist$`, w.listsChallengeExercisesMissing)
	sc.Step(`^an unauthenticated request attempts to list the exercises for challenge "([^"]+)"$`, w.unauthListsChallengeExercises)
	sc.Step(`^a challenge "([^"]+)" exists for content node "([^"]+)" with exercise shuffling disabled$`, w.putChallengeNoShuffle)
	sc.Step(`^a challenge "([^"]+)" exists for content node "([^"]+)" with exercise shuffling and option shuffling enabled$`, w.putChallengeShuffled)
	sc.Step(`^(\d+) exercises are linked to "([^"]+)" in a known order$`, w.linksNExercisesToChallenge)

	sc.Step(`^"([^"]+)" links exercise "([^"]+)" to content node "([^"]+)" as a path exercise$`, w.linksExerciseToContentNode)
	sc.Step(`^"([^"]+)" has linked exercise "([^"]+)" to content node "([^"]+)" as a path exercise$`, w.linksExerciseToContentNode)
	sc.Step(`^"([^"]+)" attempts to link exercise "([^"]+)" to content node "([^"]+)" as a path exercise$`, w.linksExerciseToContentNode)
	sc.Step(`^"([^"]+)" unlinks exercise "([^"]+)" from content node "([^"]+)"$`, w.unlinksExerciseFromContentNode)
	sc.Step(`^"([^"]+)" attempts to unlink exercise "([^"]+)" from content node "([^"]+)"$`, w.unlinksExerciseFromContentNode)
	sc.Step(`^"([^"]+)" links an exercise ID that does not exist to content node "([^"]+)" as a path exercise$`, w.linksMissingExerciseToContentNode)
	sc.Step(`^"([^"]+)" links exercise "([^"]+)" to a content node ID that does not exist as a path exercise$`, w.linksExerciseToMissingContentNode)
	sc.Step(`^"([^"]+)" lists the path exercises for content node "([^"]+)"$`, w.listsPathExercises)
	sc.Step(`^"([^"]+)" lists the path exercises for content node "([^"]+)" twice$`, w.listsPathExercisesTwice)
	sc.Step(`^"([^"]+)" lists the path exercises for a content node ID that does not exist$`, w.listsPathExercisesMissing)
	sc.Step(`^an unauthenticated request attempts to link an exercise to content node "([^"]+)"\s+as a path exercise$`, w.unauthLinksPathExercise)
	sc.Step(`^an unauthenticated request attempts to list the path exercises for content node "([^"]+)"$`, w.unauthListsPathExercises)

	sc.Step(`^the exercise is created and assigned a stable identifier$`, w.exerciseCreated)
	sc.Step(`^the exercise's type is recorded as (\S+)$`, w.exerciseTypeRecorded)
	sc.Step(`^the exercise carries skill tags "([^"]+)"$`, w.exerciseCarriesSkillTags)
	sc.Step(`^the exercise is not linked to any challenge$`, w.exerciseNotLinked)
	sc.Step(`^the response returns the exercise's title, prompt, type, options, and linked challenges$`, w.exerciseResponseComplete)
	sc.Step(`^the exercise records "([^"]+)" among its linked challenges$`, w.exerciseRecordsLinkedChallenge)
	sc.Step(`^the exercise records both "([^"]+)" and "([^"]+)" among its linked challenges$`, w.exerciseRecordsBothLinkedChallenges)
	sc.Step(`^exactly one exercise "([^"]+)" exists in the system$`, w.exactlyOneExerciseExists)
	sc.Step(`^the exercise no longer records "([^"]+)" among its linked challenges$`, w.exerciseNoLongerRecordsLinkedChallenge)
	sc.Step(`^each returned exercise's options report whether they are correct$`, w.optionsReportCorrectness)
	sc.Step(`^both responses return the exercises in the same, link order$`, w.bothResponsesExercisesSameOrder)
	sc.Step(`^both responses return the same set of exercises$`, w.bothResponsesSameSetOfExercises)
	sc.Step(`^the two responses are not required to return them in the same order$`, func() error { return nil })

	sc.Step(`^the exercise records "([^"]+)" among its linked content nodes$`, w.exerciseRecordsLinkedContentNode)
	sc.Step(`^the exercise no longer records "([^"]+)" among its linked content nodes$`, w.exerciseNoLongerRecordsLinkedContentNode)
	sc.Step(`^both responses include "([^"]+)"$`, w.bothResponsesIncludePathExercise)
	sc.Step(`^both responses return the path exercises in the same, link order$`, w.bothResponsesPathExercisesSameOrder)
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

func (w *world) listsChallengeExercises(name, challengeSlug string) error {
	resp, err := w.handler.ListChallengeExercises(w.ctx(), generated.ListChallengeExercisesRequestObject{ChallengeId: challengeID(challengeSlug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsChallengeExercisesTwice(name, challengeSlug string) error {
	w.multiResp = nil
	if err := w.listsChallengeExercises(name, challengeSlug); err != nil {
		return err
	}
	w.multiResp = append(w.multiResp, w.lastResp)
	if err := w.listsChallengeExercises(name, challengeSlug); err != nil {
		return err
	}
	w.multiResp = append(w.multiResp, w.lastResp)
	return nil
}

func (w *world) listsChallengeExercisesMissing(string) error {
	resp, err := w.handler.ListChallengeExercises(w.ctx(), generated.ListChallengeExercisesRequestObject{ChallengeId: deterministicUUID("challenge", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthListsChallengeExercises(challengeSlug string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsChallengeExercises("", challengeSlug)
}

func (w *world) putChallengeNoShuffle(challengeSlug, nodeSlug string) error {
	w.challenges.put(domain.Challenge{
		ID:            challengeID(challengeSlug).String(),
		ContentNodeID: nodeID(nodeSlug).String(),
		SubjectTag:    "subject-" + challengeSlug,
		PassThreshold: 70,
		CreatedAt:     fixedNow,
	})
	return nil
}

func (w *world) putChallengeShuffled(challengeSlug, nodeSlug string) error {
	w.challenges.put(domain.Challenge{
		ID:               challengeID(challengeSlug).String(),
		ContentNodeID:    nodeID(nodeSlug).String(),
		SubjectTag:       "subject-" + challengeSlug,
		PassThreshold:    70,
		ShuffleExercises: true,
		ShuffleOptions:   true,
		CreatedAt:        fixedNow,
	})
	return nil
}

func (w *world) linksNExercisesToChallenge(countStr, challengeSlug string) error {
	count, err := parseInt(countStr)
	if err != nil {
		return err
	}
	for i := 1; i <= count; i++ {
		slug := fmt.Sprintf("%s-ex-%d", challengeSlug, i)
		label := "option-" + slug
		w.exercises.put(domain.Exercise{
			ID:             exerciseID(slug).String(),
			Title:          "title-" + slug,
			Prompt:         "prompt-" + slug,
			ExerciseType:   domain.ExerciseTypeTextResponse,
			Options:        []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}, {ID: uuid.NewString(), IsCorrect: false, Label: &label}},
			ChallengeIDs:   []string{challengeID(challengeSlug).String()},
			ContentNodeIDs: []string{},
			CreatedAt:      fixedNow,
		})
	}
	return nil
}

func (w *world) optionsReportCorrectness() error {
	resp, ok := w.lastResp.(generated.ListChallengeExercises200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, e := range resp {
		if len(e.Options) == 0 {
			return fmt.Errorf("expected exercise %s to have options, got none", e.ExerciseId)
		}
	}
	return nil
}

func (w *world) bothResponsesExercisesSameOrder() error {
	if len(w.multiResp) != 2 {
		return fmt.Errorf("expected 2 recorded responses, got %d", len(w.multiResp))
	}
	first, ok := w.multiResp[0].(generated.ListChallengeExercises200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a list-exercises response, got %#v", w.multiResp[0])
	}
	second, ok := w.multiResp[1].(generated.ListChallengeExercises200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a list-exercises response, got %#v", w.multiResp[1])
	}
	if len(first) != len(second) {
		return fmt.Errorf("expected both responses to have the same length, got %d and %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ExerciseId != second[i].ExerciseId {
			return fmt.Errorf("expected the same order across both responses, got %v and %v", idsOfExercises(first), idsOfExercises(second))
		}
	}
	return nil
}

func (w *world) bothResponsesSameSetOfExercises() error {
	if len(w.multiResp) != 2 {
		return fmt.Errorf("expected 2 recorded responses, got %d", len(w.multiResp))
	}
	first, ok := w.multiResp[0].(generated.ListChallengeExercises200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a list-exercises response, got %#v", w.multiResp[0])
	}
	second, ok := w.multiResp[1].(generated.ListChallengeExercises200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a list-exercises response, got %#v", w.multiResp[1])
	}
	firstSet, secondSet := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, e := range first {
		firstSet[e.ExerciseId] = true
	}
	for _, e := range second {
		secondSet[e.ExerciseId] = true
	}
	if len(firstSet) != len(secondSet) {
		return fmt.Errorf("expected the same set of exercises, got %v and %v", idsOfExercises(first), idsOfExercises(second))
	}
	for id := range firstSet {
		if !secondSet[id] {
			return fmt.Errorf("expected the same set of exercises, got %v and %v", idsOfExercises(first), idsOfExercises(second))
		}
	}
	return nil
}

func idsOfExercises(exercises []generated.Exercise) []uuid.UUID {
	ids := make([]uuid.UUID, len(exercises))
	for i, e := range exercises {
		ids[i] = e.ExerciseId
	}
	return ids
}

func (w *world) linksExerciseToContentNode(name, exerciseSlug, nodeSlug string) error {
	resp, err := w.handler.LinkExerciseToContentNode(w.ctx(), generated.LinkExerciseToContentNodeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		ExerciseId:    exerciseID(exerciseSlug),
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unlinksExerciseFromContentNode(name, exerciseSlug, nodeSlug string) error {
	resp, err := w.handler.UnlinkExerciseFromContentNode(w.ctx(), generated.UnlinkExerciseFromContentNodeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		ExerciseId:    exerciseID(exerciseSlug),
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) linksMissingExerciseToContentNode(name, nodeSlug string) error {
	resp, err := w.handler.LinkExerciseToContentNode(w.ctx(), generated.LinkExerciseToContentNodeRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		ExerciseId:    deterministicUUID("exercise", "does-not-exist"),
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) linksExerciseToMissingContentNode(name, exerciseSlug string) error {
	resp, err := w.handler.LinkExerciseToContentNode(w.ctx(), generated.LinkExerciseToContentNodeRequestObject{
		ContentNodeId: deterministicUUID("node", "does-not-exist"),
		ExerciseId:    exerciseID(exerciseSlug),
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsPathExercises(name, nodeSlug string) error {
	resp, err := w.handler.ListContentNodePathExercises(w.ctx(), generated.ListContentNodePathExercisesRequestObject{ContentNodeId: nodeID(nodeSlug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsPathExercisesTwice(name, nodeSlug string) error {
	w.multiResp = nil
	if err := w.listsPathExercises(name, nodeSlug); err != nil {
		return err
	}
	w.multiResp = append(w.multiResp, w.lastResp)
	if err := w.listsPathExercises(name, nodeSlug); err != nil {
		return err
	}
	w.multiResp = append(w.multiResp, w.lastResp)
	return nil
}

func (w *world) listsPathExercisesMissing(string) error {
	resp, err := w.handler.ListContentNodePathExercises(w.ctx(), generated.ListContentNodePathExercisesRequestObject{ContentNodeId: deterministicUUID("node", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthLinksPathExercise(nodeSlug string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.linksExerciseToContentNode("", "does-not-exist", nodeSlug)
}

func (w *world) unauthListsPathExercises(nodeSlug string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsPathExercises("", nodeSlug)
}

func (w *world) exerciseRecordsLinkedContentNode(nodeSlug string) error {
	want := nodeID(nodeSlug)
	switch resp := w.lastResp.(type) {
	case generated.LinkExerciseToContentNode201JSONResponse:
		for _, id := range resp.ContentNodeIds {
			if id == want {
				return nil
			}
		}
		return fmt.Errorf("expected content_node_ids to contain %s, got %v", want, resp.ContentNodeIds)
	case generated.LinkExerciseToChallenge201JSONResponse:
		for _, id := range resp.ContentNodeIds {
			if id == want {
				return nil
			}
		}
		return fmt.Errorf("expected content_node_ids to contain %s, got %v", want, resp.ContentNodeIds)
	default:
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
}

func (w *world) exerciseNoLongerRecordsLinkedContentNode(nodeSlug string) error {
	if _, ok := w.lastResp.(generated.UnlinkExerciseFromContentNode204Response); !ok {
		return fmt.Errorf("expected a 204 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) bothResponsesIncludePathExercise(slug string) error {
	if len(w.multiResp) != 2 {
		return fmt.Errorf("expected 2 recorded responses, got %d", len(w.multiResp))
	}
	want := exerciseID(slug)
	for i, r := range w.multiResp {
		resp, ok := r.(generated.ListContentNodePathExercises200JSONResponse)
		if !ok {
			return fmt.Errorf("expected a path-exercises list response at index %d, got %#v", i, r)
		}
		found := false
		for _, e := range resp {
			if e.ExerciseId == want {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("expected response %d to include %s, got %v", i, want, resp)
		}
	}
	return nil
}

func (w *world) bothResponsesPathExercisesSameOrder() error {
	if len(w.multiResp) != 2 {
		return fmt.Errorf("expected 2 recorded responses, got %d", len(w.multiResp))
	}
	first, ok := w.multiResp[0].(generated.ListContentNodePathExercises200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a path-exercises list response, got %#v", w.multiResp[0])
	}
	second, ok := w.multiResp[1].(generated.ListContentNodePathExercises200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a path-exercises list response, got %#v", w.multiResp[1])
	}
	if len(first) != len(second) {
		return fmt.Errorf("expected both responses to have the same length, got %d and %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ExerciseId != second[i].ExerciseId {
			return fmt.Errorf("expected the same order across both responses")
		}
	}
	return nil
}
