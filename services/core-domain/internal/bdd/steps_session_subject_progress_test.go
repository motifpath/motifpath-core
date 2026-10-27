//go:build integration

package bdd

import (
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// Steps of how a session moved the student on its subjects: each subject's
// rung and level in the plan, and the subject progress read once the
// session is caught up.
func registerSessionSubjectProgressSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" is learning every know item of "([^"]+)" on guitar$`, w.isLearningOnKnow)
	sc.Step(`^"([^"]+)" practised session "([^"]+)" with "([a-z-]+)" in hand, and became accurate on every know item of "([^"]+)"$`, w.practisedSessionBecomingAccurate)
	sc.Step(`^the session "([^"]+)"'s end is recorded$`, w.sessionEndIsRecorded)
	sc.Step(`^"([^"]+)" practised session "([^"]+)" with "([a-z-]+)" in hand, and its end has not been recorded yet$`, w.practisedSessionNotEnded)
	sc.Step(`^"([^"]+)" has items at the know rung on guitar$`, w.hasKnowCellsOnGuitar)
	sc.Step(`^the session "([^"]+)" of "([^"]+)" with "([a-z-]+)" in hand has its end recorded$`, w.sessionInHandEnded)
	sc.Step(`^the session "([^"]+)" of "([^"]+)" with no instrument in hand has its end recorded$`, w.sessionInTheHeadEnded)
	sc.Step(`^"([^"]+)" is accurate on every know item of "([^"]+)" on guitar, and learning on electric-bass$`, w.isAccurateOnGuitarLearningOnBass)

	sc.Step(`^"([^"]+)" reads the subject progress of session "([^"]+)" with "([a-z-]+)" in hand, for "([^"]+)"$`, func(name, session, instrument, node string) error {
		return w.readsSubjectProgress(name, sessionUUID(session), w.instrumentPtr(instrument), node)
	})
	sc.Step(`^"([^"]+)" reads the subject progress of session "([^"]+)" with "([a-z-]+)" in hand, for "([^"]+)" and "([^"]+)"$`, func(name, session, instrument, a, b string) error {
		return w.readsSubjectProgress(name, sessionUUID(session), w.instrumentPtr(instrument), a, b)
	})
	sc.Step(`^"([^"]+)" reads the subject progress of session "([^"]+)" with no instrument in hand, for "([^"]+)"$`, func(name, session, node string) error {
		return w.readsSubjectProgress(name, sessionUUID(session), nil, node)
	})
	sc.Step(`^"([^"]+)" reads the subject progress of session "([^"]+)" with "([a-z-]+)" in hand, for "([^"]+)" twice$`, w.readsSubjectProgressTwice)
	sc.Step(`^"([^"]+)" reads the subject progress of a session never started, with "([a-z-]+)" in hand, for "([^"]+)"$`, func(name, instrument, node string) error {
		return w.readsSubjectProgress(name, sessionUUID("never-started"), w.instrumentPtr(instrument), node)
	})
	sc.Step(`^"([^"]+)" reads the subject progress of session "([^"]+)" with "([a-z-]+)" in hand, for no node$`, func(name, session, instrument string) error {
		return w.readSubjectProgressOf(name, sessionUUID(session), w.instrumentPtr(instrument), nil)
	})
	sc.Step(`^"([^"]+)" reads the subject progress of session "([^"]+)" with "([a-z-]+)" in hand, for four nodes$`, func(name, session, instrument string) error {
		var ids []openapi_types.UUID
		for i := range 4 {
			ids = append(ids, w.nodeIDByName(focusSkill(i)))
		}
		return w.readSubjectProgressOf(name, sessionUUID(session), w.instrumentPtr(instrument), ids)
	})
	sc.Step(`^"([^"]+)" reads the subject progress of session "([^"]+)" with "([a-z-]+)" in hand, for a node that doesn't exist$`, func(name, session, instrument string) error {
		missing := uuid.NewSHA1(uuid.NameSpaceOID, []byte("no-such-node"))
		return w.readSubjectProgressOf(name, sessionUUID(session), w.instrumentPtr(instrument), []openapi_types.UUID{missing})
	})

	sc.Step(`^the plan's focus node "([^"]+)" is at the (\w+) rung, at level (\w+)$`, w.focusNodeIsAtRung)
	sc.Step(`^the session is caught up$`, func() error { return w.sessionIsCaughtUp(true) })
	sc.Step(`^the session is not caught up$`, func() error { return w.sessionIsCaughtUp(false) })
	sc.Step(`^"([^"]+)" is accurate at the know rung and new at the hear and play_clean rungs$`, w.accurateAtKnowNewAbove)
	sc.Step(`^"([^"]+)" is (\w+) at the (\w+) rung$`, w.subjectIsAtRungLevel)
	sc.Step(`^"([^"]+)"'s current rung is (\w+)$`, w.subjectCurrentRungIs)
	sc.Step(`^"([^"]+)" still shows its rungs and current rung as they are now$`, w.subjectShowsRungsAsNow)
	sc.Step(`^the subjects are "([^"]+)" then "([^"]+)"$`, w.subjectsAreInOrder)
	sc.Step(`^both reads are the same$`, w.bothReadsAreTheSame)
}

// sessionUUID is the practice session id a scenario names.
func sessionUUID(name string) openapi_types.UUID {
	return deterministicUUID("practice-session", name)
}

func (w *world) instrumentPtr(instrument string) *openapi_types.UUID {
	id := instrumentID(instrument)
	return &id
}

func (w *world) isLearningOnKnow(name, skill string) error {
	know, _ := w.putRungItems(name, skill)
	w.putItemStates(name, domain.KnowledgeLevelLearning, 2, 1, 2, know...)
	return nil
}

// recordProgressSession records a session of name the worker has seen start, ended
// or not, as the worker keeps it.
func (w *world) recordProgressSession(name, session string, instrument *string, ended bool) {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	started := fixedNow.Add(-15 * time.Minute)
	s := fakePracticeSession{sessionID: sessionUUID(session).String(), instrumentID: instrument, startedAt: &started, lastEventAt: fixedNow}
	if ended {
		s.endedAt = &fixedNow
	}
	w.practiceActivity.addSession(studentID, s)
	w.subjectProgressStudent = name
}

func instrumentIDString(instrument string) *string {
	id := instrumentID(instrument).String()
	return &id
}

func (w *world) practisedSessionBecomingAccurate(name, session, instrument, skill string) error {
	know, _ := w.putRungItems(name, skill)
	w.accurateItems(name, know...)
	w.recordProgressSession(name, session, instrumentIDString(instrument), false)
	return nil
}

func (w *world) sessionEndIsRecorded(session string) error {
	return w.practiceActivity.endSession(sessionUUID(session).String(), fixedNow)
}

func (w *world) practisedSessionNotEnded(name, session, instrument string) error {
	w.recordProgressSession(name, session, instrumentIDString(instrument), false)
	return nil
}

func (w *world) hasKnowCellsOnGuitar(skill string) error {
	w.putCells("guitar", skill, 12, 6, 5)
	return nil
}

func (w *world) sessionInHandEnded(session, name, instrument string) error {
	w.recordProgressSession(name, session, instrumentIDString(instrument), true)
	return nil
}

func (w *world) sessionInTheHeadEnded(session, name string) error {
	w.recordProgressSession(name, session, nil, true)
	return nil
}

// isAccurateOnGuitarLearningOnBass makes name accurate on every guitar cell
// of skill and learning on as many electric bass cells of it.
func (w *world) isAccurateOnGuitarLearningOnBass(name, skill string) error {
	w.putCells("electric-bass", skill, 12, 4, 3)
	for instrument, level := range map[string]domain.KnowledgeLevel{"guitar": domain.KnowledgeLevelAccurate, "electric-bass": domain.KnowledgeLevelLearning} {
		var keys []string
		for _, item := range w.practiceItems.cells[instrumentID(instrument).String()] {
			if slices.Contains(item.NodeIDs, w.skillIDFor(skill).String()) {
				keys = append(keys, item.ItemKey)
			}
		}
		w.putItemStates(name, level, 3, 2, 5, keys...)
	}
	return nil
}

func (w *world) readSubjectProgressOf(name string, session openapi_types.UUID, instrument *openapi_types.UUID, nodes []openapi_types.UUID) error {
	w.authenticateAs(name, domain.RoleStudent)
	resp, err := w.handler.GetPracticeSessionSubjectProgress(w.ctx(), generated.GetPracticeSessionSubjectProgressRequestObject{
		PracticeSessionId: session,
		Params:            generated.GetPracticeSessionSubjectProgressParams{InstrumentId: instrument, NodeId: nodes},
	})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) readsSubjectProgress(name string, session openapi_types.UUID, instrument *openapi_types.UUID, nodes ...string) error {
	ids := make([]openapi_types.UUID, len(nodes))
	for i, n := range nodes {
		ids[i] = w.nodeIDByName(n)
	}
	return w.readSubjectProgressOf(name, session, instrument, ids)
}

func (w *world) readsSubjectProgressTwice(name, session, instrument, node string) error {
	if err := w.readsSubjectProgress(name, sessionUUID(session), w.instrumentPtr(instrument), node); err != nil {
		return err
	}
	first, err := w.subjectProgress()
	if err != nil {
		return err
	}
	w.firstSubjectProgress = &first
	return w.readsSubjectProgress(name, sessionUUID(session), w.instrumentPtr(instrument), node)
}

func (w *world) subjectProgress() (generated.PracticeSessionSubjectProgress, error) {
	if w.lastErr != nil {
		return generated.PracticeSessionSubjectProgress{}, w.lastErr
	}
	resp, ok := w.lastResp.(generated.GetPracticeSessionSubjectProgress200JSONResponse)
	if !ok {
		return generated.PracticeSessionSubjectProgress{}, fmt.Errorf("expected the subject progress, got %#v", w.lastResp)
	}
	return generated.PracticeSessionSubjectProgress(resp), nil
}

// progressOf finds node among the read's subjects.
func (w *world) progressOf(node string) (generated.PracticeSubjectProgress, error) {
	progress, err := w.subjectProgress()
	if err != nil {
		return generated.PracticeSubjectProgress{}, err
	}
	for _, s := range progress.Subjects {
		if s.NodeId == w.nodeIDByName(node) {
			return s, nil
		}
	}
	return generated.PracticeSubjectProgress{}, fmt.Errorf("%q is not among the subjects read", node)
}

// rungLevel is node's level at rung in the read; empty when the read has no such rung.
func rungLevel(s generated.PracticeSubjectProgress, rung string) generated.KnowledgeLevel {
	for _, r := range s.Rungs {
		if string(r.Rung) == rung {
			return r.Level
		}
	}
	return ""
}

func (w *world) focusNodeIsAtRung(node, rung, level string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, n := range plan.FocusNodes {
		if n.NodeId != w.nodeIDByName(node) {
			continue
		}
		if string(n.Rung) != rung || string(n.RungLevel) != level {
			return fmt.Errorf("expected %q at the %s rung, at level %s, got the %s rung at level %s", node, rung, level, n.Rung, n.RungLevel)
		}
		return nil
	}
	return fmt.Errorf("%q is not a focus node of the plan", node)
}

func (w *world) sessionIsCaughtUp(want bool) error {
	progress, err := w.subjectProgress()
	if err != nil {
		return err
	}
	if progress.CaughtUp != want {
		return fmt.Errorf("expected caught_up %t, got %t", want, progress.CaughtUp)
	}
	return nil
}

func (w *world) accurateAtKnowNewAbove(node string) error {
	s, err := w.progressOf(node)
	if err != nil {
		return err
	}
	want := map[string]generated.KnowledgeLevel{"know": "accurate", "hear": "new", "play_clean": "new"}
	for rung, level := range want {
		if got := rungLevel(s, rung); got != level {
			return fmt.Errorf("expected %q %s at the %s rung, got %q (rungs %v)", node, level, rung, got, s.Rungs)
		}
	}
	return nil
}

func (w *world) subjectIsAtRungLevel(node, level, rung string) error {
	s, err := w.progressOf(node)
	if err != nil {
		return err
	}
	if got := rungLevel(s, rung); string(got) != level {
		return fmt.Errorf("expected %q %s at the %s rung, got %q (rungs %v)", node, level, rung, got, s.Rungs)
	}
	return nil
}

func (w *world) subjectCurrentRungIs(node, rung string) error {
	s, err := w.progressOf(node)
	if err != nil {
		return err
	}
	if s.CurrentRung == nil || string(*s.CurrentRung) != rung {
		return fmt.Errorf("expected %q's current rung %s, got %v", node, rung, s.CurrentRung)
	}
	return nil
}

// subjectShowsRungsAsNow checks the read still gives node's rungs, its
// current rung the same as the subjects list gives now.
func (w *world) subjectShowsRungsAsNow(node string) error {
	s, err := w.progressOf(node)
	if err != nil {
		return err
	}
	if len(s.Rungs) == 0 || s.CurrentRung == nil {
		return fmt.Errorf("expected %q to show its rungs and current rung, got %v and %v", node, s.Rungs, s.CurrentRung)
	}
	saved, savedErr := w.lastResp, w.lastErr
	defer func() { w.lastResp, w.lastErr = saved, savedErr }()
	if err := w.listsSubjectsInHand(w.subjectProgressStudent, "guitar"); err != nil {
		return err
	}
	listed, _, err := w.listedSubject(node)
	if err != nil {
		return err
	}
	if listed.CurrentRung != *s.CurrentRung {
		return fmt.Errorf("expected %q's current rung as listed now, %s, got %s", node, listed.CurrentRung, *s.CurrentRung)
	}
	return nil
}

func (w *world) subjectsAreInOrder(first, second string) error {
	progress, err := w.subjectProgress()
	if err != nil {
		return err
	}
	got := make([]openapi_types.UUID, len(progress.Subjects))
	for i, s := range progress.Subjects {
		got[i] = s.NodeId
	}
	if want := []openapi_types.UUID{w.nodeIDByName(first), w.nodeIDByName(second)}; !slices.Equal(got, want) {
		return fmt.Errorf("expected the subjects %v, got %v", want, got)
	}
	return nil
}

func (w *world) bothReadsAreTheSame() error {
	second, err := w.subjectProgress()
	if err != nil {
		return err
	}
	if w.firstSubjectProgress == nil || !reflect.DeepEqual(*w.firstSubjectProgress, second) {
		return fmt.Errorf("expected both reads the same, got %#v then %#v", w.firstSubjectProgress, second)
	}
	return nil
}
