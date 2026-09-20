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
	sc.Step(`^(\d+) exercises linked to skill "([^"]+)" exist in the system$`, w.putNExercisesLinkedToSkill)

	sc.Step(`^"([^"]+)" starts a practice session for skill "([^"]+)" with count (\d+)$`, w.startsPracticeSession)
	sc.Step(`^"([^"]+)" starts a practice session for skill "([^"]+)" without specifying a count$`, w.startsPracticeSessionNoCount)
	sc.Step(`^"([^"]+)" starts two practice sessions for skill "([^"]+)" with count (\d+)$`, w.startsTwoPracticeSessions)
	sc.Step(`^"([^"]+)" submits a start practice session request with the skill_id field omitted$`, w.submitsPracticeSessionMissingSkillID)
	sc.Step(`^"([^"]+)" submits a start practice session request with skill "([^"]+)" and count (\d+)$`, w.submitsPracticeSessionWithSkillAndCount)
	sc.Step(`^an unauthenticated request attempts to start a practice session for skill "([^"]+)"$`, w.unauthStartsPracticeSession)

	sc.Step(`^the practice session contains (\d+) exercises$`, w.practiceSessionContains)
	sc.Step(`^every exercise in the practice session is linked to skill "([^"]+)"$`, w.everyExerciseInPracticeSessionLinkedToSkill)
	sc.Step(`^the practice session is assigned a stable practice_session_id$`, w.practiceSessionHasStableID)
	sc.Step(`^the two practice sessions are assigned different practice_session_ids$`, w.twoPracticeSessionsDifferentIDs)
}

func (w *world) putNExercisesLinkedToSkill(countStr, skillName string) error {
	count, err := parseInt(countStr)
	if err != nil {
		return err
	}
	skillID := w.skillIDFor(skillName)
	conceptID := w.conceptIDFor("concept-for-" + skillName)
	for i := 1; i <= count; i++ {
		slug := fmt.Sprintf("%s-linked-%d", skillName, i)
		label := "option-" + slug
		w.exercises.put(domain.Exercise{
			ID:             exerciseID(slug).String(),
			Title:          "title-" + slug,
			Prompt:         domain.NewPlainTextPrompt("prompt-" + slug),
			ExerciseType:   domain.ExerciseTypeTextResponse,
			Skills:         []domain.Skill{{ID: skillID.String(), Name: skillName}},
			Concepts:       []domain.Concept{{ID: conceptID.String(), Name: "concept-for-" + skillName}},
			Options:        []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}, {ID: uuid.NewString(), IsCorrect: false, Label: &label}},
			ChallengeIDs:   []string{},
			ContentNodeIDs: []string{},
			CreatedAt:      fixedNow,
		})
	}
	return nil
}

func (w *world) startsPracticeSession(name, skillName, countStr string) error {
	count, err := parseInt(countStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.StartPracticeSession(w.ctx(), generated.StartPracticeSessionRequestObject{
		Params: generated.StartPracticeSessionParams{SkillId: w.skillIDFor(skillName), Count: &count},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) startsPracticeSessionNoCount(name, skillName string) error {
	resp, err := w.handler.StartPracticeSession(w.ctx(), generated.StartPracticeSessionRequestObject{
		Params: generated.StartPracticeSessionParams{SkillId: w.skillIDFor(skillName)},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) startsTwoPracticeSessions(name, skillName, countStr string) error {
	w.multiResp = nil
	if err := w.startsPracticeSession(name, skillName, countStr); err != nil {
		return err
	}
	w.multiResp = append(w.multiResp, w.lastResp)
	if err := w.startsPracticeSession(name, skillName, countStr); err != nil {
		return err
	}
	w.multiResp = append(w.multiResp, w.lastResp)
	return nil
}

func (w *world) submitsPracticeSessionMissingSkillID(string) error {
	resp, err := w.handler.StartPracticeSession(w.ctx(), generated.StartPracticeSessionRequestObject{
		Params: generated.StartPracticeSessionParams{},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsPracticeSessionWithSkillAndCount(name, skillName, countStr string) error {
	return w.startsPracticeSession(name, skillName, countStr)
}

func (w *world) unauthStartsPracticeSession(skillName string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.startsPracticeSessionNoCount("", skillName)
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

func (w *world) everyExerciseInPracticeSessionLinkedToSkill(skillName string) error {
	resp, ok := w.lastResp.(generated.StartPracticeSession200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	want := w.skillIDFor(skillName)
	for _, e := range resp.Exercises {
		found := false
		for _, s := range e.Skills {
			if s.SkillId == want {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("expected exercise %s to be linked to skill %q, got %+v", e.ExerciseId, skillName, e.Skills)
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
