//go:build integration

package bdd

import (
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// lastCell names a fretboard cell a student answered.
type lastCell struct {
	student, layout string
	str, fret       int
}

func registerFretboardMapSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^student "([^"]+)" plays "([^"]+)"$`, w.studentPlaysOne)

	sc.Step(`^"([^"]+)" has made the "([^"]+)" cell on string (\d+), fret (\d+) accurate$`, w.madeCellAccurate)
	sc.Step(`^"([^"]+)" has practised no fretboard cell$`, w.practisedNoCell)
	sc.Step(`^"([^"]+)" has named the note of the "([^"]+)" cell on string (\d+), fret (\d+) correctly on 2 different days$`, w.namedCellOnTwoDays)
	sc.Step(`^found it correctly on a third day$`, w.foundCellOnThirdDay)
	sc.Step(`^"([^"]+)"'s "([^"]+)" cell on string (\d+), fret (\d+) is fluent and its review was due yesterday$`, w.cellFluentDueYesterday)
	sc.Step(`^"([^"]+)"'s "([^"]+)" cell on string (\d+), fret (\d+) is fluent, in box (\d+), and its review is (\d+) days overdue$`, w.cellFluentOverdue)

	sc.Step(`^"([^"]+)" reads their fretboard map for "([^"]+)"$`, w.readsFretboardMap)
	sc.Step(`^"([^"]+)" reads their fretboard map for an instrument that doesn't exist$`, w.readsFretboardMapForMissingInstrument)

	sc.Step(`^the cell on string (\d+), fret (\d+) is ([a-z]+)$`, w.mapCellIs)
	sc.Step(`^the cell on string (\d+), fret (\d+) is ([a-z]+) and fading$`, w.mapCellIsFading)
	sc.Step(`^the map has (\d+) cells, on strings (\d+) to (\d+) at frets (\d+) to (\d+)$`, w.mapHasCells)
	sc.Step(`^every cell is new$`, w.everyMapCellIsNew)
	sc.Step(`^the map has no cells$`, w.mapHasNoCells)
}

func (w *world) studentPlaysOne(name, instrument string) error {
	w.authenticateAs(name, domain.RoleStudent)
	_, err := w.ensureCatalogInstrument(instrument)
	return err
}

// ensureCatalogInstrument seeds a catalog fretted instrument with its real
// strings and tuning, or any other instrument as the diagram steps do.
func (w *world) ensureCatalogInstrument(name string) (domain.Instrument, error) {
	if _, ok := catalogTunings[name]; ok {
		return w.seedCatalogInstrument(name)
	}
	return w.ensureInstrumentSeeded(name)
}

// putCellState gives the student a state on a cell of the named layout
// instrument. Answers by naming and by finding the note are folded into this
// one state by the Aggregation Worker; this service only reads it.
func (w *world) putCellState(name, layout string, str, fret int, state domain.PracticeItemState) {
	state.ItemKey = domain.FretboardCellItemKey(instrumentID(layout).String(), str, fret)
	state.RulesVersion = domain.PracticeRulesVersion
	w.practiceStates.put(w.ensureRegistered(name, domain.RoleStudent).String(), state)
}

func (w *world) madeCellAccurate(name, layout string, str, fret int) error {
	due := fixedNow.AddDate(0, 0, 3)
	w.putCellState(name, layout, str, fret, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: &due, LastAt: &fixedNow})
	return nil
}

func (w *world) practisedNoCell(string) error { return nil }

// namedCellOnTwoDays is the state after two right answers on two days:
// still learning. The cell is remembered for the next step's third answer.
func (w *world) namedCellOnTwoDays(name, layout string, str, fret int) error {
	due := fixedNow.AddDate(0, 0, 2)
	w.putCellState(name, layout, str, fret, domain.PracticeItemState{Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 2, DueAt: &due, LastAt: &fixedNow})
	w.lastCell = &lastCell{student: name, layout: layout, str: str, fret: fret}
	return nil
}

// foundCellOnThirdDay is the same cell's state once a third right answer,
// by finding the note, makes it accurate.
func (w *world) foundCellOnThirdDay() error {
	if w.lastCell == nil {
		return fmt.Errorf("no cell was named before")
	}
	c := *w.lastCell
	due := fixedNow.AddDate(0, 0, 4)
	w.putCellState(c.student, c.layout, c.str, c.fret, domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 3, DueAt: &due, LastAt: &fixedNow})
	return nil
}

func (w *world) cellFluentDueYesterday(name, layout string, str, fret int) error {
	due := fixedNow.Add(-24 * time.Hour)
	w.putCellState(name, layout, str, fret, domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: 4, DueAt: &due, LastAt: &due})
	return nil
}

func (w *world) cellFluentOverdue(name, layout string, str, fret, box, days int) error {
	due := fixedNow.AddDate(0, 0, -days)
	w.putCellState(name, layout, str, fret, domain.PracticeItemState{Level: domain.KnowledgeLevelFluent, Counted: 6, Box: box, DueAt: &due, LastAt: &due})
	return nil
}

// readsFretboardMap seeds the instrument and lets it share the installed
// cells of every layout of the same geometry, as the Postgres source does.
func (w *world) readsFretboardMap(_, instrument string) error {
	played, err := w.ensureCatalogInstrument(instrument)
	if err != nil {
		return err
	}
	for layoutID := range w.practiceItems.cells {
		layout, err := w.instruments.GetByID(w.ctx(), layoutID)
		if err != nil {
			return err
		}
		if played.SameGeometry(layout) {
			w.practiceItems.layouts[played.ID] = layoutID
		}
	}
	return w.readFretboardMap(openapi_types.UUID(instrumentID(instrument)))
}

func (w *world) readsFretboardMapForMissingInstrument(string) error {
	return w.readFretboardMap(openapi_types.UUID(uuid.New()))
}

func (w *world) readFretboardMap(id openapi_types.UUID) error {
	resp, err := w.handler.GetFretboardMap(w.ctx(), generated.GetFretboardMapRequestObject{
		Params: generated.GetFretboardMapParams{InstrumentId: id},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) fretboardMap() (generated.FretboardMap, error) {
	resp, ok := w.lastResp.(generated.GetFretboardMap200JSONResponse)
	if !ok {
		return generated.FretboardMap{}, fmt.Errorf("expected a fretboard map, got %T", w.lastResp)
	}
	return generated.FretboardMap(resp), nil
}

func (w *world) mapCell(str, fret int) (generated.FretboardMapCell, error) {
	m, err := w.fretboardMap()
	if err != nil {
		return generated.FretboardMapCell{}, err
	}
	for _, c := range m.Cells {
		if c.String == str && c.Fret == fret {
			return c, nil
		}
	}
	return generated.FretboardMapCell{}, fmt.Errorf("the map has no cell on string %d, fret %d", str, fret)
}

func (w *world) mapCellIs(str, fret int, level string) error {
	c, err := w.mapCell(str, fret)
	if err != nil {
		return err
	}
	if string(c.Level) != level {
		return fmt.Errorf("the cell on string %d, fret %d is %s, want %s", str, fret, c.Level, level)
	}
	return nil
}

func (w *world) mapCellIsFading(str, fret int, level string) error {
	if err := w.mapCellIs(str, fret, level); err != nil {
		return err
	}
	c, err := w.mapCell(str, fret)
	if err != nil {
		return err
	}
	if !c.Fading {
		return fmt.Errorf("the cell on string %d, fret %d is not fading", str, fret)
	}
	return nil
}

func (w *world) mapHasCells(count, fromString, toString, fromFret, toFret int) error {
	m, err := w.fretboardMap()
	if err != nil {
		return err
	}
	if len(m.Cells) != count {
		return fmt.Errorf("the map has %d cells, want %d", len(m.Cells), count)
	}
	i := 0
	for str := fromString; str <= toString; str++ {
		for fret := fromFret; fret <= toFret; fret++ {
			if i >= len(m.Cells) || m.Cells[i].String != str || m.Cells[i].Fret != fret {
				return fmt.Errorf("cell %d is not string %d, fret %d: the map is not every cell by string then fret", i, str, fret)
			}
			i++
		}
	}
	if i != count {
		return fmt.Errorf("strings %d to %d at frets %d to %d are %d cells, not %d", fromString, toString, fromFret, toFret, i, count)
	}
	return nil
}

func (w *world) everyMapCellIsNew() error {
	m, err := w.fretboardMap()
	if err != nil {
		return err
	}
	for _, c := range m.Cells {
		if c.Level != generated.KnowledgeLevelNew || c.Fading {
			return fmt.Errorf("the cell on string %d, fret %d is %s (fading %t), want new", c.String, c.Fret, c.Level, c.Fading)
		}
	}
	return nil
}

func (w *world) mapHasNoCells() error {
	m, err := w.fretboardMap()
	if err != nil {
		return err
	}
	if len(m.Cells) != 0 || m.LayoutInstrumentId != nil {
		return fmt.Errorf("the map has %d cells and layout %v, want none", len(m.Cells), m.LayoutInstrumentId)
	}
	return nil
}
