//go:build integration

package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// cellWorld is what the fretboard cell steps share in a scenario.
type cellWorld struct {
	// key is the cell the latest step named: alice's practised cell, or the one
	// she was just asked.
	key string
	// lastAnswer is the latest answer sent, for a step that delivers it again.
	lastAnswer domain.PracticeAnswer
	// evidenceBefore counts the cell's evidence before the latest answer.
	evidenceBefore int
	// catalog holds the drill catalog's threshold versions, by template.
	catalog map[string][]catalogThreshold
}

type catalogThreshold struct {
	Template    string `yaml:"template"`
	Version     int    `yaml:"version"`
	FluentNetMs int    `yaml:"fluent_net_ms"`
	Source      string `yaml:"source"`
}

// standardGuitar is the tuning of the guitar every cell scenario plays, lowest
// string first.
var standardGuitar = []string{"E2", "A2", "D3", "G3", "B3", "E4"}

// cellLatency is a cell answer's time when a scenario doesn't state one: well
// within any fluent time a scenario sets, once the tap time is taken off.
const cellLatency = 1500

func registerCellSteps(sc *godog.ScenarioContext, w *world) {
	// ── Grading a cell ─────────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" answers the guitar cell on string (\d+) at fret (\d+) by naming the note "([^"]*)" after (\d+) milliseconds$`, w.answersTheCellByNaming)
	sc.Step(`^"([^"]*)" is asked for the guitar cell on string (\d+) at fret (\d+) and taps string (\d+) at fret (\d+)$`, w.isAskedForTheCellAndTaps)
	sc.Step(`^"([^"]*)" answers the guitar cell on string (\d+) at fret (\d+) by selecting option "([^"]*)" of exercise "([^"]*)"$`, w.answersTheCellBySelectingAnOption)
	sc.Step(`^"([^"]*)" completed a tap check with a median tap of (\d+) milliseconds$`, w.hasATapTime)
	sc.Step(`^"([^"]*)"'s tap time is (\d+) milliseconds$`, w.hasATapTime)
	sc.Step(`^"([^"]*)" has auto-graded evidence for that cell that is (correct|wrong)(?: with a latency of (\d+) milliseconds)?$`, w.hasEvidenceForThatCell)
	sc.Step(`^"([^"]*)" has no evidence for that cell$`, w.hasNoEvidenceForThatCell)
	sc.Step(`^the evidence keeps the response exactly as "([^"]*)" sent it$`, w.evidenceKeepsTheResponse)
	sc.Step(`^the evidence names the grader "([^"]*)"$`, w.evidenceNamesTheGrader)
	sc.Step(`^the evidence is identified by the identifier of the practice\.item_answered event$`, w.evidenceIsIdentifiedByTheEvent)
	sc.Step(`^the evidence for that cell records a tap time of (\d+) milliseconds$`, w.evidenceRecordsATapTime)
	sc.Step(`^the evidence's answer key is string (\d+), fret (\d+), note "([^"]*)"$`, w.answerKeyIsTheCell)
	sc.Step(`^the exercise's options are later edited to ((?:"[^"]*"(?:, | and )?)+), with "([^"]*)" correct as well$`, w.exerciseOptionsAreEdited)
	sc.Step(`^the evidence's answer key shows the options ((?:"[^"]*"(?:, | and )?)+), with ((?:"[^"]*"(?:, | and )?)+) correct$`, w.answerKeyShowsTheOptions)
	sc.Step(`^the answer is rejected because the response does not fit the item$`, w.answerRejectedBecause(domain.GradeRejectionResponseDoesNotFitItem))
	sc.Step(`^the answer is rejected because the cell is not on the instrument$`, w.answerRejectedBecause(domain.GradeRejectionInvalidCell))

	// ── Knowledge of a cell ────────────────────────────────────────────────────
	sc.Step(`^"([^"]*)" answers the cell correctly$`, w.answersTheCell(true))
	sc.Step(`^"([^"]*)" answers the cell wrongly$`, w.answersTheCell(false))
	sc.Step(`^"([^"]*)" answers the cell correctly (\d+) times on (\d+) different days$`, w.answersTheCellOnDifferentDays)
	sc.Step(`^"([^"]*)" answers the cell correctly (\d+) times, each in (\d+) milliseconds$`, w.answersTheCellRepeatedly)
	sc.Step(`^"([^"]*)" answers the cell correctly when it falls due$`, w.answersTheCellWhenDue)
	sc.Step(`^"([^"]*)" is fluent on the cell and the cell is in box (\d+)$`, w.isFluentOnTheCellInBox)
	sc.Step(`^"([^"]*)" is accurate on the cell$`, w.isAccurateOnTheCell)
	sc.Step(`^the cell is in box (\d+)$`, w.theCellIsInBox)
	sc.Step(`^the cell is in box (\d+) and due today$`, w.theCellIsInBoxDueIn(0))
	sc.Step(`^the cell is in box (\d+) and due in 1 day$`, w.theCellIsInBoxDueIn(1))
	sc.Step(`^the cell moves to box (\d+)$`, w.theCellMovesToBox)
	sc.Step(`^the cell moves to box (\d+) and is next due in (\d+) days?$`, w.theCellMovesToBoxDueIn)
	sc.Step(`^the cell stays in box (\d+)$`, w.theCellStaysInBox)
	sc.Step(`^"([^"]*)"'s level for the cell is (?:still )?"([^"]*)"$`, w.levelForTheCellIs)
	sc.Step(`^the same practice\.item_answered event for the cell arrives twice$`, w.theSameCellAnswerArrivesTwice)
	sc.Step(`^"([^"]*)" has one piece of evidence for the cell$`, w.hasOnePieceOfEvidenceForTheCell)
	sc.Step(`^"([^"]*)" answered the cell correctly on Monday and Wednesday$`, w.answeredOnMondayAndWednesday)
	sc.Step(`^"([^"]*)"'s wrong answer from Tuesday arrives after Wednesday's$`, w.wrongAnswerFromTuesdayArrives)
	sc.Step(`^"([^"]*)"'s state for the cell is the same as if the three answers had arrived in order$`, w.stateIsAsIfInOrder)
	sc.Step(`^"([^"]*)" answered the cell correctly in (\d+) milliseconds when the fluent time was (\d+) milliseconds$`, w.answeredWhenTheFluentTimeWas)
	sc.Step(`^the fluent time for naming a note changes to (\d+) milliseconds$`, w.namingFluentTimeChanges)
	sc.Step(`^that answer still counts as within the fluent time$`, w.thatAnswerStillCountsAsWithin)
	sc.Step(`^"([^"]*)" sends an answer to the cell that the grader rejects$`, w.sendsARejectedAnswer)
	sc.Step(`^"([^"]*)" has no new evidence for the cell$`, w.hasNoNewEvidenceForTheCell)
	sc.Step(`^"([^"]*)" says naming notes felt "([^"]*)" at the end of a session$`, w.saysNamingFelt)

	// ── Timed thresholds on a cell ─────────────────────────────────────────────
	sc.Step(`^"([^"]*)" names the note of a cell correctly in (\d+) milliseconds$`, w.namesACellCorrectly)
	sc.Step(`^"([^"]*)" names the note of a cell correctly on (\d{4}-\d{2}-\d{2}) in (\d+) milliseconds$`, w.namesACellCorrectlyOn)
	sc.Step(`^"([^"]*)" finds a note correctly in (\d+) milliseconds$`, w.findsANoteCorrectly)
	sc.Step(`^version 2 of "([^"]*)" has a fluent time of (\d+) milliseconds from (\d{4}-\d{2}-\d{2})$`, w.templateHasVersion2From)
	sc.Step(`^"([^"]*)" became fluent on a cell with answers judged against version 1$`, w.becameFluentOnACell)
	sc.Step(`^version 2 lowers the fluent time to (\d+) milliseconds$`, w.versionTwoLowersTheFluentTime)
	sc.Step(`^"([^"]*)" is still fluent on that cell$`, w.isStillFluentOnThatCell)
	sc.Step(`^student "([^"]*)" has never done a tap check$`, w.hasNeverDoneATapCheck)
	sc.Step(`^the drill template "([^"]*)" has no version yet$`, w.templateHasNoVersion)
	sc.Step(`^the answer counts toward "([^"]*)"'s accuracy$`, w.answerCountsTowardAccuracy)
	sc.Step(`^it never counts toward fluency, even after version 1 is installed$`, w.neverCountsTowardFluency)
	sc.Step(`^the practice drill catalog is installed$`, w.practiceDrillCatalogIsInstalled)
	sc.Step(`^"([^"]*)" has version 1 from source "([^"]*)" with a fluent time of (\d+) milliseconds$`, w.catalogHasVersion1)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// guitarCell names a cell of the scenario's guitar, which exists in the
// reference snapshot from the moment a scenario mentions it.
func (w *world) guitarCell(str, fret int) string {
	id := stableUUID("instrument", "guitar")
	if _, ok := w.reference.instruments[id]; !ok {
		w.reference.instruments[id] = domain.InstrumentReference{ID: id, Tuning: standardGuitar}
	}
	w.cells.key = fmt.Sprintf("fretboard_cell:%s:%d:%d", id, str, fret)
	return w.cells.key
}

// currentCell is the cell the scenario practises: the one a step named, or
// alice's guitar cell on string 5 at fret 3.
func (w *world) currentCell() string {
	if w.cells.key == "" {
		return w.guitarCell(5, 3)
	}
	return w.cells.key
}

var sharpNames = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// noteOf is the note of a guitar cell, spelled with sharps; offset moves it by
// semitones, so offset 1 names a wrong note.
func noteOf(itemKey string, offset int) (string, error) {
	var layout string
	var str, fret int
	if _, err := fmt.Sscanf(strings.ReplaceAll(strings.TrimPrefix(itemKey, "fretboard_cell:"), ":", " "), "%s %d %d", &layout, &str, &fret); err != nil {
		return "", err
	}
	open := standardGuitar[len(standardGuitar)-str]
	pc := slices.Index(sharpNames, strings.TrimRight(open, "0123456789"))
	return sharpNames[(pc+fret+offset)%12], nil
}

func (w *world) sendCell(student, itemKey string, response domain.PracticeResponse) error {
	before, err := w.evidenceFor(student, itemKey)
	if err != nil {
		return err
	}
	w.cells.evidenceBefore = len(before)
	if err := w.answer(student, itemKey, response); err != nil {
		return err
	}
	w.cells.lastAnswer = w.lastAnswer
	return nil
}

// nameTheCell names the note of the cell, right or wrong, in latency ms.
func (w *world) nameTheCell(student, itemKey string, right bool, latency int) error {
	offset := 0
	if !right {
		offset = 1
	}
	note, err := noteOf(itemKey, offset)
	if err != nil {
		return err
	}
	return w.sendCell(student, itemKey, domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: note, LatencyMs: &latency})
}

func (w *world) cellFold(student string) (domain.ItemFold, error) {
	return w.fold(student, w.currentCell())
}

// onlyCellEvidence is the one piece of auto-graded evidence for the cell.
func (w *world) onlyCellEvidence(student string) (domain.PracticeEvidence, error) {
	evidence, err := w.evidenceFor(student, w.currentCell())
	if err != nil {
		return domain.PracticeEvidence{}, err
	}
	if len(evidence) != 1 {
		return domain.PracticeEvidence{}, fmt.Errorf("want one piece of evidence for the cell, got %d", len(evidence))
	}
	return evidence[0], nil
}

// cellGoal is what a cell's fluency is measured against, as the evidence
// processor builds it.
func (w *world) cellGoal() domain.ItemGoal {
	return domain.ItemGoal{FluentTimesByResponse: map[domain.PracticeResponseType][]domain.FluentTime{
		domain.PracticeResponseNameTheNote: w.reference.fluentTimes["fretboard_cell:name_the_note"],
		domain.PracticeResponseFindTheNote: w.reference.fluentTimes["fretboard_cell:find_the_note"],
	}}
}

// redeliver delivers the latest answer again: it stores no evidence, but
// rebuilds the item from its evidence under today's reference data.
func (w *world) redeliver() error {
	return w.service.Process(context.Background(), w.cells.lastAnswer)
}

// ── Grading a cell ───────────────────────────────────────────────────────────

func (w *world) answersTheCellByNaming(student string, str, fret int, note string, latency int) error {
	return w.sendCell(student, w.guitarCell(str, fret), domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: note, LatencyMs: &latency})
}

func (w *world) isAskedForTheCellAndTaps(student string, str, fret, tappedString, tappedFret int) error {
	latency := cellLatency
	return w.sendCell(student, w.guitarCell(str, fret), domain.PracticeResponse{
		Type: domain.PracticeResponseFindTheNote, String: &tappedString, Fret: &tappedFret, LatencyMs: &latency,
	})
}

func (w *world) answersTheCellBySelectingAnOption(student string, str, fret int, label, exercise string) error {
	latency := cellLatency
	return w.sendCell(student, w.guitarCell(str, fret), domain.PracticeResponse{
		Type: domain.PracticeResponseOptionChoice, OptionIDs: []string{optionID(exercise, label)}, LatencyMs: &latency,
	})
}

func (w *world) hasEvidenceForThatCell(student, verdict, latency string) error {
	e, err := w.onlyCellEvidence(student)
	if err != nil {
		return err
	}
	if e.Source != domain.EvidenceSourceAutoGraded || e.Correct == nil {
		return fmt.Errorf("evidence is %s with no verdict, want auto-graded", e.Source)
	}
	if *e.Correct != (verdict == "correct") {
		return fmt.Errorf("the answer is correct = %v, want %s", *e.Correct, verdict)
	}
	if latency != "" && (e.LatencyMs == nil || fmt.Sprint(*e.LatencyMs) != latency) {
		return fmt.Errorf("latency is %v, want %s ms", e.LatencyMs, latency)
	}
	return nil
}

func (w *world) hasNoEvidenceForThatCell(student string) error {
	evidence, err := w.evidenceFor(student, w.currentCell())
	if err != nil {
		return err
	}
	if len(evidence) != 0 {
		return fmt.Errorf("want no evidence for the cell, got %d", len(evidence))
	}
	return nil
}

func (w *world) evidenceKeepsTheResponse(student string) error {
	e, err := w.onlyCellEvidence(student)
	if err != nil {
		return err
	}
	sent, kept := w.cells.lastAnswer.Response, e.Response
	if sent.Type != kept.Type || sent.NoteName != kept.NoteName || *sent.LatencyMs != *kept.LatencyMs {
		return fmt.Errorf("evidence keeps %+v, want the response as sent, %+v", kept, sent)
	}
	return nil
}

// evidenceNamesTheGrader checks the evidence of the latest answer's item, a
// fretboard cell or a diagram shape.
func (w *world) evidenceNamesTheGrader(grader string) error {
	e, err := w.onlyEvidenceFor(w.lastStudent, w.lastItemKey)
	if err != nil {
		return err
	}
	if e.GraderID != grader {
		return fmt.Errorf("evidence names the grader %q, want %q", e.GraderID, grader)
	}
	return nil
}

func (w *world) evidenceIsIdentifiedByTheEvent() error {
	e, err := w.onlyCellEvidence(w.lastStudent)
	if err != nil {
		return err
	}
	if e.EvidenceID != w.cells.lastAnswer.EventID {
		return fmt.Errorf("evidence id is %q, want the event's id %q", e.EvidenceID, w.cells.lastAnswer.EventID)
	}
	return nil
}

func (w *world) evidenceRecordsATapTime(tapMs int) error {
	e, err := w.onlyCellEvidence(w.lastStudent)
	if err != nil {
		return err
	}
	if e.TapMs == nil || *e.TapMs != tapMs {
		return fmt.Errorf("evidence records a tap time of %v, want %d ms", e.TapMs, tapMs)
	}
	return nil
}

func (w *world) answerKeyIsTheCell(str, fret int, note string) error {
	e, err := w.onlyCellEvidence(w.lastStudent)
	if err != nil {
		return err
	}
	k := e.AnswerKey
	if k == nil || k.String == nil || k.Fret == nil || *k.String != str || *k.Fret != fret || k.NoteName != note {
		return fmt.Errorf("answer key is %+v, want string %d, fret %d, note %s", k, str, fret, note)
	}
	return nil
}

// exerciseOptionsAreEdited replaces the exercise's options in the reference
// snapshot, as an edit in core would.
func (w *world) exerciseOptionsAreEdited(all, alsoCorrect string) error {
	id := w.lastExerciseID
	old := w.reference.exercises[id]
	edited := domain.ExerciseReference{ID: id, ExerciseType: old.ExerciseType}
	var correct []string
	for _, o := range old.Options {
		if o.IsCorrect {
			correct = append(correct, shownLabel(o))
		}
	}
	correct = append(correct, alsoCorrect)
	edited.OptionIDs, edited.CorrectOptionIDs, edited.Options = exerciseOptions(w.lastExercise, labels(all), correct)
	w.reference.exercises[id] = edited
	return nil
}

func (w *world) answerKeyShowsTheOptions(all, correct string) error {
	evidence, err := w.evidenceFor(w.lastStudent, "exercise:"+w.lastExerciseID)
	if err != nil {
		return err
	}
	if len(evidence) != 1 || evidence[0].AnswerKey == nil {
		return fmt.Errorf("want one piece of evidence with an answer key, got %+v", evidence)
	}
	var shown, right []string
	for _, o := range evidence[0].AnswerKey.Options {
		shown = append(shown, shownLabel(o))
		if o.IsCorrect {
			right = append(right, shownLabel(o))
		}
	}
	if !slices.Equal(shown, labels(all)) || !slices.Equal(right, labels(correct)) {
		return fmt.Errorf("answer key shows %v with %v correct, want %v with %v correct", shown, right, labels(all), labels(correct))
	}
	return nil
}

// shownLabel reads an option's label from what the student was shown.
func shownLabel(o domain.AnswerOption) string {
	var shown struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(o.Shown, &shown); err != nil {
		return ""
	}
	return shown.Label
}

// ── Knowledge of a cell ──────────────────────────────────────────────────────

func (w *world) answersTheCell(right bool) func(string) error {
	return func(student string) error {
		return w.nameTheCell(student, w.currentCell(), right, cellLatency)
	}
}

func (w *world) answersTheCellOnDifferentDays(student string, times, days int) error {
	if times != days {
		return fmt.Errorf("one answer a day: %d answers on %d days", times, days)
	}
	for range times {
		if err := w.nameTheCell(student, w.currentCell(), true, cellLatency); err != nil {
			return err
		}
		w.clock = w.clock.AddDate(0, 0, 1)
	}
	return nil
}

// answerOnDueDays answers the cell right times times, each on the day it falls due.
func (w *world) answerOnDueDays(student string, times, latency int) error {
	for range times {
		if err := w.nameTheCell(student, w.currentCell(), true, latency); err != nil {
			return err
		}
		fold, err := w.cellFold(student)
		if err != nil {
			return err
		}
		w.clock = *fold.DueAt
	}
	return nil
}

// answersTheCellRepeatedly answers the cell right times times in a row: only the
// first moves its box, since the others come before it falls due.
func (w *world) answersTheCellRepeatedly(student string, times, latency int) error {
	for range times {
		if err := w.nameTheCell(student, w.currentCell(), true, latency); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) answersTheCellWhenDue(student string) error {
	fold, err := w.cellFold(student)
	if err != nil {
		return err
	}
	if fold.DueAt == nil {
		return fmt.Errorf("the cell has never been practised")
	}
	w.clock = *fold.DueAt
	return w.nameTheCell(student, w.currentCell(), true, cellLatency)
}

// reachBox answers the cell right on its due days until it is in box.
func (w *world) reachBox(student string, box int) error {
	for range box + 1 {
		fold, err := w.cellFold(student)
		if err != nil {
			return err
		}
		if fold.Box == box {
			return nil
		}
		if err := w.answerOnDueDays(student, 1, cellLatency); err != nil {
			return err
		}
	}
	return fmt.Errorf("the cell didn't reach box %d", box)
}

func (w *world) isFluentOnTheCellInBox(student string, box int) error {
	// Five quick right answers on the first day make the cell fluent in box 1;
	// answers before it falls due don't move it.
	for range 5 {
		if err := w.nameTheCell(student, w.currentCell(), true, cellLatency); err != nil {
			return err
		}
	}
	fold, err := w.cellFold(student)
	if err != nil {
		return err
	}
	w.clock = *fold.DueAt
	if err := w.reachBox(student, box); err != nil {
		return err
	}
	return w.levelForTheCellIs(student, string(domain.KnowledgeLevelFluent))
}

func (w *world) isAccurateOnTheCell(student string) error {
	if err := w.answersTheCellOnDifferentDays(student, 3, 3); err != nil {
		return err
	}
	return w.levelForTheCellIs(student, string(domain.KnowledgeLevelAccurate))
}

func (w *world) theCellIsInBox(box int) error {
	return w.theCellIsInBoxDueIn(0)(box)
}

// theCellIsInBoxDueIn brings alice's cell to box and sets today daysBefore its
// review.
func (w *world) theCellIsInBoxDueIn(daysBefore int) func(int) error {
	return func(box int) error {
		student := "alice"
		if err := w.nameTheCell(student, w.currentCell(), true, cellLatency); err != nil {
			return err
		}
		fold, err := w.cellFold(student)
		if err != nil {
			return err
		}
		w.clock = *fold.DueAt
		if err := w.reachBox(student, box); err != nil {
			return err
		}
		if fold, err = w.cellFold(student); err != nil {
			return err
		}
		w.clock = fold.DueAt.AddDate(0, 0, -daysBefore)
		return nil
	}
}

func (w *world) theCellMovesToBox(box int) error {
	fold, err := w.cellFold(w.lastStudent)
	if err != nil {
		return err
	}
	if fold.Box != box {
		return fmt.Errorf("the cell is in box %d, want %d", fold.Box, box)
	}
	return nil
}

func (w *world) theCellMovesToBoxDueIn(box, days int) error {
	if err := w.theCellMovesToBox(box); err != nil {
		return err
	}
	fold, err := w.cellFold(w.lastStudent)
	if err != nil {
		return err
	}
	want := w.cells.lastAnswer.OccurredAt.AddDate(0, 0, days)
	if fold.DueAt == nil || !fold.DueAt.Equal(want) {
		return fmt.Errorf("the cell is next due %v, want %v", fold.DueAt, want)
	}
	return nil
}

func (w *world) theCellStaysInBox(box int) error {
	return w.itemStaysInBox("the cell", box)
}

func (w *world) levelForTheCellIs(student, level string) error {
	fold, err := w.cellFold(student)
	if err != nil {
		return err
	}
	if got := fold.Level(); string(got) != level {
		return fmt.Errorf("the level for the cell is %q, want %q", got, level)
	}
	return nil
}

func (w *world) theSameCellAnswerArrivesTwice() error {
	if err := w.nameTheCell("alice", w.currentCell(), true, cellLatency); err != nil {
		return err
	}
	return w.redeliver()
}

func (w *world) hasOnePieceOfEvidenceForTheCell(student string) error {
	_, err := w.onlyCellEvidence(student)
	return err
}

// monday is the week the late-answer scenario happens in.
var monday = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

func (w *world) answeredOnMondayAndWednesday(student string) error {
	for _, day := range []int{0, 2} {
		w.clock = monday.AddDate(0, 0, day)
		if err := w.nameTheCell(student, w.currentCell(), true, cellLatency); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) wrongAnswerFromTuesdayArrives(student string) error {
	w.clock = monday.AddDate(0, 0, 1)
	return w.nameTheCell(student, w.currentCell(), false, cellLatency)
}

func (w *world) stateIsAsIfInOrder(student string) error {
	evidence, err := w.evidenceFor(student, w.currentCell())
	if err != nil {
		return err
	}
	slices.SortFunc(evidence, func(a, b domain.PracticeEvidence) int { return a.OccurredAt.Compare(b.OccurredAt) })
	var want domain.ItemFold
	for _, e := range evidence {
		if want, err = domain.FoldEvidence(want, e, w.cellGoal()); err != nil {
			return err
		}
	}
	got, err := w.cellFold(student)
	if err != nil {
		return err
	}
	if got.Box != want.Box || got.Counted != want.Counted || got.Accuracy != want.Accuracy || !got.DueAt.Equal(*want.DueAt) {
		return fmt.Errorf("the cell's state is %+v, want %+v", got, want)
	}
	return nil
}

func (w *world) answeredWhenTheFluentTimeWas(student string, latency, fluentMs int) error {
	if err := w.templateHasVersion1("fretboard_cell:name_the_note", fluentMs); err != nil {
		return err
	}
	return w.nameTheCell(student, w.currentCell(), true, latency)
}

func (w *world) namingFluentTimeChanges(fluentMs int) error {
	template := "fretboard_cell:name_the_note"
	versions := w.reference.fluentTimes[template]
	w.reference.fluentTimes[template] = append(versions, domain.FluentTime{Version: len(versions) + 1, EffectiveFrom: w.clock, FluentNetMs: fluentMs})
	// A new version rebuilds nothing by itself; a redelivery rebuilds the item, so
	// the earlier answer is judged again under every version there now is.
	return w.redeliver()
}

func (w *world) thatAnswerStillCountsAsWithin() error {
	e, err := w.onlyCellEvidence(w.lastStudent)
	if err != nil {
		return err
	}
	judged, ok := domain.JudgeTimed(e, w.cellGoal())
	if !ok || !judged.WithinFluentTime {
		return fmt.Errorf("the answer is judged %+v, want within the fluent time in force when it was given", judged)
	}
	fold, err := w.cellFold(w.lastStudent)
	if err != nil {
		return err
	}
	if fold.Fluency < 1-1e-9 {
		return fmt.Errorf("the rebuilt item's fluency is %.3f, want the answer still fully fluent", fold.Fluency)
	}
	return nil
}

func (w *world) sendsARejectedAnswer(student string) error {
	latency := cellLatency
	return w.sendCell(student, w.currentCell(), domain.PracticeResponse{
		Type: domain.PracticeResponseOptionChoice, OptionIDs: []string{"00000000-0000-4000-8000-0000000000b1"}, LatencyMs: &latency,
	})
}

func (w *world) hasNoNewEvidenceForTheCell(student string) error {
	evidence, err := w.evidenceFor(student, w.currentCell())
	if err != nil {
		return err
	}
	if len(evidence) != w.cells.evidenceBefore {
		return fmt.Errorf("the cell has %d pieces of evidence, want the %d it had", len(evidence), w.cells.evidenceBefore)
	}
	return nil
}

// saysNamingFelt ends alice's session with a felt rating. A session's end is
// folded into no item, so mastery can't move.
func (w *world) saysNamingFelt(student, _ string) error {
	end, err := w.endOf(student, w.clock.Format("15:04"), false, 1)
	if err != nil {
		return err
	}
	return w.deliverEnd(end)
}

// ── Timed thresholds on a cell ───────────────────────────────────────────────

func (w *world) namesACellCorrectly(student string, latency int) error {
	return w.nameTheCell(student, w.currentCell(), true, latency)
}

func (w *world) namesACellCorrectlyOn(student, date string, latency int) error {
	day, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return err
	}
	w.clock = day.Add(9 * time.Hour)
	return w.nameTheCell(student, w.currentCell(), true, latency)
}

func (w *world) findsANoteCorrectly(student string, latency int) error {
	var layout string
	var str, fret int
	key := w.currentCell()
	if _, err := fmt.Sscanf(strings.ReplaceAll(strings.TrimPrefix(key, "fretboard_cell:"), ":", " "), "%s %d %d", &layout, &str, &fret); err != nil {
		return err
	}
	return w.sendCell(student, key, domain.PracticeResponse{Type: domain.PracticeResponseFindTheNote, String: &str, Fret: &fret, LatencyMs: &latency})
}

func (w *world) templateHasVersion2From(template string, fluentMs int, from string) error {
	at, err := time.Parse(time.DateOnly, from)
	if err != nil {
		return err
	}
	w.reference.fluentTimes[template] = append(w.reference.fluentTimes[template], domain.FluentTime{Version: 2, EffectiveFrom: at, FluentNetMs: fluentMs})
	return nil
}

func (w *world) becameFluentOnACell(student string) error {
	for range 5 {
		if err := w.nameTheCell(student, w.currentCell(), true, cellLatency); err != nil {
			return err
		}
	}
	return w.levelForTheCellIs(student, string(domain.KnowledgeLevelFluent))
}

func (w *world) versionTwoLowersTheFluentTime(fluentMs int) error {
	return w.namingFluentTimeChanges(fluentMs)
}

func (w *world) isStillFluentOnThatCell(student string) error {
	return w.levelForTheCellIs(student, string(domain.KnowledgeLevelFluent))
}

func (w *world) hasNeverDoneATapCheck(student string) error {
	delete(w.tapMs, student)
	w.studentID(student)
	return nil
}

func (w *world) templateHasNoVersion(template string) error {
	delete(w.reference.fluentTimes, template)
	return nil
}

func (w *world) answerCountsTowardAccuracy(student string) error {
	fold, err := w.cellFold(student)
	if err != nil {
		return err
	}
	if fold.Counted != 1 || fold.Accuracy < 1-1e-9 {
		return fmt.Errorf("the cell has %d counted answers at accuracy %.2f, want the one right answer counted", fold.Counted, fold.Accuracy)
	}
	return nil
}

func (w *world) neverCountsTowardFluency() error {
	template := "fretboard_cell:find_the_note"
	w.reference.fluentTimes[template] = []domain.FluentTime{{Version: 1, EffectiveFrom: w.clock, FluentNetMs: 4000}}
	if err := w.redeliver(); err != nil {
		return err
	}
	fold, err := w.cellFold(w.lastStudent)
	if err != nil {
		return err
	}
	if fold.Fluency != 0 {
		return fmt.Errorf("the rebuilt cell's fluency is %.3f, want the answer to count for no fluency", fold.Fluency)
	}
	return nil
}

func (w *world) practiceDrillCatalogIsInstalled() error {
	raw, err := os.ReadFile(filepath.Join(specsDir, "catalogs", "practice-drills.yaml"))
	if err != nil {
		return err
	}
	var file struct {
		Thresholds []catalogThreshold `yaml:"thresholds"`
	}
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return err
	}
	w.cells.catalog = map[string][]catalogThreshold{}
	for _, t := range file.Thresholds {
		w.cells.catalog[t.Template] = append(w.cells.catalog[t.Template], t)
	}
	return nil
}

func (w *world) catalogHasVersion1(template, source string, fluentMs int) error {
	for _, t := range w.cells.catalog[template] {
		if t.Version == 1 {
			if t.Source != source || t.FluentNetMs != fluentMs {
				return fmt.Errorf("%s v1 is %d ms from %q, want %d ms from %q", template, t.FluentNetMs, t.Source, fluentMs, source)
			}
			return nil
		}
	}
	return fmt.Errorf("the catalog has no version 1 of %s", template)
}
