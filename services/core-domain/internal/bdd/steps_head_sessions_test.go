//go:build integration

package bdd

import (
	"fmt"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerHeadSessionSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" has nothing due or weak on guitar and new fretboard cells on the E and A strings$`, w.hasNewCellsOnRootStrings)
	sc.Step(`^"([^"]+)" has named the note of the "([^"]+)" cell on string (\d+), fret (\d+) correctly (\d+) times and found it correctly once$`, w.namedCellAndFoundOnce)
	sc.Step(`^"([^"]+)" has never answered the "([^"]+)" cell on string (\d+), fret (\d+) correctly$`, w.neverAnsweredCellRight)
	sc.Step(`^"([^"]+)" has (\d+) new items on guitar and (\d+) new items on electric bass$`, w.hasNewItemsOnBoth)
	sc.Step(`^"([^"]+)" has no practice history and their path has the skill "([^"]+)"$`, w.noHistoryPathHasSkill)
	sc.Step(`^student "([^"]+)" is enrolled in nothing and has no practice history$`, w.enrolledInNothing)

	sc.Step(`^"([^"]+)" composes a (\d+)-minute session with no instrument in hand$`, w.composesSessionInTheHead)
	sc.Step(`^the cell is picked for "([^"]+)"'s session in the head$`, w.cellPickedInTheHead)

	sc.Step(`^the session includes fretboard cells of the "([^"]+)" layout$`, w.sessionIncludesCellsOf)
	sc.Step(`^no item in the session is a play-along$`, w.noItemIsPlayAlong)
	sc.Step(`^the session may include fretboard cells of both "([^"]+)" and "([^"]+)"$`, w.sessionCellsOnlyOf)
	sc.Step(`^no item in the session has the reason (\w+) or (\w+)$`, w.noItemHasEitherReason)
	sc.Step(`^it is asked as (\w+)$`, w.cellIsAskedAs)
	sc.Step(`^every fretboard cell in the session is estimated at (\d+) seconds$`, w.everyCellEstimatedAt)
	sc.Step(`^the new items in the session are split evenly between guitar and electric bass$`, w.newItemsSplitEvenly)
	sc.Step(`^the session has at least one fretboard cell with the reason (\w+)$`, w.sessionHasCellWithReason)
}

// putCells declares the fretboard cells of strs at frets 0 to frets-1 on
// instrument's own layout, classified under skill.
func (w *world) putCells(instrument, skill string, frets int, strs ...int) {
	layout := instrumentID(instrument).String()
	w.practiceItems.layouts[layout] = layout
	for _, str := range strs {
		for fret := range frets {
			w.practiceItems.cells[layout] = append(w.practiceItems.cells[layout], domain.ClassifiedItem{
				ItemKey: domain.FretboardCellItemKey(layout, str, fret),
				NodeIDs: []string{w.skillIDFor(skill).String()},
			})
		}
	}
}

func (w *world) hasNewCellsOnRootStrings(string) error {
	w.putCells("guitar", practiceSkill, 12, 6, 5)
	return nil
}

// cellDue gives name a due state on the cell after counted answers, with
// right of them by each drill, and remembers the cell for "the cell" steps.
func (w *world) cellDue(name, layout string, str, fret, counted int, right map[string]int) {
	id := instrumentID(layout).String()
	w.practiceItems.layouts[id] = id
	w.practiceItems.cells[id] = append(w.practiceItems.cells[id], domain.ClassifiedItem{
		ItemKey: domain.FretboardCellItemKey(id, str, fret), NodeIDs: []string{w.skillIDFor(practiceSkill).String()},
	})
	due := fixedNow.Add(-time.Hour)
	w.putCellState(name, layout, str, fret, domain.PracticeItemState{
		Level: domain.KnowledgeLevelLearning, Counted: counted, Box: 1, DueAt: &due, LastAt: &fixedNow, RightByResponse: right,
	})
	w.lastCell = &lastCell{student: name, layout: layout, str: str, fret: fret}
}

func (w *world) namedCellAndFoundOnce(name, layout string, str, fret, named int) error {
	w.cellDue(name, layout, str, fret, named+1, map[string]int{string(domain.FretboardDrillNameTheNote): named, string(domain.FretboardDrillFindTheNote): 1})
	return nil
}

// neverAnsweredCellRight makes the cell due after two wrong answers, so the
// session in the head picks it.
func (w *world) neverAnsweredCellRight(name, layout string, str, fret int) error {
	w.cellDue(name, layout, str, fret, 2, nil)
	return nil
}

// hasNewItemsOnBoth declares count new cells on each instrument's layout,
// ten frets per string.
func (w *world) hasNewItemsOnBoth(_ string, guitar, bass int) error {
	w.putCells("guitar", practiceSkill, 10, []int{6, 5, 4, 3}[:guitar/10]...)
	w.putCells("electric-bass", practiceSkill, 10, []int{4, 3, 2, 1}[:bass/10]...)
	return nil
}

// noHistoryPathHasSkill gives name a path teaching skill, with guitar cells
// on its root strings.
func (w *world) noHistoryPathHasSkill(name, skill string) error {
	w.addPathSkill(name, skill)
	w.putCells("guitar", skill, 12, 6, 5)
	return nil
}

func (w *world) enrolledInNothing(name string) error {
	w.authenticateAs(name, domain.RoleStudent)
	return nil
}

func (w *world) composesSessionInTheHead(_ string, minutes int) error {
	return w.composeSession(minutes, nil)
}

func (w *world) cellPickedInTheHead(string) error {
	return w.composeSession(pickMinutes, nil)
}

// plannedCells lists the composed plan's fretboard cells.
func (w *world) plannedCells() ([]generated.PracticeSessionItem, error) {
	plan, err := w.composedPlan()
	if err != nil {
		return nil, err
	}
	var cells []generated.PracticeSessionItem
	for _, item := range plan.Items {
		if item.FretboardCell != nil {
			cells = append(cells, item)
		}
	}
	return cells, nil
}

func (w *world) sessionIncludesCellsOf(layout string) error {
	cells, err := w.plannedCells()
	if err != nil {
		return err
	}
	for _, item := range cells {
		if item.FretboardCell.LayoutInstrumentId == instrumentID(layout) {
			return nil
		}
	}
	return fmt.Errorf("expected fretboard cells of the %q layout in the session, got %d cells", layout, len(cells))
}

func (w *world) noItemIsPlayAlong() error {
	plan, err := w.composedPlan()
	if err != nil {
		return err
	}
	for _, item := range plan.Items {
		if item.PlayAlong != nil {
			return fmt.Errorf("expected no play-along, got %s", item.ItemKey)
		}
	}
	return nil
}

func (w *world) sessionCellsOnlyOf(first, second string) error {
	cells, err := w.plannedCells()
	if err != nil {
		return err
	}
	for _, item := range cells {
		if id := item.FretboardCell.LayoutInstrumentId; id != instrumentID(first) && id != instrumentID(second) {
			return fmt.Errorf("expected cells of %q or %q only, got %s", first, second, item.ItemKey)
		}
	}
	return nil
}

func (w *world) noItemHasEitherReason(first, second string) error {
	if err := w.noItemHasReason(first); err != nil {
		return err
	}
	return w.noItemHasReason(second)
}

func (w *world) cellIsAskedAs(drill string) error {
	if w.lastCell == nil {
		return fmt.Errorf("no cell was set up before")
	}
	key := domain.FretboardCellItemKey(instrumentID(w.lastCell.layout).String(), w.lastCell.str, w.lastCell.fret)
	cells, err := w.plannedCells()
	if err != nil {
		return err
	}
	for _, item := range cells {
		if item.ItemKey == key {
			if string(item.FretboardCell.Drill) != drill {
				return fmt.Errorf("expected %s to be asked as %s, got %s", key, drill, item.FretboardCell.Drill)
			}
			return nil
		}
	}
	return fmt.Errorf("expected %s in the session", key)
}

func (w *world) everyCellEstimatedAt(seconds int) error {
	cells, err := w.plannedCells()
	if err != nil {
		return err
	}
	if len(cells) == 0 {
		return fmt.Errorf("expected fretboard cells in the session")
	}
	for _, item := range cells {
		if item.EstimatedSeconds != seconds {
			return fmt.Errorf("expected %s at %d seconds, got %d", item.ItemKey, seconds, item.EstimatedSeconds)
		}
	}
	return nil
}

func (w *world) newItemsSplitEvenly() error {
	cells, err := w.plannedCells()
	if err != nil {
		return err
	}
	guitar, bass := 0, 0
	for _, item := range cells {
		if item.Reason != generated.PracticePickReasonNew {
			continue
		}
		switch item.FretboardCell.LayoutInstrumentId {
		case instrumentID("guitar"):
			guitar++
		case instrumentID("electric-bass"):
			bass++
		}
	}
	if guitar == 0 || abs(guitar-bass) > 1 {
		return fmt.Errorf("expected the new items split evenly, got %d on guitar and %d on electric bass", guitar, bass)
	}
	return nil
}

func (w *world) sessionHasCellWithReason(reason string) error {
	cells, err := w.plannedCells()
	if err != nil {
		return err
	}
	for _, item := range cells {
		if string(item.Reason) == reason {
			return nil
		}
	}
	return fmt.Errorf("expected a fretboard cell with the reason %s, got %d cells", reason, len(cells))
}
