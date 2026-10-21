//go:build integration

package bdd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// everyInstanceWorld is what the find every instance steps share in a scenario.
type everyInstanceWorld struct {
	// cells are the guitar cells the latest step named, for a step about "each of
	// them" or "each of the three cells".
	cells []string
}

// everyInstanceLatency is an answer's time when a scenario doesn't state one.
const everyInstanceLatency = 5000

func registerEveryInstanceSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]*)" answers the guitar cell on string (\d+) at fret (\d+) by tapping (.+) in frets (\d+) to (\d+)(?: after (\d+) milliseconds)?$`, w.answersByTappingEveryInstance)
	sc.Step(`^"([^"]*)" answered the guitar cell on string (\d+) at fret (\d+) by tapping every ([A-G]#?) in frets (\d+) to (\d+)$`, w.answeredByTappingEveryPlaceOf)
	sc.Step(`^"([^"]*)" has auto-graded evidence that is (correct|wrong) for (?:each of )?the guitar cells? (.+)$`, w.hasEvidenceOnTheGuitarCells)
	sc.Step(`^each of them names the grader "([^"]*)"$`, w.eachOfThemNamesTheGrader)
	sc.Step(`^its answer key names the note "([^"]*)", which is not that cell's note$`, w.answerKeyNamesAnotherNote)
	sc.Step(`^"([^"]*)" has evidence for the guitar cell on string (\d+) at fret (\d+), of skill "([^"]*)"$`, w.hasEvidenceForTheCellOfSkill)
	sc.Step(`^none of "([^"]*)"'s evidence from that answer has a latency$`, w.noEvidenceFromThatAnswerHasALatency)
	sc.Step(`^each cell's evidence is identified by the name-based UUID of its item key under the practice\.item_answered event's identifier$`, w.eachCellsEvidenceIsIdentifiedByItsKey)
	sc.Step(`^the same practice\.item_answered event is delivered again$`, w.theSameEventIsDeliveredAgain)
	sc.Step(`^"([^"]*)" still has one piece of evidence for each of the three cells$`, w.stillHasOnePieceOfEvidenceForEachCell)
	sc.Step(`^the answer is rejected because a cell is invalid$`, w.answerRejectedBecause(domain.GradeRejectionInvalidCell))
	sc.Step(`^"([^"]*)" has no evidence for any guitar cell$`, w.hasNoEvidenceForAnyGuitarCell)
	sc.Step(`^"([^"]*)" finds every instance of the cell's note correctly (\d+) times on (\d+) different days, each in (\d+) milliseconds$`, w.findsEveryInstanceOnDifferentDays)
	sc.Step(`^"([^"]*)" has found every instance of the cell's note correctly (\d+) times$`, w.hasFoundEveryInstance)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

var placePattern = regexp.MustCompile(`string (\d+) fret (\d+)`)

// guitarCellsIn reads "string 6 fret 3, string 1 fret 3 and string 4 fret 5".
func guitarCellsIn(list string) ([]domain.AnswerCell, error) {
	var cells []domain.AnswerCell
	for _, m := range placePattern.FindAllStringSubmatch(list, -1) {
		str, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, err
		}
		fret, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, err
		}
		cells = append(cells, domain.AnswerCell{String: str, Fret: fret})
	}
	return cells, nil
}

// stringAndFretOf reads a fretboard cell key's string and fret.
func stringAndFretOf(itemKey string) (int, int, error) {
	parts := strings.Split(itemKey, ":")
	str, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return 0, 0, err
	}
	fret, err := strconv.Atoi(parts[len(parts)-1])
	return str, fret, err
}

// placesOfInWindow is every guitar cell of frets from to to that sounds note.
func (w *world) placesOfInWindow(note string, from, to int) ([]domain.AnswerCell, error) {
	var places []domain.AnswerCell
	for str := 1; str <= len(standardGuitar); str++ {
		for fret := from; fret <= to; fret++ {
			got, err := noteOf(w.guitarCell(str, fret), 0)
			if err != nil {
				return nil, err
			}
			if got == note {
				places = append(places, domain.AnswerCell{String: str, Fret: fret})
			}
		}
	}
	return places, nil
}

// tapEveryInstance answers the planned cell by tapping taps in the window that
// starts at from.
func (w *world) tapEveryInstance(student string, str, fret, from int, taps []domain.AnswerCell, latency int) error {
	planned := w.guitarCell(str, fret)
	w.everyInstance.cells = nil
	for _, t := range taps {
		w.everyInstance.cells = append(w.everyInstance.cells, w.guitarCell(t.String, t.Fret))
	}
	w.cells.key = planned
	if err := w.answer(student, planned, domain.PracticeResponse{
		Type: domain.PracticeResponseMultiTap, FretWindowFrom: &from, Cells: taps, LatencyMs: &latency,
	}); err != nil {
		return err
	}
	w.cells.lastAnswer = w.lastAnswer
	return nil
}

// answerEvidence is every piece of evidence the latest answer gave.
func (w *world) answerEvidence() []domain.PracticeEvidence {
	var out []domain.PracticeEvidence
	for _, e := range w.evidence.stored {
		if e.Response.Type == domain.PracticeResponseMultiTap && e.OccurredAt.Equal(w.lastAnswer.OccurredAt) {
			out = append(out, e)
		}
	}
	return out
}

// ── Grading ──────────────────────────────────────────────────────────────────

func (w *world) answersByTappingEveryInstance(student string, str, fret int, taps string, from, to int, latency string) error {
	if to != from+4 && !(from == 0 && to == 5) {
		return fmt.Errorf("frets %d to %d is not a fret window", from, to)
	}
	ms := everyInstanceLatency
	if latency != "" {
		var err error
		if ms, err = strconv.Atoi(latency); err != nil {
			return err
		}
	}
	cells, err := guitarCellsIn(taps)
	if err != nil {
		return err
	}
	return w.tapEveryInstance(student, str, fret, from, cells, ms)
}

func (w *world) answeredByTappingEveryPlaceOf(student string, str, fret int, note string, from, to int) error {
	places, err := w.placesOfInWindow(note, from, to)
	if err != nil {
		return err
	}
	return w.tapEveryInstance(student, str, fret, from, places, everyInstanceLatency)
}

func (w *world) hasEvidenceOnTheGuitarCells(student, verdict, cells string) error {
	list := cells
	if m := regexp.MustCompile(`^on string (\d+) at fret (\d+)$`).FindStringSubmatch(cells); m != nil {
		list = "string " + m[1] + " fret " + m[2]
	}
	named, err := guitarCellsIn(list)
	if err != nil {
		return err
	}
	if len(named) == 0 {
		return fmt.Errorf("no guitar cell named in %q", cells)
	}
	w.everyInstance.cells = nil
	for _, c := range named {
		key := w.guitarCell(c.String, c.Fret)
		w.everyInstance.cells = append(w.everyInstance.cells, key)
		e, err := w.onlyEvidenceFor(student, key)
		if err != nil {
			return err
		}
		if e.Source != domain.EvidenceSourceAutoGraded || e.Correct == nil || *e.Correct != (verdict == "correct") {
			return fmt.Errorf("evidence for string %d fret %d is %s, correct = %v; want auto-graded and %s", c.String, c.Fret, e.Source, e.Correct, verdict)
		}
	}
	return nil
}

func (w *world) eachOfThemNamesTheGrader(grader string) error {
	for _, key := range w.everyInstance.cells {
		e, err := w.onlyEvidenceFor(w.lastStudent, key)
		if err != nil {
			return err
		}
		if e.GraderID != grader {
			return fmt.Errorf("%s was graded by %s, want %s", key, e.GraderID, grader)
		}
	}
	return nil
}

func (w *world) answerKeyNamesAnotherNote(note string) error {
	key := w.everyInstance.cells[len(w.everyInstance.cells)-1]
	e, err := w.onlyEvidenceFor(w.lastStudent, key)
	if err != nil {
		return err
	}
	own, err := noteOf(key, 0)
	if err != nil {
		return err
	}
	if e.AnswerKey == nil || e.AnswerKey.NoteName != note || own == note {
		return fmt.Errorf("answer key of %s (a %s) names %v, want %s", key, own, e.AnswerKey, note)
	}
	return nil
}

func (w *world) hasEvidenceForTheCellOfSkill(student string, str, fret int, skill string) error {
	if _, err := w.onlyEvidenceFor(student, w.guitarCell(str, fret)); err != nil {
		return err
	}
	skillStrings, err := guitarStringsOfSkill(skill)
	if err != nil {
		return err
	}
	if !slices.Contains(skillStrings, str) {
		return fmt.Errorf("string %d is not a string of %s on guitar (%v)", str, skill, skillStrings)
	}
	return nil
}

// guitarStringsOfSkill reads the guitar strings a skill's cells are on from the
// drill catalog.
func guitarStringsOfSkill(skill string) ([]int, error) {
	raw, err := os.ReadFile(filepath.Join(specsDir, "catalogs", "practice-drills.yaml"))
	if err != nil {
		return nil, err
	}
	var file struct {
		FretboardCells []struct {
			Skill   string `yaml:"skill"`
			Layouts map[string]struct {
				Strings []int `yaml:"strings"`
			} `yaml:"layouts"`
		} `yaml:"fretboard_cells"`
	}
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	for _, c := range file.FretboardCells {
		if c.Skill == skill {
			return c.Layouts["guitar"].Strings, nil
		}
	}
	return nil, fmt.Errorf("the drill catalog has no fretboard cells for %s", skill)
}

func (w *world) noEvidenceFromThatAnswerHasALatency(_ string) error {
	evidence := w.answerEvidence()
	if len(evidence) == 0 {
		return fmt.Errorf("the answer gave no evidence")
	}
	for _, e := range evidence {
		if e.LatencyMs != nil {
			return fmt.Errorf("%s has a latency of %d ms", e.ItemKey, *e.LatencyMs)
		}
	}
	return nil
}

func (w *world) eachCellsEvidenceIsIdentifiedByItsKey() error {
	event, err := uuid.Parse(w.lastAnswer.EventID)
	if err != nil {
		return err
	}
	evidence := w.answerEvidence()
	if len(evidence) == 0 {
		return fmt.Errorf("the answer gave no evidence")
	}
	for _, e := range evidence {
		if want := uuid.NewSHA1(event, []byte(e.ItemKey)).String(); e.EvidenceID != want {
			return fmt.Errorf("%s's evidence is %s, want %s", e.ItemKey, e.EvidenceID, want)
		}
	}
	return nil
}

func (w *world) theSameEventIsDeliveredAgain() error {
	return w.service.Process(context.Background(), w.cells.lastAnswer)
}

func (w *world) stillHasOnePieceOfEvidenceForEachCell(student string) error {
	if len(w.everyInstance.cells) != 3 {
		return fmt.Errorf("the scenario named %d cells, want 3", len(w.everyInstance.cells))
	}
	for _, key := range w.everyInstance.cells {
		if _, err := w.onlyEvidenceFor(student, key); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) hasNoEvidenceForAnyGuitarCell(student string) error {
	id := w.studentID(student)
	for _, e := range w.evidence.stored {
		if e.StudentID == id && strings.HasPrefix(e.ItemKey, "fretboard_cell:") {
			return fmt.Errorf("want no evidence for any guitar cell, got some for %s", e.ItemKey)
		}
	}
	return nil
}

// ── Knowledge state ──────────────────────────────────────────────────────────

// findEveryInstanceOfTheCell taps every place of the current cell's note in the
// window of frets 3 to 7, which holds every cell scenario's default cell.
func (w *world) findEveryInstanceOfTheCell(student string, latency int) error {
	planned := w.currentCell()
	note, err := noteOf(planned, 0)
	if err != nil {
		return err
	}
	str, fret, err := stringAndFretOf(planned)
	if err != nil {
		return err
	}
	from := 3
	places, err := w.placesOfInWindow(note, from, from+4)
	if err != nil {
		return err
	}
	return w.tapEveryInstance(student, str, fret, from, places, latency)
}

func (w *world) findsEveryInstanceOnDifferentDays(student string, times, days, latency int) error {
	if times != days {
		return fmt.Errorf("one answer a day: %d answers on %d days", times, days)
	}
	planned := w.currentCell()
	for range times {
		w.cells.key = planned
		if err := w.findEveryInstanceOfTheCell(student, latency); err != nil {
			return err
		}
		w.clock = w.clock.AddDate(0, 0, 1)
	}
	w.cells.key = planned
	return nil
}

func (w *world) hasFoundEveryInstance(student string, times int) error {
	planned := w.currentCell()
	for range times {
		w.cells.key = planned
		if err := w.findEveryInstanceOfTheCell(student, everyInstanceLatency); err != nil {
			return err
		}
	}
	w.cells.key = planned
	return nil
}
