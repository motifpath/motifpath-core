//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// practiceSkill is the skill the practising student's path teaches; every
// play-along a practice scenario sets up is classified under it.
const practiceSkill = "practice-skill"

// pickMinutes is the length of the session a "picked as" step composes:
// long enough for a play-along to fit the new share of its focus time.
const pickMinutes = 20

func registerPracticeSessionSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^student "([^"]+)" plays "([^"]+)" and "([^"]+)"$`, w.studentPlays)
	sc.Step(`^"([^"]+)" is enrolled in a path whose skills have practice items for both instruments$`, w.enrolledInPracticePath)
	sc.Step(`^"([^"]+)" has a clean play-along "([^"]+)" with a best clean tempo of (\d+) BPM$`, w.hasCleanPlayAlong)
	sc.Step(`^"([^"]+)"'s best clean tempo on "([^"]+)" is (\d+) BPM and the diagram's tempo is (\d+) BPM$`, w.hasBestCleanTempoOn)
	sc.Step(`^"([^"]+)" has never rated "([^"]+)" clean and the diagram's tempo is (\d+) BPM$`, w.neverRatedClean)

	sc.Step(`^"([^"]+)" has plenty of due, weak and new items on guitar$`, w.hasPlentyOfEverything)
	sc.Step(`^"([^"]+)" has (plenty of|no) due items, (plenty of|no) weak items and (plenty of|no) new items on guitar$`, w.hasItemsOnGuitar)
	sc.Step(`^"([^"]+)" has (\d+) minutes of due items, nothing weak and (\d+) minutes? of new items on guitar, well under their shares of the focus time$`, w.hasMinutesOfItems)
	sc.Step(`^"([^"]+)" has (\d+) due items and (\d+) new items on guitar$`, w.hasDueAndNewItems)
	sc.Step(`^"([^"]+)" has known items coming due within the week$`, w.hasKnownItemsComingDue)
	sc.Step(`^"([^"]+)" has nothing due, weak or new on their path for guitar$`, w.hasNothingDueOnPath)
	sc.Step(`^"([^"]+)" has nothing due, weak, new or coming due on their path for guitar$`, w.hasNothingComingDueOnPath)
	sc.Step(`^"([^"]+)" is ready to start the skill "([^"]+)"(?:, which builds on a skill they have met)?$`, w.isReadyToStart)
	sc.Step(`^"([^"]+)" is ready to start "([^"]+)", which requires a skill they have met$`, w.isReadyToStartBuildingOn)
	sc.Step(`^"([^"]+)" is ready to start the concept "([^"]+)", which requires nothing and a skill on their path applies$`, w.isReadyToStartAppliedConcept)
	sc.Step(`^"([^"]+)" is ready to start the skill "([^"]+)", which requires nothing and a skill on their path is part of$`, w.isReadyToStartPathParent)
	sc.Step(`^"([^"]+)" is ready to start the skill "([^"]+)", which requires nothing and has no link to their path$`, w.isReadyToStartUnlinked)
	sc.Step(`^"([^"]+)" is accurate but not fluent on "([^"]+)", and its review isn't due$`, w.isAccurateNotDueOn)
	sc.Step(`^"([^"]+)" is fluent on "([^"]+)", and its review isn't due$`, w.isFluentNotDueOn)
	sc.Step(`^"([^"]+)"'s path skill "([^"]+)" has exercises and the play-along "([^"]+)" on guitar$`, w.pathSkillHasExercisesAndPlayAlong)
	sc.Step(`^"([^"]+)"'s path skill "([^"]+)" has the play-along "([^"]+)" on guitar$`, w.pathSkillHasPlayAlong)
	sc.Step(`^"([^"]+)"'s path skill "([^"]+)" has the exercise "([^"]+)" for every instrument$`, w.pathSkillHasExercise)
	sc.Step(`^"([^"]+)" has never answered "([^"]+)"$`, w.hasNeverAnswered)
	sc.Step(`^no skill of the session's focus items has a play-along on guitar$`, w.noFocusSkillHasPlayAlongOnGuitar)
	sc.Step(`^none of "([^"]+)"'s path skills has a play-along on "([^"]+)"$`, w.noPathSkillHasPlayAlongOn)
	sc.Step(`^"([^"]+)" is a skill on "([^"]+)"'s path$`, w.isSkillOnPath)
	sc.Step(`^"([^"]+)" has never practised "([^"]+)"$`, w.hasNeverPractised)

	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with "?([a-z-]+)"? in hand$`, w.composesSession)
	sc.Step(`^"([^"]+)" composes a caught-up (\d+)-minute session with "([^"]+)" in hand$`, w.composesCaughtUpSession)
	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with an instrument that doesn't exist in hand$`, w.composesSessionWithMissingInstrument)
	sc.Step(`^"([^"]+)" is picked as a (due|new) item$`, w.isPickedAs)

	sc.Step(`^the session starts with "([^"]+)" with the reason (\w+) at (\d+) BPM$`, w.sessionStartsWith)
	sc.Step(`^no item in the session has the reason (\w+)$`, w.noItemHasReason)
	sc.Step(`^every item in the session suits "([^"]+)" or every instrument$`, w.everyItemSuits)
	sc.Step(`^it starts at (\d+) BPM with a target of (\d+) BPM$`, w.pickedStartsAt)
	sc.Step(`^every item in the session has one of the reasons (.+)$`, w.everyItemHasOneOfReasons)
	sc.Step(`^about (\d+)% of the focus time goes to (\w+) items, (\d+)% to (\w+) items and (\d+)% to (\w+) items$`, w.focusTimeShares3)
	sc.Step(`^about (\d+)% of the focus time goes to (\w+) items and (\d+)% to (\w+) items$`, w.focusTimeShares2)
	sc.Step(`^the last item is a play-along with the reason (\w+)$`, w.lastItemIsPlayAlongWithReason)
	sc.Step(`^the last item is "([^"]+)" with the reason (\w+)$`, w.lastItemIs)
	sc.Step(`^it applies a skill that the session's focus items practise$`, w.endingAppliesFocusSkill)
	sc.Step(`^exactly one item has the reason (\w+)$`, w.exactlyOneItemHasReason)
	sc.Step(`^the focus time is the (\d+) minutes less the warm-up and that play-along's estimated time$`, w.focusTimeLessWarmUpAndEnding)
	sc.Step(`^the focus time is the (\d+) minutes less the warm-up$`, w.focusTimeLessWarmUp)
	sc.Step(`^the session includes "([^"]+)" with the reason (\w+)$`, w.sessionIncludesWithReason)
	sc.Step(`^"([^"]+)" is not in the session with the reason (\w+)$`, w.notInSessionWithReason)
	sc.Step(`^the session includes items of "([^"]+)"$`, w.sessionIncludesItemsOf)
	sc.Step(`^the session has items of "([^"]+)" with the reason (\w+)$`, w.sessionHasItemsOfWithReason)
	sc.Step(`^no item in the session is of "([^"]+)"$`, w.noItemIsOf)
	sc.Step(`^the focus time left after the due and new items is split about evenly between review_ahead and stretch items$`, w.leftoverSplitEvenly)
	sc.Step(`^no more than (\d+) minutes go to new items$`, w.noMoreMinutesToNew)
	sc.Step(`^about half the session is items with the reason (\w+)$`, w.halfTheSessionHasReason)
	sc.Step(`^about half the session is items of "([^"]+)" with the reason (\w+)$`, w.halfTheSessionIsOfWithReason)
	sc.Step(`^every item after the warm-up has the reason (\w+)$`, w.everyItemAfterWarmUpHasReason)
	sc.Step(`^the stretch items are of "([^"]+)" before "([^"]+)"$`, w.stretchItemsOfBefore)
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
	w.putPlayAlongOn(slug, instrument, practiceSkill, tempo)
}

// putPlayAlongOn is putPlayAlong classified under skill.
func (w *world) putPlayAlongOn(slug, instrument, skill string, tempo int) {
	d := domain.Diagram{
		ID:            diagramID(slug).String(),
		InstrumentID:  instrumentID(instrument).String(),
		InstrumentIDs: []string{instrumentID(instrument).String()},
		Names:         domain.LocalizedText{"en": slug},
		Kind:          domain.DiagramKindBasic,
		CreatedBy:     w.curatorID(),
		TimeSignature: domain.DefaultTimeSignature,
		TempoBPM:      &tempo,
		Skills:        []domain.KnowledgeNode{{ID: w.skillIDFor(skill).String()}},
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

// isPickedAs composes a session in which slug is a focus item: a due
// exercise on the path's skill makes the path's other play-along the
// application ending.
func (w *world) isPickedAs(slug, reason string) error {
	w.pickedSlug, w.pickedReason = slug, reason
	w.dueItems("alice", w.putPracticeExercise("due-exercise", practiceSkill, "guitar"))
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
		if item.Exercise != nil {
			if ids := item.Exercise.InstrumentIds; len(ids) > 0 && !slices.Contains(ids, instrumentID(instrument)) {
				return fmt.Errorf("expected every item to suit %q, but %s suits %v", instrument, item.ItemKey, ids)
			}
			continue
		}
		if item.PlayAlong == nil {
			return fmt.Errorf("expected a play-along or an exercise, got %s", item.ItemKey)
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

// practiceExerciseSeconds is the estimate of every exercise a practice
// scenario seeds, fine-grained enough for the session's shares to show.
const practiceExerciseSeconds = 30

// plentyOfItems is how many items "plenty of" seeds: more than any share
// of an hour-long session can take.
const plentyOfItems = 40

// putPracticeExercise seeds a 30-second exercise classified under skill,
// for instruments (none: every instrument), and returns its item key.
func (w *world) putPracticeExercise(slug, skill string, instruments ...string) string {
	seconds := practiceExerciseSeconds
	var ids []string
	for _, name := range instruments {
		ids = append(ids, instrumentID(name).String())
	}
	id := exerciseID(slug).String()
	w.exercises.put(domain.Exercise{
		ID: id, Title: slug, ExerciseType: domain.ExerciseTypeTextResponse,
		Skills:                   []domain.KnowledgeNode{{ID: w.skillIDFor(skill).String()}},
		EstimatedDurationSeconds: &seconds,
		InstrumentIDs:            ids,
		CreatedAt:                fixedNow,
	})
	return domain.ExerciseItemKey(id)
}

// putGuitarExercises seeds count guitar exercises under skill and returns
// their item keys.
func (w *world) putGuitarExercises(prefix, skill string, count int) []string {
	keys := make([]string, count)
	for i := range count {
		keys[i] = w.putPracticeExercise(fmt.Sprintf("%s-%02d", prefix, i), skill, "guitar")
	}
	return keys
}

// putItemStates gives name a state on each of keys, due in dueInDays (0 or
// less: due), at level after counted answers.
func (w *world) putItemStates(name string, level domain.KnowledgeLevel, counted, box, dueInDays int, keys ...string) {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	dueAt := fixedNow.AddDate(0, 0, dueInDays)
	if dueInDays <= 0 {
		dueAt = fixedNow.Add(-time.Hour)
	}
	for _, key := range keys {
		w.practiceStates.put(studentID, domain.PracticeItemState{
			ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: level,
			Counted: counted, Box: box, DueAt: &dueAt, LastAt: &fixedNow,
		})
	}
}

func (w *world) dueItems(name string, keys ...string) {
	w.putItemStates(name, domain.KnowledgeLevelLearning, 2, 1, 0, keys...)
}

func (w *world) weakItems(name string, keys ...string) {
	w.putItemStates(name, domain.KnowledgeLevelAccurate, 3, 2, 2, keys...)
}

func (w *world) knownItems(name string, dueInDays int, keys ...string) {
	w.putItemStates(name, domain.KnowledgeLevelFluent, 6, 4, dueInDays, keys...)
}

// cleanGuitarLick makes the path's guitar play-along known and played
// clean, due in dueInDays: the session's warm-up, never its focus.
func (w *world) cleanGuitarLick(name string, dueInDays int) {
	dueAt := fixedNow.AddDate(0, 0, dueInDays)
	clean := 100
	w.putPlayAlongState(name, "guitar-lick", domain.PracticeItemState{
		Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: &dueAt, LastAt: &fixedNow, BestCleanBPM: &clean,
	})
}

// addPathSkill gives name another standalone path, whose one lesson
// teaches skill.
func (w *world) addPathSkill(name, skill string) {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	node := domain.ContentNode{
		ID:             nodeID("practice-lesson-" + skill).String(),
		Title:          "practice-lesson-" + skill,
		ContentType:    domain.ContentTypeArticle,
		Classification: domain.Classification{Skills: []domain.KnowledgeNode{{ID: w.skillIDFor(skill).String()}}},
	}
	w.nodes.put(node)
	w.studentPaths.put(domain.StudentPath{
		ID:               deterministicUUID("student-path", name+"/"+skill).String(),
		StudentID:        studentID,
		SourceTemplateID: pathID("practice-path-" + skill).String(),
		AssignedAt:       fixedNow.Add(time.Minute),
		Items:            []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID}},
	})
}

// deletePlayAlongs removes the play-alongs seeded for instrument.
func (w *world) deletePlayAlongs(instrument string) {
	w.diagrams.mu.Lock()
	defer w.diagrams.mu.Unlock()
	for id, d := range w.diagrams.byID {
		if slices.Contains(d.InstrumentIDs, instrumentID(instrument).String()) {
			delete(w.diagrams.byID, id)
		}
	}
}

func (w *world) hasPlentyOfEverything(name string) error {
	return w.hasItemsOnGuitar(name, "plenty of", "plenty of", "plenty of")
}

func (w *world) hasItemsOnGuitar(name, due, weak, fresh string) error {
	if due == "plenty of" {
		w.dueItems(name, w.putGuitarExercises("due", practiceSkill, plentyOfItems)...)
	}
	if weak == "plenty of" {
		w.weakItems(name, w.putGuitarExercises("weak", practiceSkill, plentyOfItems)...)
	}
	if fresh == "plenty of" {
		w.putGuitarExercises("new", practiceSkill, plentyOfItems)
	}
	return nil
}

func (w *world) hasMinutesOfItems(name string, dueMinutes, newMinutes int) error {
	w.dueItems(name, w.putGuitarExercises("due", practiceSkill, dueMinutes*60/practiceExerciseSeconds)...)
	w.putGuitarExercises("new", practiceSkill, newMinutes*60/practiceExerciseSeconds)
	return nil
}

func (w *world) hasDueAndNewItems(name string, due, fresh int) error {
	w.dueItems(name, w.putGuitarExercises("due", practiceSkill, due)...)
	w.putGuitarExercises("new", practiceSkill, fresh)
	return nil
}

func (w *world) hasKnownItemsComingDue(name string) error {
	w.knownItems(name, 3, w.putGuitarExercises("known", practiceSkill, plentyOfItems)...)
	return nil
}

func (w *world) hasNothingDueOnPath(name string) error {
	w.cleanGuitarLick(name, 3)
	return nil
}

func (w *world) hasNothingComingDueOnPath(name string) error {
	w.cleanGuitarLick(name, 30)
	return nil
}

// isReadyToStart makes skill, with plenty of unseen items, build on a
// foundation skill the student is accurate on, which connects it to what
// they are learning.
func (w *world) isReadyToStart(name, skill string) error {
	return w.buildOnFoundation(name, skill, plentyOfItems)
}

// rankedStretchItems is how many unseen items each node of a stretch
// ranking scenario has: few enough for a short session to reach every node.
const rankedStretchItems = 4

// isReadyToStartBuildingOn makes skill require a foundation skill the
// student is accurate on.
func (w *world) isReadyToStartBuildingOn(name, skill string) error {
	return w.buildOnFoundation(name, skill, rankedStretchItems)
}

// buildOnFoundation gives skill count unseen guitar items and makes it
// require a foundation skill the student is accurate on.
func (w *world) buildOnFoundation(name, skill string, count int) error {
	foundation := skill + "-foundation"
	w.putItemStates(name, domain.KnowledgeLevelAccurate, 3, 2, 3, w.putGuitarExercises(foundation, foundation, rankedStretchItems)...)
	if err := w.skillRequiresSkill(skill, foundation, string(domain.KnowledgeLevelAccurate)); err != nil {
		return err
	}
	w.putGuitarExercises(skill, skill, count)
	return nil
}

// isReadyToStartAppliedConcept gives concept unseen guitar exercises and
// has the background's path skill apply it.
func (w *world) isReadyToStartAppliedConcept(_, concept string) error {
	conceptID := w.conceptIDFor(concept)
	for i := range rankedStretchItems {
		seconds := practiceExerciseSeconds
		w.exercises.put(domain.Exercise{
			ID: exerciseID(fmt.Sprintf("%s-%02d", concept, i)).String(), Title: concept, ExerciseType: domain.ExerciseTypeTextResponse,
			Concepts:                 []domain.KnowledgeNode{{ID: conceptID.String()}},
			EstimatedDurationSeconds: &seconds,
			InstrumentIDs:            []string{instrumentID("guitar").String()},
			CreatedAt:                fixedNow,
		})
	}
	return w.putEdge(w.skillIDFor(practiceSkill).String(), conceptID.String(), domain.KnowledgeEdgeTypeApplies, nil)
}

// isReadyToStartPathParent gives skill plenty of unseen guitar items and
// makes it the parent of the background's path skill.
func (w *world) isReadyToStartPathParent(_, skill string) error {
	parentID := w.skillIDFor(skill)
	w.putSkill(practiceSkill, &parentID)
	w.putGuitarExercises(skill, skill, plentyOfItems)
	return nil
}

// isReadyToStartUnlinked gives skill plenty of unseen guitar items and no
// link to anything.
func (w *world) isReadyToStartUnlinked(_, skill string) error {
	w.putGuitarExercises(skill, skill, plentyOfItems)
	return nil
}

func (w *world) isAccurateNotDueOn(name, slug string) error {
	w.putPlayAlong(slug, "guitar", 100)
	w.weakItems(name, domain.PlayAlongItemKey(diagramID(slug).String()))
	return nil
}

func (w *world) isFluentNotDueOn(name, slug string) error {
	w.putPlayAlong(slug, "guitar", 100)
	w.knownItems(name, 5, domain.PlayAlongItemKey(diagramID(slug).String()))
	return nil
}

func (w *world) pathSkillHasExercisesAndPlayAlong(name, skill, slug string) error {
	w.addPathSkill(name, skill)
	w.putGuitarExercises(skill, skill, 4)
	w.putPlayAlongOn(slug, "guitar", skill, 100)
	return nil
}

func (w *world) pathSkillHasPlayAlong(name, skill, slug string) error {
	w.addPathSkill(name, skill)
	w.putPlayAlongOn(slug, "guitar", skill, 100)
	return nil
}

func (w *world) pathSkillHasExercise(name, skill, slug string) error {
	w.addPathSkill(name, skill)
	w.putPracticeExercise(slug, skill)
	return nil
}

func (w *world) hasNeverAnswered(name, slug string) error {
	key := domain.ExerciseItemKey(exerciseID(slug).String())
	states, err := w.practiceStates.GetStates(w.ctx(), w.ensureRegistered(name, domain.RoleStudent).String(), []string{key})
	if err != nil {
		return err
	}
	if len(states) > 0 {
		return fmt.Errorf("expected %q never answered, got %+v", slug, states)
	}
	return nil
}

// noFocusSkillHasPlayAlongOnGuitar removes the path's guitar play-along and
// gives the student due guitar exercises on the path's skill instead.
func (w *world) noFocusSkillHasPlayAlongOnGuitar() error {
	w.deletePlayAlongs("guitar")
	w.dueItems("alice", w.putGuitarExercises("due", practiceSkill, 4)...)
	return nil
}

// noPathSkillHasPlayAlongOn removes the path's play-alongs for instrument
// and gives the student due exercises for every instrument instead.
func (w *world) noPathSkillHasPlayAlongOn(name, instrument string) error {
	w.deletePlayAlongs(instrument)
	for i := range 4 {
		w.dueItems(name, w.putPracticeExercise(fmt.Sprintf("due-%02d", i), practiceSkill))
	}
	return nil
}

// isSkillOnPath gives name a path teaching skill, with two guitar
// exercises on it.
func (w *world) isSkillOnPath(skill, name string) error {
	if _, err := w.ensureInstrumentSeeded("guitar"); err != nil {
		return err
	}
	w.authenticateAs(name, domain.RoleStudent)
	w.addPathSkill(name, skill)
	w.putGuitarExercises(skill, skill, 2)
	return nil
}

func (w *world) hasNeverPractised(string, string) error { return nil }

func (w *world) composesCaughtUpSession(name string, minutes int, instrument string) error {
	w.cleanGuitarLick(name, 30)
	return w.composesSession(name, minutes, instrument)
}

// sessionSeconds sums the plan's estimated seconds per reason.
func sessionSeconds(plan generated.PracticeSessionPlan) map[string]int {
	seconds := map[string]int{}
	for _, item := range plan.Items {
		seconds[string(item.Reason)] += item.EstimatedSeconds
	}
	return seconds
}

// focusSeconds is the plan's time less its warm-up and its application
// ending.
func focusSeconds(plan generated.PracticeSessionPlan) int {
	seconds := sessionSeconds(plan)
	return plan.Minutes*60 - seconds[string(generated.WarmUp)] - seconds[string(generated.Application)]
}

func (w *world) everyItemHasOneOfReasons(list string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	reasons := strings.FieldsFunc(strings.ReplaceAll(list, " or ", ","), func(r rune) bool { return r == ',' || r == ' ' })
	for _, item := range plan.Items {
		if !slices.Contains(reasons, string(item.Reason)) {
			return fmt.Errorf("%s has the reason %s, not one of %v", item.ItemKey, item.Reason, reasons)
		}
	}
	return nil
}

// sharePoints is how far, in percentage points, an "about" share may be
// from its target: one 30-second item in a short focus block.
const sharePoints = 5

func (w *world) focusTimeShares(want map[string]int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	focus := focusSeconds(plan)
	seconds := sessionSeconds(plan)
	for reason, percent := range want {
		got := seconds[reason] * 100 / focus
		if got < percent-sharePoints || got > percent+sharePoints {
			return fmt.Errorf("%s items take %d%% of the %d seconds of focus time, want about %d%%: %v", reason, got, focus, percent, seconds)
		}
	}
	return nil
}

func (w *world) focusTimeShares3(p1 int, r1 string, p2 int, r2 string, p3 int, r3 string) error {
	return w.focusTimeShares(map[string]int{r1: p1, r2: p2, r3: p3})
}

func (w *world) focusTimeShares2(p1 int, r1 string, p2 int, r2 string) error {
	return w.focusTimeShares(map[string]int{r1: p1, r2: p2})
}

func (w *world) lastItem() (generated.PracticeSessionItem, error) {
	plan, err := w.composedPlan()
	if err != nil {
		return generated.PracticeSessionItem{}, err
	}
	return plan.Items[len(plan.Items)-1], nil
}

func (w *world) lastItemIsPlayAlongWithReason(reason string) error {
	last, err := w.lastItem()
	if err != nil {
		return err
	}
	if last.PlayAlong == nil || string(last.Reason) != reason {
		return fmt.Errorf("expected the last item to be a play-along with the reason %s, got %s with %s", reason, last.ItemKey, last.Reason)
	}
	return nil
}

func (w *world) lastItemIs(slug, reason string) error {
	last, err := w.lastItem()
	if err != nil {
		return err
	}
	if last.PlayAlong == nil || last.PlayAlong.DiagramId != diagramID(slug) || string(last.Reason) != reason {
		return fmt.Errorf("expected the last item to be %q with the reason %s, got %s with %s", slug, reason, last.ItemKey, last.Reason)
	}
	return nil
}

func (w *world) endingAppliesFocusSkill() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	last := plan.Items[len(plan.Items)-1]
	d, err := w.diagrams.GetByID(w.ctx(), last.PlayAlong.DiagramId.String())
	if err != nil {
		return err
	}
	for _, item := range plan.Items[:len(plan.Items)-1] {
		if item.Reason != generated.WarmUp && item.NodeId != nil && slices.Contains(d.SkillIDs(), item.NodeId.String()) {
			return nil
		}
	}
	return fmt.Errorf("the ending %s applies %v, which no focus item practises", last.ItemKey, d.SkillIDs())
}

func (w *world) exactlyOneItemHasReason(reason string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	count := 0
	for _, item := range plan.Items {
		if string(item.Reason) == reason {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("expected exactly one item with the reason %s, got %d", reason, count)
	}
	return nil
}

func (w *world) focusFits(minutes int, withEnding bool) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	seconds := sessionSeconds(plan)
	ending := seconds[string(generated.Application)]
	if withEnding != (ending > 0) {
		return fmt.Errorf("expected an application ending: %t, got %d seconds of one", withEnding, ending)
	}
	if focus := minutes*60 - seconds[string(generated.WarmUp)] - ending; focus != focusSeconds(plan) {
		return fmt.Errorf("expected %d seconds of focus time, got %d", focus, focusSeconds(plan))
	}
	used := 0
	for reason, s := range seconds {
		if reason != string(generated.WarmUp) && reason != string(generated.Application) {
			used += s
		}
	}
	if used > focusSeconds(plan) {
		return fmt.Errorf("the focus items take %d seconds, more than the %d of focus time", used, focusSeconds(plan))
	}
	return nil
}

func (w *world) focusTimeLessWarmUpAndEnding(minutes int) error { return w.focusFits(minutes, true) }

func (w *world) focusTimeLessWarmUp(minutes int) error { return w.focusFits(minutes, false) }

// itemKeyFor is the item key of the play-along or exercise called slug.
func (w *world) itemKeyFor(slug string) string {
	if _, err := w.diagrams.GetByID(w.ctx(), diagramID(slug).String()); err == nil {
		return domain.PlayAlongItemKey(diagramID(slug).String())
	}
	return domain.ExerciseItemKey(exerciseID(slug).String())
}

func (w *world) sessionIncludesWithReason(slug, reason string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	key := w.itemKeyFor(slug)
	for _, item := range plan.Items {
		if item.ItemKey == key && string(item.Reason) == reason {
			return nil
		}
	}
	return fmt.Errorf("expected %q with the reason %s, got %+v", slug, reason, sessionSeconds(plan))
}

func (w *world) notInSessionWithReason(slug, reason string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	key := w.itemKeyFor(slug)
	for _, item := range plan.Items {
		if item.ItemKey == key && string(item.Reason) == reason {
			return fmt.Errorf("expected %q not to be picked as %s", slug, reason)
		}
	}
	return nil
}

// nodeIDByName resolves a skill or concept a scenario named.
func (w *world) nodeIDByName(name string) uuid.UUID {
	if id, ok := w.conceptIDByName[name]; ok {
		return id
	}
	return w.skillIDFor(name)
}

func (w *world) sessionHasItemsOfWithReason(node, reason string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if item.NodeId != nil && *item.NodeId == w.nodeIDByName(node) && string(item.Reason) == reason {
			return nil
		}
	}
	return fmt.Errorf("expected items of %q with the reason %s in the session", node, reason)
}

func (w *world) noItemIsOf(node string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if item.NodeId != nil && *item.NodeId == w.nodeIDByName(node) {
			return fmt.Errorf("expected no item of %q, found %s", node, item.ItemKey)
		}
	}
	return nil
}

func (w *world) sessionIncludesItemsOf(skill string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if item.NodeId != nil && *item.NodeId == w.skillIDFor(skill) {
			return nil
		}
	}
	return fmt.Errorf("expected items of %q in the session", skill)
}

func (w *world) leftoverSplitEvenly() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	seconds := sessionSeconds(plan)
	review, stretch := seconds[string(generated.ReviewAhead)], seconds[string(generated.Stretch)]
	if review == 0 || stretch == 0 || abs(review-stretch)*10 > review+stretch {
		return fmt.Errorf("expected the time left split about evenly, got %d seconds of review_ahead and %d of stretch", review, stretch)
	}
	return nil
}

func abs(n int) int { return max(n, -n) }

func (w *world) noMoreMinutesToNew(minutes int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	if got := sessionSeconds(plan)[string(generated.New)]; got > minutes*60 {
		return fmt.Errorf("expected no more than %d minutes of new items, got %d seconds", minutes, got)
	}
	return nil
}

// halfTheSession checks that the items matching keep take about half the
// session after its warm-up.
func (w *world) halfTheSession(keep func(generated.PracticeSessionItem) bool, what string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	total, got := 0, 0
	for _, item := range plan.Items {
		if item.Reason == generated.WarmUp {
			continue
		}
		total += item.EstimatedSeconds
		if keep(item) {
			got += item.EstimatedSeconds
		}
	}
	if total == 0 || got*100/total < 50-2*sharePoints || got*100/total > 50+2*sharePoints {
		return fmt.Errorf("expected about half the session to be %s, got %d of %d seconds", what, got, total)
	}
	return nil
}

func (w *world) halfTheSessionHasReason(reason string) error {
	return w.halfTheSession(func(item generated.PracticeSessionItem) bool { return string(item.Reason) == reason }, reason)
}

func (w *world) halfTheSessionIsOfWithReason(skill, reason string) error {
	id := w.skillIDFor(skill)
	return w.halfTheSession(func(item generated.PracticeSessionItem) bool {
		return string(item.Reason) == reason && item.NodeId != nil && *item.NodeId == id
	}, skill+" "+reason)
}

func (w *world) everyItemAfterWarmUpHasReason(reason string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if item.Reason != generated.WarmUp && string(item.Reason) != reason {
			return fmt.Errorf("expected every item after the warm-up to be %s, got %s with %s", reason, item.ItemKey, item.Reason)
		}
	}
	return nil
}

func (w *world) stretchItemsOfBefore(first, second string) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	firstAt, secondAt := -1, -1
	for i, item := range plan.Items {
		if item.Reason != generated.Stretch || item.NodeId == nil {
			continue
		}
		if *item.NodeId == w.nodeIDByName(first) && firstAt < 0 {
			firstAt = i
		}
		if *item.NodeId == w.nodeIDByName(second) && secondAt < 0 {
			secondAt = i
		}
	}
	if firstAt < 0 || secondAt < 0 || firstAt > secondAt {
		return fmt.Errorf("expected stretch items of %q before %q, got them at %d and %d", first, second, firstAt, secondAt)
	}
	return nil
}
