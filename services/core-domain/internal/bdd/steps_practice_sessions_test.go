//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"time"

	"github.com/cucumber/godog"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// practiceSkill is the skill the practising student's path teaches; every
// play-along a practice scenario sets up is classified under it.
const practiceSkill = "practice-skill"

// pickMinutes is the length of the session a "picked as" step composes:
// long enough for two play-alongs, short of a warm-up.
const pickMinutes = 4

func registerPracticeSessionSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^student "([^"]+)" plays "([^"]+)" and "([^"]+)"$`, w.studentPlays)
	sc.Step(`^"([^"]+)" is enrolled in a path whose skills have practice items for both instruments$`, w.enrolledInPracticePath)
	sc.Step(`^"([^"]+)" has a clean play-along "([^"]+)" with a best clean tempo of (\d+) BPM$`, w.hasCleanPlayAlong)
	sc.Step(`^"([^"]+)"'s best clean tempo on "([^"]+)" is (\d+) BPM and the diagram's tempo is (\d+) BPM$`, w.hasBestCleanTempoOn)
	sc.Step(`^"([^"]+)" has never rated "([^"]+)" clean and the diagram's tempo is (\d+) BPM$`, w.neverRatedClean)

	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with "([^"]+)" in hand$`, w.composesSession)
	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with an instrument that doesn't exist in hand$`, w.composesSessionWithMissingInstrument)
	sc.Step(`^"([^"]+)" is picked as a (due|new) item$`, w.isPickedAs)

	sc.Step(`^the session starts with "([^"]+)" with the reason (\w+) at (\d+) BPM$`, w.sessionStartsWith)
	sc.Step(`^no item in the session has the reason (\w+)$`, w.noItemHasReason)
	sc.Step(`^every item in the session suits "([^"]+)" or every instrument$`, w.everyItemSuits)
	sc.Step(`^it starts at (\d+) BPM with a target of (\d+) BPM$`, w.pickedStartsAt)
}

func (w *world) studentPlays(name, first, second string) error {
	w.authenticateAs(name, domain.RoleStudent)
	for _, instrument := range []string{first, second} {
		if _, err := w.ensureInstrumentSeeded(instrument); err != nil {
			return err
		}
	}
	return nil
}

// enrolledInPracticePath gives name a standalone path teaching
// practiceSkill, with a play-along for each of the student's instruments.
func (w *world) enrolledInPracticePath(name string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	node := domain.ContentNode{
		ID:             nodeID("practice-lesson").String(),
		Title:          "practice-lesson",
		ContentType:    domain.ContentTypeArticle,
		Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: w.skillIDFor(practiceSkill).String()}}},
	}
	w.nodes.put(node)
	w.studentPaths.put(domain.StudentPath{
		ID:               deterministicUUID("student-path", name+"/practice").String(),
		StudentID:        studentID,
		SourceTemplateID: pathID("practice-path").String(),
		AssignedAt:       fixedNow,
		Items:            []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID}},
	})
	w.putPlayAlong("guitar-lick", "guitar", 100)
	w.putPlayAlong("bass-line", "electric-bass", 100)
	return nil
}

// putPlayAlong seeds a basic diagram on instrument with eight quarter notes
// of playback at tempo, classified under practiceSkill.
func (w *world) putPlayAlong(slug, instrument string, tempo int) {
	d := domain.Diagram{
		ID:            diagramID(slug).String(),
		InstrumentID:  instrumentID(instrument).String(),
		InstrumentIDs: []string{instrumentID(instrument).String()},
		Names:         domain.LocalizedText{"en": slug},
		Kind:          domain.DiagramKindBasic,
		CreatedBy:     w.curatorID(),
		TimeSignature: domain.DefaultTimeSignature,
		TempoBPM:      &tempo,
		Skills:        []domain.KnowledgeNode{{ID: w.skillIDFor(practiceSkill).String()}},
		CreatedAt:     fixedNow,
	}
	for range 8 {
		d.Sequence = append(d.Sequence, domain.SequenceStep{PositionIDs: []string{"p1"}, Value: domain.NoteValue{Num: 1, Den: 4}, Strum: domain.StrumNone})
	}
	w.diagrams.put(d)
}

func (w *world) putPlayAlongState(name, slug string, s domain.PracticeItemState) {
	s.ItemKey = domain.PlayAlongItemKey(diagramID(slug).String())
	s.RulesVersion = domain.PracticeRulesVersion
	w.practiceStates.put(w.ensureRegistered(name, domain.RoleStudent).String(), s)
}

func (w *world) hasCleanPlayAlong(name, slug string, bestClean int) error {
	w.putPlayAlong(slug, "guitar", 120)
	dueAt := fixedNow.AddDate(0, 0, 3)
	w.putPlayAlongState(name, slug, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 3, DueAt: &dueAt, LastAt: &fixedNow, BestCleanBPM: &bestClean})
	return nil
}

func (w *world) hasBestCleanTempoOn(name, slug string, bestClean, tempo int) error {
	w.putPlayAlong(slug, "guitar", tempo)
	dueAt := fixedNow.Add(-time.Hour)
	w.putPlayAlongState(name, slug, domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: &dueAt, LastAt: &fixedNow, BestCleanBPM: &bestClean})
	return nil
}

func (w *world) neverRatedClean(_, slug string, tempo int) error {
	w.putPlayAlong(slug, "guitar", tempo)
	return nil
}

func (w *world) composeSession(minutes int, instrument *openapi_types.UUID) error {
	resp, err := w.handler.CreatePracticeSessionPlan(w.ctx(), generated.CreatePracticeSessionPlanRequestObject{
		Body: &generated.CreatePracticeSessionPlanRequest{InstrumentId: instrument, Minutes: minutes},
	})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) composesSession(_ string, minutes int, instrument string) error {
	id := instrumentID(instrument)
	return w.composeSession(minutes, &id)
}

func (w *world) composesSessionWithMissingInstrument(_ string, minutes int) error {
	id := instrumentID("no-such-instrument")
	return w.composeSession(minutes, &id)
}

func (w *world) isPickedAs(slug, reason string) error {
	w.pickedSlug, w.pickedReason = slug, reason
	id := instrumentID("guitar")
	return w.composeSession(pickMinutes, &id)
}

func (w *world) composedPlan() (generated.PracticeSessionPlan, error) {
	if w.lastErr != nil {
		return generated.PracticeSessionPlan{}, w.lastErr
	}
	plan, ok := w.lastResp.(generated.CreatePracticeSessionPlan200JSONResponse)
	if !ok {
		return generated.PracticeSessionPlan{}, fmt.Errorf("expected a composed session, got %#v", w.lastResp)
	}
	if len(plan.Items) == 0 {
		return generated.PracticeSessionPlan{}, fmt.Errorf("expected a session with items, got none")
	}
	return generated.PracticeSessionPlan(plan), nil
}

func (w *world) sessionStartsWith(slug, reason string, tempo int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	first := plan.Items[0]
	if first.PlayAlong == nil || first.PlayAlong.DiagramId != diagramID(slug) {
		return fmt.Errorf("expected the session to start with %q, got %+v", slug, first)
	}
	if string(first.Reason) != reason || first.PlayAlong.StartTempoBpm != tempo {
		return fmt.Errorf("expected %q with the reason %s at %d BPM, got %s at %d BPM", slug, reason, tempo, first.Reason, first.PlayAlong.StartTempoBpm)
	}
	return nil
}

func (w *world) noItemHasReason(reason string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if string(item.Reason) == reason {
			return fmt.Errorf("expected no item with the reason %s, got %s", reason, item.ItemKey)
		}
	}
	return nil
}

func (w *world) everyItemSuits(instrument string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if item.PlayAlong == nil {
			return fmt.Errorf("expected only play-alongs, got %s", item.ItemKey)
		}
		d, err := w.diagrams.GetByID(w.ctx(), item.PlayAlong.DiagramId.String())
		if err != nil {
			return err
		}
		if !slices.Contains(d.InstrumentIDs, instrumentID(instrument).String()) {
			return fmt.Errorf("expected every item to suit %q, but %s suits %v", instrument, item.ItemKey, d.InstrumentIDs)
		}
	}
	return nil
}

func (w *world) pickedStartsAt(start, target int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if item.PlayAlong == nil || item.PlayAlong.DiagramId != diagramID(w.pickedSlug) {
			continue
		}
		if string(item.Reason) != w.pickedReason {
			return fmt.Errorf("expected %q to be picked as %s, got %s", w.pickedSlug, w.pickedReason, item.Reason)
		}
		if item.PlayAlong.StartTempoBpm != start || item.PlayAlong.TargetTempoBpm != target {
			return fmt.Errorf("expected %q to start at %d BPM with a target of %d BPM, got %d and %d", w.pickedSlug, start, target, item.PlayAlong.StartTempoBpm, item.PlayAlong.TargetTempoBpm)
		}
		return nil
	}
	return fmt.Errorf("expected %q in the session, got %+v", w.pickedSlug, plan.Items)
}
