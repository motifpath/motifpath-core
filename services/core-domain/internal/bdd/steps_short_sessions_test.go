//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerShortSessionSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" has nothing due or weak, and (\d+) new fretboard cells and (\d+) new shapes on guitar$`, w.hasNewCellsAndShapes)
	sc.Step(`^"([^"]+)" has (\d+) fretboard cells due since 3 days ago and (\d+) due since yesterday on guitar$`, w.hasCellsDueOnTwoDays)

	sc.Step(`^student "([^"]+)"'s only path skill is "([^"]+)", practised by fretboard cells on guitar$`, w.onlyPathSkillIsCells)
	sc.Step(`^no item in the session is a fretboard cell or a diagram shape$`, w.noCellOrShapeInSession)

	sc.Step(`^no drill has more than (\d+) items in the session$`, w.noDrillHasMoreThan)
	sc.Step(`^the session's items take less than (\d+) minutes$`, w.sessionTakesLessThan)
	sc.Step(`^the session has more than (\d+) exercises of one type$`, w.sessionHasMoreExercisesOfOneType)
	sc.Step(`^the session's fretboard cells are not in string and fret order$`, w.cellsNotInStringAndFretOrder)
	sc.Step(`^the (\d+) cells due since 3 days ago are in the session$`, w.oldestDueCellsAreIn)
	sc.Step(`^its other due cells are some of those due since yesterday$`, w.otherDueCellsAreYesterdays)
	sc.Step(`^no two items in a row are of the same drill while another drill has items left$`, w.drillsTakeTurns)
	sc.Step(`^no two fretboard cells in a row are on the same string while a cell on another string is left$`, w.noSameStringTwiceInARow)
}

// cellsDueOnTwoDays holds the cells a scenario made due on each of two days.
type cellsDueOnTwoDays struct {
	oldest, yesterday []string
}

// hasNewCellsAndShapes puts cells guitar cells and the first shapes catalog
// CAGED grips on name's path, none of them practised.
func (w *world) hasNewCellsAndShapes(name string, cells, shapes int) error {
	const strings = 4
	w.putCells("guitar", practiceSkill, cells/strings, 6, 5, 4, 3)
	if err := w.loadShapeCatalog(); err != nil {
		return err
	}
	w.addGuitarPathSkill(name, "map-fretboard-caged")
	put := 0
	for _, shape := range w.diagramShapes().shapes {
		if put == shapes {
			break
		}
		if catalogFamilyClassification[shape.diagram.Family][0] != "map-fretboard-caged" {
			continue
		}
		if err := w.putCatalogShape(shape); err != nil {
			return err
		}
		put++
	}
	if put < shapes {
		return fmt.Errorf("the catalog has only %d CAGED shapes, want %d", put, shapes)
	}
	return nil
}

// hasCellsDueOnTwoDays makes oldest guitar cells due since 3 days ago and
// yesterday more due since yesterday, each a minute after the one before,
// the way answers in one session fall due.
func (w *world) hasCellsDueOnTwoDays(name string, oldest, yesterday int) error {
	const frets = 12
	w.putCells("guitar", practiceSkill, frets, 6, 5, 4)
	layout := instrumentID("guitar").String()
	days := &cellsDueOnTwoDays{}
	for i := range oldest + yesterday {
		str, fret := 6-i/frets, i%frets
		dueAt := fixedNow.AddDate(0, 0, -3)
		if i >= oldest {
			dueAt = fixedNow.AddDate(0, 0, -1).Add(time.Duration(i) * time.Minute)
		}
		w.putCellState(name, "guitar", str, fret, domain.PracticeItemState{
			Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: &dueAt, LastAt: &dueAt,
		})
		key := domain.FretboardCellItemKey(layout, str, fret)
		if i < oldest {
			days.oldest = append(days.oldest, key)
		} else {
			days.yesterday = append(days.yesterday, key)
		}
	}
	w.dueDays = days
	return nil
}

// drillOf is the drill an item is asked by: its drill template, or its kind
// for a play-along.
func drillOf(item generated.PracticeSessionItem) string {
	switch {
	case item.FretboardCell != nil:
		return string(item.Kind) + ":" + string(item.FretboardCell.Drill)
	case item.DiagramShape != nil:
		return string(item.Kind) + ":" + string(item.DiagramShape.Drill)
	case item.Exercise != nil:
		return string(item.Kind) + ":" + string(item.Exercise.ExerciseType)
	}
	return string(item.Kind)
}

func (w *world) noDrillHasMoreThan(most int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, item := range plan.Items {
		counts[drillOf(item)]++
	}
	for drill, n := range counts {
		if n > most {
			return fmt.Errorf("the session has %d items of %s, want at most %d", n, drill, most)
		}
	}
	return nil
}

func (w *world) sessionTakesLessThan(minutes int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	seconds := 0
	for _, item := range plan.Items {
		seconds += item.EstimatedSeconds
	}
	if seconds >= minutes*60 {
		return fmt.Errorf("the session's items take %d s, want less than %d minutes", seconds, minutes)
	}
	return nil
}

func (w *world) sessionHasMoreExercisesOfOneType(least int) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, item := range plan.Items {
		if item.Exercise != nil {
			counts[string(item.Exercise.ExerciseType)]++
		}
	}
	for _, n := range counts {
		if n > least {
			return nil
		}
	}
	return fmt.Errorf("the session's exercises by type are %v, want more than %d of one", counts, least)
}

// cellsInARow calls check on each fretboard cell item right after another,
// with the items left from it on.
func (w *world) cellsInARow(check func(prev, item generated.PracticeSessionItem, left []generated.PracticeSessionItem) error) error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for i := 1; i < len(plan.Items); i++ {
		if plan.Items[i-1].FretboardCell == nil || plan.Items[i].FretboardCell == nil {
			continue
		}
		if err := check(plan.Items[i-1], plan.Items[i], plan.Items[i:]); err != nil {
			return err
		}
	}
	return nil
}

// cellsNotInStringAndFretOrder checks the session's cells aren't in the
// order the generator lists them: string by string from the lowest-pitched,
// fret by fret along each.
func (w *world) cellsNotInStringAndFretOrder() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	type place struct{ str, fret int }
	var cells []place
	for _, item := range plan.Items {
		if c := item.FretboardCell; c != nil {
			cells = append(cells, place{c.String, c.Fret})
		}
	}
	if len(cells) < 3 {
		return fmt.Errorf("the session has %d fretboard cells, too few to tell their order", len(cells))
	}
	inOrder := slices.IsSortedFunc(cells, func(a, b place) int {
		if a.str != b.str {
			return b.str - a.str
		}
		return a.fret - b.fret
	})
	if inOrder {
		return fmt.Errorf("the session's cells %v run string by string, fret by fret", cells)
	}
	return nil
}

func (w *world) noSameStringTwiceInARow() error {
	return w.cellsInARow(func(prevItem, item generated.PracticeSessionItem, left []generated.PracticeSessionItem) error {
		prev, cell := prevItem.FretboardCell, item.FretboardCell
		if prev.LayoutInstrumentId != cell.LayoutInstrumentId || prev.String != cell.String {
			return nil
		}
		other := slices.ContainsFunc(left, func(item generated.PracticeSessionItem) bool {
			c := item.FretboardCell
			return c != nil && c.Drill == cell.Drill && (c.LayoutInstrumentId != prev.LayoutInstrumentId || c.String != prev.String)
		})
		if other {
			return fmt.Errorf("two cells in a row on string %d, while a cell on another string was left", cell.String)
		}
		return nil
	})
}

func (w *world) dueCellsInSession() ([]string, error) {
	plan, err := w.composedPlan()
	if err != nil {
		return nil, err
	}
	var due []string
	for _, item := range plan.Items {
		if item.FretboardCell != nil && item.Reason == generated.PracticePickReasonDue {
			due = append(due, item.ItemKey)
		}
	}
	return due, nil
}

func (w *world) oldestDueCellsAreIn(count int) error {
	due, err := w.dueCellsInSession()
	if err != nil {
		return err
	}
	if len(w.dueDays.oldest) != count {
		return fmt.Errorf("the scenario made %d cells due since 3 days ago, want %d", len(w.dueDays.oldest), count)
	}
	for _, key := range w.dueDays.oldest {
		if !slices.Contains(due, key) {
			return fmt.Errorf("%s, due since 3 days ago, is not in the session's due cells %v", key, due)
		}
	}
	return nil
}

func (w *world) otherDueCellsAreYesterdays() error {
	due, err := w.dueCellsInSession()
	if err != nil {
		return err
	}
	others := 0
	for _, key := range due {
		if slices.Contains(w.dueDays.oldest, key) {
			continue
		}
		if !slices.Contains(w.dueDays.yesterday, key) {
			return fmt.Errorf("the session's due cell %s fell due neither 3 days ago nor yesterday", key)
		}
		others++
	}
	if others == 0 {
		return fmt.Errorf("the session has no due cell from yesterday")
	}
	return nil
}

func (w *world) drillsTakeTurns() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for i := 1; i < len(plan.Items); i++ {
		drill := drillOf(plan.Items[i])
		if drill != drillOf(plan.Items[i-1]) {
			continue
		}
		for _, later := range plan.Items[i:] {
			if drillOf(later) != drill {
				return fmt.Errorf("item %d repeats %s while %s has items left", i+1, drill, drillOf(later))
			}
		}
	}
	return nil
}

// onlyPathSkillIsCells signs name in on a path whose only skill is
// practised by guitar fretboard cells: nothing to play. (A catalog shape
// plays, so with the instrument in hand its diagram is a play-along.)
func (w *world) onlyPathSkillIsCells(name, skill string) error {
	w.authenticateAs(name, domain.RoleStudent)
	return w.noHistoryPathHasSkill(name, skill)
}

func (w *world) noCellOrShapeInSession() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if item.FretboardCell != nil || item.DiagramShape != nil {
			return fmt.Errorf("the session has %s, which is recalled in the head", item.ItemKey)
		}
	}
	return nil
}
