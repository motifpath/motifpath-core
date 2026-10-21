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

func registerFretWindowSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" has started (\d+) sessions in the head$`, w.hasStartedHeadSessions)
	sc.Step(`^"([^"]+)" has nothing due on guitar$`, w.hasNothingDueOnGuitar)
	sc.Step(`^"([^"]+)" has nothing due or weak, and (\d+) new fretboard cells on guitar$`, w.hasNewCellsOnGuitar)
	sc.Step(`^(\d+) weak "([^"]+)" cells across frets (\d+) to (\d+), to be asked as (\w+)$`, w.weakCellsAcrossFrets)
	sc.Step(`^(\d+) weak "([^"]+)" cells at frets (\d+) to (\d+), to be asked as (\w+)$`, w.weakCellsAtFrets)
	sc.Step(`^(\d+) "([^"]+)" cells due at frets (\d+) to (\d+), to be asked as (\w+)$`, w.dueCellsAtFrets)
	sc.Step(`^(\d+) "([^"]+)" cells due at frets (\d+) and (\d+), to be asked as (\w+)$`, w.dueCellsAtTwoFrets)
	sc.Step(`^(\d+) "([^"]+)" cells due at frets (\d+) to (\d+) and (\d+) due at fret (\d+), all to be asked as (\w+)$`, w.dueCellsAndOneMore)
	sc.Step(`^every "([^"]+)" cell at frets (\d+) to (\d+) has been answered, (\d+) of them due, to be asked as (\w+)$`, w.everyCellAnswered)
	sc.Step(`^the "([^"]+)" cells at frets (\d+) to (\d+) are new$`, w.cellsAreNew)

	sc.Step(`^the session has "([^"]+)" cells asked as (\w+) and as (\w+)$`, w.sessionHasCellsAskedBoth)
	sc.Step(`^every "([^"]+)" (\w+) cell in the session carries the same fret window$`, w.drillCarriesOneWindow)
	sc.Step(`^every fretboard cell's fret is inside its window$`, w.everyCellInsideItsWindow)
	sc.Step(`^the "([^"]+)" (\w+) cells carry the fret window (\d+) to (\d+)$`, w.drillCarriesWindow)
	sc.Step(`^the session has no "([^"]+)" (\w+) cell at fret (\d+)$`, w.noDrillCellAtFret)
}

// stringCounts is how many strings each layout of the fret window
// scenarios has.
var stringCounts = map[string]int{"guitar": 6, "electric-bass": 4}

func (w *world) hasStartedHeadSessions(name string, count int) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	for i := range count {
		started := fixedNow.AddDate(0, 0, -count+i)
		w.practiceActivity.addSession(studentID, fakePracticeSession{startedAt: &started, lastEventAt: started})
	}
	return nil
}

// hasNothingDueOnGuitar sets nothing up: the student has no state on any
// guitar cell until a later step gives one.
func (w *world) hasNothingDueOnGuitar(string) error { return nil }

// hasNewCellsOnGuitar declares count new guitar cells, ten frets (0 to 9)
// per string from the lowest-pitched.
func (w *world) hasNewCellsOnGuitar(_ string, count int) error {
	for i := range count {
		w.putCell("guitar", 6-i/10, i%10)
	}
	return nil
}

// putCell declares the cell on str at fret of layout's own fretboard,
// classified under practiceSkill, once.
func (w *world) putCell(layout string, str, fret int) {
	id := instrumentID(layout).String()
	w.practiceItems.layouts[id] = id
	key := domain.FretboardCellItemKey(id, str, fret)
	if slices.ContainsFunc(w.practiceItems.cells[id], func(item domain.ClassifiedItem) bool { return item.ItemKey == key }) {
		return
	}
	w.practiceItems.cells[id] = append(w.practiceItems.cells[id], domain.ClassifiedItem{
		ItemKey: key, NodeIDs: []string{w.skillIDFor(practiceSkill).String()},
	})
}

// cellPlace is a string and fret.
type cellPlace struct{ str, fret int }

// spread lists count places of layout at frets from to to: every fret of
// its lowest-pitched string, then of the next, and so on.
func spread(layout string, count, from, to int) []cellPlace {
	var places []cellPlace
	for str := stringCounts[layout]; str >= 1 && len(places) < count; str-- {
		for fret := from; fret <= to && len(places) < count; fret++ {
			places = append(places, cellPlace{str: str, fret: fret})
		}
	}
	return places
}

// putCellsAskedAs declares each of places on layout and gives "alice", the
// fret window scenarios' student, state s on it, with right answers that
// ask it as drill next.
func (w *world) putCellsAskedAs(layout, drill string, s domain.PracticeItemState, places []cellPlace) error {
	switch domain.FretboardDrill(drill) {
	case domain.FretboardDrillNameTheNote:
		s.RightByResponse = nil
	case domain.FretboardDrillFindTheNote:
		s.RightByResponse = map[string]int{string(domain.FretboardDrillNameTheNote): 1}
	case domain.FretboardDrillFindEveryInstance:
		s.RightByResponse = map[string]int{string(domain.FretboardDrillNameTheNote): 1, string(domain.FretboardDrillFindTheNote): 1}
	default:
		return fmt.Errorf("unknown fretboard drill %q", drill)
	}
	for _, p := range places {
		w.putCell(layout, p.str, p.fret)
		w.putCellState("alice", layout, p.str, p.fret, s)
	}
	return nil
}

func dueState() domain.PracticeItemState {
	due := fixedNow.Add(-time.Hour)
	return domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: &due, LastAt: &fixedNow}
}

func weakState() domain.PracticeItemState {
	due := fixedNow.AddDate(0, 0, 2)
	return domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: &due, LastAt: &fixedNow}
}

// answeredState is a fluent cell not due for a month: neither due, weak
// nor reviewed ahead.
func answeredState() domain.PracticeItemState {
	due := fixedNow.AddDate(0, 1, 0)
	return domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: &due, LastAt: &fixedNow}
}

// weakCellsAcrossFrets declares count weak cells across frets from to to,
// and the third-lowest string's cells there new, so the session has new cells to
// name too.
func (w *world) weakCellsAcrossFrets(count int, layout string, from, to int, drill string) error {
	if err := w.putCellsAskedAs(layout, drill, weakState(), spread(layout, count, from, to)); err != nil {
		return err
	}
	for fret := from; fret <= to; fret++ {
		w.putCell(layout, stringCounts[layout]-2, fret)
	}
	return nil
}

func (w *world) weakCellsAtFrets(count int, layout string, from, to int, drill string) error {
	return w.putCellsAskedAs(layout, drill, weakState(), spread(layout, count, from, to))
}

func (w *world) dueCellsAtFrets(count int, layout string, from, to int, drill string) error {
	return w.putCellsAskedAs(layout, drill, dueState(), spread(layout, count, from, to))
}

func (w *world) dueCellsAtTwoFrets(count int, layout string, first, second int, drill string) error {
	if count != 2 {
		return fmt.Errorf("expected 2 cells at two frets, got %d", count)
	}
	return w.putCellsAskedAs(layout, drill, dueState(), []cellPlace{{str: stringCounts[layout], fret: first}, {str: stringCounts[layout], fret: second}})
}

func (w *world) dueCellsAndOneMore(count int, layout string, from, to, more, fret int, drill string) error {
	if more != 1 {
		return fmt.Errorf("expected 1 more due cell, got %d", more)
	}
	return w.putCellsAskedAs(layout, drill, dueState(), append(spread(layout, count, from, to), cellPlace{str: stringCounts[layout], fret: fret}))
}

// everyCellAnswered gives every cell of layout at frets from to to a state
// asking it as drill: due ones on the lowest-pitched strings, and the rest
// answered but not due.
func (w *world) everyCellAnswered(layout string, from, to, due int, drill string) error {
	places := spread(layout, stringCounts[layout]*(to-from+1), from, to)
	if err := w.putCellsAskedAs(layout, drill, dueState(), places[:due]); err != nil {
		return err
	}
	return w.putCellsAskedAs(layout, drill, answeredState(), places[due:])
}

func (w *world) cellsAreNew(layout string, from, to int) error {
	for _, p := range spread(layout, stringCounts[layout]*(to-from+1), from, to) {
		w.putCell(layout, p.str, p.fret)
	}
	return nil
}

// drillCells lists the planned cells of layout asked as drill.
func (w *world) drillCells(layout, drill string) ([]generated.PracticeSessionItem, error) {
	cells, err := w.plannedCells()
	if err != nil {
		return nil, err
	}
	var out []generated.PracticeSessionItem
	for _, item := range cells {
		if item.FretboardCell.LayoutInstrumentId == instrumentID(layout) && string(item.FretboardCell.Drill) == drill {
			out = append(out, item)
		}
	}
	return out, nil
}

func (w *world) sessionHasCellsAskedBoth(layout, first, second string) error {
	for _, drill := range []string{first, second} {
		cells, err := w.drillCells(layout, drill)
		if err != nil {
			return err
		}
		if len(cells) == 0 {
			return fmt.Errorf("expected %q cells asked as %s in the session", layout, drill)
		}
	}
	return nil
}

// drillWindows lists the fret windows, by their first fret, the planned
// cells of layout asked as drill carry, each once. A drill with no cell in
// the session fails.
func (w *world) drillWindows(layout, drill string) ([]int, error) {
	cells, err := w.drillCells(layout, drill)
	if err != nil {
		return nil, err
	}
	if len(cells) == 0 {
		return nil, fmt.Errorf("expected %q cells asked as %s in the session", layout, drill)
	}
	var froms []int
	for _, item := range cells {
		if from := int(item.FretboardCell.FretWindow.FromFret); !slices.Contains(froms, from) {
			froms = append(froms, from)
		}
	}
	return froms, nil
}

func (w *world) drillCarriesOneWindow(layout, drill string) error {
	froms, err := w.drillWindows(layout, drill)
	if err != nil {
		return err
	}
	if len(froms) != 1 {
		return fmt.Errorf("expected one window for %q %s, got windows from frets %v", layout, drill, froms)
	}
	return nil
}

// windowToFret is the highest fret of the window from from.
func windowToFret(from int) int {
	return domain.FretWindow{FromFret: from}.ToFret()
}

func (w *world) everyCellInsideItsWindow() error {
	cells, err := w.plannedCells()
	if err != nil {
		return err
	}
	if len(cells) == 0 {
		return fmt.Errorf("expected fretboard cells in the session")
	}
	for _, item := range cells {
		c := item.FretboardCell
		from := int(c.FretWindow.FromFret)
		if c.Fret < from || c.Fret > windowToFret(from) {
			return fmt.Errorf("expected %s inside its window %d to %d", item.ItemKey, from, windowToFret(from))
		}
	}
	return nil
}

func (w *world) drillCarriesWindow(layout, drill string, from, to int) error {
	if windowToFret(from) != to {
		return fmt.Errorf("no fret window runs from %d to %d", from, to)
	}
	froms, err := w.drillWindows(layout, drill)
	if err != nil {
		return err
	}
	if !slices.Equal(froms, []int{from}) {
		return fmt.Errorf("expected the %q %s cells in the window %d to %d, got windows from frets %v", layout, drill, from, to, froms)
	}
	return w.everyCellInsideItsWindow()
}

func (w *world) noDrillCellAtFret(layout, drill string, fret int) error {
	cells, err := w.drillCells(layout, drill)
	if err != nil {
		return err
	}
	if len(cells) == 0 {
		return fmt.Errorf("expected %q cells asked as %s in the session", layout, drill)
	}
	for _, item := range cells {
		if item.FretboardCell.Fret == fret {
			return fmt.Errorf("expected no %q %s cell at fret %d, got %s", layout, drill, fret, item.ItemKey)
		}
	}
	return nil
}
