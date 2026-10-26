//go:build integration

package bdd

import (
	"fmt"
	"slices"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/domain"
)

func registerEveryInstanceSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" has named the note of the "([^"]+)" cell on string (\d+), fret (\d+) correctly (once|twice), found it correctly (once|twice) and found every instance of it correctly (once|twice)$`, w.cellAnsweredThreeWays)
	sc.Step(`^the "([^"]+)" cells on string (\d+), fret (\d+) and string (\d+), fret (\d+) are both due, to be asked as (\w+)$`, w.twoCellsDueAs)
	sc.Step(`^at most one of them is asked as (\w+)$`, w.atMostOneOfThemIsAskedAs)
	sc.Step(`^the other one, if picked, is asked as (\w+) or (\w+)$`, w.theOtherIsAskedAsEither)
	sc.Step(`^every fretboard cell asked as (\w+) is estimated at (\d+) seconds$`, w.everyCellAskedAsEstimatedAt)
}

var timesWord = map[string]int{"once": 1, "twice": 2}

func (w *world) cellAnsweredThreeWays(name, layout string, str, fret int, named, found, everywhere string) error {
	right := map[string]int{
		string(domain.FretboardDrillNameTheNote):       timesWord[named],
		string(domain.FretboardDrillFindTheNote):       timesWord[found],
		string(domain.FretboardDrillFindEveryInstance): timesWord[everywhere],
	}
	w.cellDue(name, layout, str, fret, right[string(domain.FretboardDrillNameTheNote)]+right[string(domain.FretboardDrillFindTheNote)]+right[string(domain.FretboardDrillFindEveryInstance)], right)
	return nil
}

// twoCellsDueAs makes two cells of layout due to be asked as drill, on a
// layout seeded with its real tuning so their notes can be read.
func (w *world) twoCellsDueAs(layout string, str1, fret1, str2, fret2 int, drill string) error {
	if _, err := w.seedCatalogInstrument(layout); err != nil {
		return err
	}
	w.everyInstancePair = nil
	for _, p := range []cellPlace{{str: str1, fret: fret1}, {str: str2, fret: fret2}} {
		w.everyInstancePair = append(w.everyInstancePair, domain.FretboardCellItemKey(instrumentID(layout).String(), p.str, p.fret))
	}
	return w.putCellsAskedAs(layout, drill, dueState(), []cellPlace{{str: str1, fret: fret1}, {str: str2, fret: fret2}})
}

// pairDrills is how the session asks each of the two cells a step named, by
// item key; a cell not picked is missing.
func (w *world) pairDrills() (map[string]domain.FretboardDrill, error) {
	cells, err := w.plannedCells()
	if err != nil {
		return nil, err
	}
	drills := map[string]domain.FretboardDrill{}
	for _, item := range cells {
		if slices.Contains(w.everyInstancePair, item.ItemKey) {
			drills[item.ItemKey] = domain.FretboardDrill(item.FretboardCell.Drill)
		}
	}
	return drills, nil
}

func (w *world) atMostOneOfThemIsAskedAs(drill string) error {
	drills, err := w.pairDrills()
	if err != nil {
		return err
	}
	asked := 0
	for _, d := range drills {
		if string(d) == drill {
			asked++
		}
	}
	if asked > 1 {
		return fmt.Errorf("both cells are asked as %s", drill)
	}
	return nil
}

func (w *world) theOtherIsAskedAsEither(first, second string) error {
	drills, err := w.pairDrills()
	if err != nil {
		return err
	}
	for key, d := range drills {
		if d == domain.FretboardDrillFindEveryInstance {
			continue
		}
		if string(d) != first && string(d) != second {
			return fmt.Errorf("%s is asked as %s, want %s or %s", key, d, first, second)
		}
	}
	return nil
}

func (w *world) everyCellAskedAsEstimatedAt(drill string, seconds int) error {
	cells, err := w.plannedCells()
	if err != nil {
		return err
	}
	found := false
	for _, item := range cells {
		if string(item.FretboardCell.Drill) != drill {
			continue
		}
		found = true
		if item.EstimatedSeconds != seconds {
			return fmt.Errorf("expected %s at %d seconds, got %d", item.ItemKey, seconds, item.EstimatedSeconds)
		}
	}
	if !found {
		return fmt.Errorf("expected a fretboard cell asked as %s in the session", drill)
	}
	return nil
}
