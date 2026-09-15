//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerPracticeSessionSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^(\d+) exercises tagged "([^"]+)" exist in the system$`, w.putNExercisesTagged)

	sc.Step(`^"([^"]+)" starts a practice session for skill tag "([^"]+)" with count (\d+)$`, w.startsPracticeSession)
	sc.Step(`^"([^"]+)" starts a practice session for skill tag "([^"]+)" without specifying a count$`, w.startsPracticeSessionNoCount)
	sc.Step(`^"([^"]+)" starts two practice sessions for skill tag "([^"]+)" with count (\d+)$`, w.startsTwoPracticeSessions)
	sc.Step(`^"([^"]+)" submits a start practice session request with the skill_tag field omitted$`, w.submitsPracticeSessionMissingSkillTag)
	sc.Step(`^"([^"]+)" submits a start practice session request with skill_tag "([^"]+)" and count (\d+)$`, w.submitsPracticeSessionWithSkillTagAndCount)
	sc.Step(`^an unauthenticated request attempts to start a practice session for skill tag "([^"]+)"$`, w.unauthStartsPracticeSession)

	sc.Step(`^the practice session contains (\d+) exercises$`, w.practiceSessionContains)
	sc.Step(`^every exercise in the practice session is tagged "([^"]+)"$`, w.everyExerciseInPracticeSessionTagged)
	sc.Step(`^the practice session is assigned a stable practice_session_id$`, w.practiceSessionHasStableID)
	sc.Step(`^the two practice sessions are assigned different practice_session_ids$`, w.twoPracticeSessionsDifferentIDs)
}

func (w *world) putNExercisesTagged(countStr, skillTag string) error {
	count, err := parseInt(countStr)
	if err != nil {
		return err
	}
	for i := 1; i <= count; i++ {
		slug := fmt.Sprintf("%s-tagged-%d", skillTag, i)
		label := "option-" + slug
		w.exercises.put(domain.Exercise{
			ID:             exerciseID(slug).String(),
			Title:          "title-" + slug,
			Prompt:         "prompt-" + slug,
			ExerciseType:   domain.ExerciseTypeTextResponse,
			SkillTags:      []string{skillTag},
			Options:        []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}, {ID: uuid.NewString(), IsCorrect: false, Label: &label}},
			ChallengeIDs:   []string{},
			ContentNodeIDs: []string{},
			CreatedAt:      fixedNow,
		})
	}
	return nil
}

func (w *world) startsPracticeSession(name, skillTag, countStr string) error {
	count, err := parseInt(countStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.StartPracticeSession(w.ctx(), generated.StartPracticeSessionRequestObject{
		Params: generated.StartPracticeSessionParams{SkillTag: skillTag, Count: &count},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) startsPracticeSessionNoCount(name, skillTag string) error {
	resp, err := w.handler.StartPracticeSession(w.ctx(), generated.StartPracticeSessionRequestObject{
		Params: generated.StartPracticeSessionParams{SkillTag: skillTag},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) startsTwoPracticeSessions(name, skillTag, countStr string) error {
	w.multiResp = nil
	if err := w.startsPracticeSession(name, skillTag, countStr); err != nil {
		return err
	}
	w.multiResp = append(w.multiResp, w.lastResp)
	if err := w.startsPracticeSession(name, skillTag, countStr); err != nil {
		return err
	}
	w.multiResp = append(w.multiResp, w.lastResp)
	return nil
}

func (w *world) submitsPracticeSessionMissingSkillTag(string) error {
	resp, err := w.handler.StartPracticeSession(w.ctx(), generated.StartPracticeSessionRequestObject{
		Params: generated.StartPracticeSessionParams{},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsPracticeSessionWithSkillTagAndCount(name, skillTag, countStr string) error {
	return w.startsPracticeSession(name, skillTag, countStr)
}

func (w *world) unauthStartsPracticeSession(skillTag string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.startsPracticeSessionNoCount("", skillTag)
}

func (w *world) practiceSessionContains(countStr string) error {
	count, err := parseInt(countStr)
	if err != nil {
		return err
	}
	resp, ok := w.lastResp.(generated.StartPracticeSession200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(resp.Exercises) != count {
		return fmt.Errorf("expected %d exercises, got %d", count, len(resp.Exercises))
	}
	return nil
}

func (w *world) everyExerciseInPracticeSessionTagged(skillTag string) error {
	resp, ok := w.lastResp.(generated.StartPracticeSession200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, e := range resp.Exercises {
		if e.SkillTags == nil {
			return fmt.Errorf("expected exercise %s to carry skill tags, got none", e.ExerciseId)
		}
		found := false
		for _, tag := range *e.SkillTags {
			if tag == skillTag {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("expected exercise %s to be tagged %q, got %v", e.ExerciseId, skillTag, *e.SkillTags)
		}
	}
	return nil
}

func (w *world) practiceSessionHasStableID() error {
	resp, ok := w.lastResp.(generated.StartPracticeSession200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.PracticeSessionId == uuid.Nil {
		return fmt.Errorf("expected a non-nil practice_session_id")
	}
	return nil
}

func (w *world) twoPracticeSessionsDifferentIDs() error {
	if len(w.multiResp) != 2 {
		return fmt.Errorf("expected 2 recorded responses, got %d", len(w.multiResp))
	}
	first, ok := w.multiResp[0].(generated.StartPracticeSession200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a practice-session response, got %#v", w.multiResp[0])
	}
	second, ok := w.multiResp[1].(generated.StartPracticeSession200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a practice-session response, got %#v", w.multiResp[1])
	}
	if first.PracticeSessionId == second.PracticeSessionId {
		return fmt.Errorf("expected different practice_session_ids, got the same: %s", first.PracticeSessionId)
	}
	return nil
}
