//go:build integration

package bdd

import (
	"fmt"
	"strconv"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerDiagramSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a diagram "([^"]+)" exists on instrument "([^"]+)"$`, w.aDiagramExistsOn)
	sc.Step(`^a diagram "([^"]+)" exists on instrument "([^"]+)" with positions:$`, w.aDiagramExistsOnWithPositions)

	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" classified under skills "([^"]+)", concepts "([^"]+)" with fretted positions:$`, w.createsDiagramWithFrettedPositions)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" classified under skills "([^"]+)", concepts "([^"]+)" with keyboard positions:$`, w.createsDiagramWithKeyboardPositions)
	sc.Step(`^"([^"]+)" submits a create diagram request on instrument "([^"]+)" with the skill_ids field omitted$`, w.submitsDiagramWithoutSkills)
	sc.Step(`^"([^"]+)" submits a create diagram request with an instrument id that does not exist$`, w.submitsDiagramOnMissingInstrument)
	sc.Step(`^"([^"]+)" attempts to create a diagram$`, w.attemptsCreateDiagram)
	sc.Step(`^an unauthenticated request attempts to create a diagram$`, w.unauthCreatesDiagram)

	sc.Step(`^the diagram is created and assigned a stable identifier$`, w.diagramCreated)
	sc.Step(`^the diagram has (\d+) positions$`, w.diagramHasPositions)

	sc.Step(`^"([^"]+)" lists diagrams filtered by instrument "([^"]+)"$`, w.listsDiagramsByInstrument)
	sc.Step(`^"([^"]+)" lists all diagrams$`, w.listsAllDiagrams)
	sc.Step(`^"([^"]+)" retrieves a diagram with an ID that does not exist$`, w.retrievesMissingDiagram)
}

// ensureInstrumentSeeded returns the instrument named instrumentName,
// seeding it first if a preceding step (e.g. diagrams.feature's own
// Background) has not already — "piano" as keyboard, everything else as
// fretted, matching this codebase's own instrument-naming convention. This
// lets a scenario outside diagrams.feature that only cares about a diagram
// existing, not the instrument-management story, skip declaring the
// instrument fixture itself.
func (w *world) ensureInstrumentSeeded(instrumentName string) (domain.Instrument, error) {
	instrument, err := w.instruments.GetByID(w.ctx(), instrumentID(instrumentName).String())
	if err == nil {
		return instrument, nil
	}
	if instrumentName == "piano" {
		if seedErr := w.aKeyboardInstrumentExists(instrumentName); seedErr != nil {
			return domain.Instrument{}, seedErr
		}
	} else if seedErr := w.aFrettedInstrumentExists(instrumentName); seedErr != nil {
		return domain.Instrument{}, seedErr
	}
	instrument, err = w.instruments.GetByID(w.ctx(), instrumentID(instrumentName).String())
	if err != nil {
		return domain.Instrument{}, fmt.Errorf("seeding instrument %q: %w", instrumentName, err)
	}
	return instrument, nil
}

// aDiagramExistsOn seeds a one-position diagram directly, shaped to match
// whichever family the named instrument has.
func (w *world) aDiagramExistsOn(slug, instrumentName string) error {
	instrument, err := w.ensureInstrumentSeeded(instrumentName)
	if err != nil {
		return err
	}
	position := domain.Position{ID: deterministicUUID("position", slug).String(), Interval: "R", NoteName: "A"}
	if instrument.Family == domain.InstrumentFamilyFretted {
		str, fret := 6, 5
		position.String, position.Fret = &str, &fret
	} else {
		key := "A3"
		position.Key = &key
	}
	skillID, conceptID := w.skillIDFor("seeded-skill"), w.conceptIDFor("seeded-concept")
	diagram, err := domain.NewDiagram(diagramID(slug).String(), instrument, slug, []domain.Position{position},
		[]string{skillID.String()}, []string{conceptID.String()}, fixedNow)
	if err != nil {
		return fmt.Errorf("seeding diagram %q: %w", slug, err)
	}
	w.diagrams.put(diagram)
	return nil
}

// aDiagramExistsOnWithPositions is aDiagramExistsOn's table-driven
// counterpart, for scenarios that need more than one position — a diagram-
// driven exercise's derived options depend on there being several to
// derive from.
func (w *world) aDiagramExistsOnWithPositions(slug, instrumentName string, table *godog.Table) error {
	instrument, err := w.ensureInstrumentSeeded(instrumentName)
	if err != nil {
		return err
	}
	positions := make([]domain.Position, 0, len(table.Rows)-1)
	for row := 1; row < len(table.Rows); row++ {
		interval, err := cell(table, row, "interval")
		if err != nil {
			return err
		}
		note, err := cell(table, row, "note_name")
		if err != nil {
			return err
		}
		stringCell, err := cell(table, row, "string")
		if err != nil {
			return err
		}
		fretCell, err := cell(table, row, "fret")
		if err != nil {
			return err
		}
		str, err := strconv.Atoi(stringCell)
		if err != nil {
			return fmt.Errorf("string %q is not a number: %w", stringCell, err)
		}
		fret, err := strconv.Atoi(fretCell)
		if err != nil {
			return fmt.Errorf("fret %q is not a number: %w", fretCell, err)
		}
		positions = append(positions, domain.Position{
			ID: deterministicUUID("position", fmt.Sprintf("%s-%d", slug, row)).String(),
			Interval: interval, NoteName: note, String: &str, Fret: &fret,
		})
	}
	skillID, conceptID := w.skillIDFor("seeded-skill"), w.conceptIDFor("seeded-concept")
	diagram, err := domain.NewDiagram(diagramID(slug).String(), instrument, slug, positions,
		[]string{skillID.String()}, []string{conceptID.String()}, fixedNow)
	if err != nil {
		return fmt.Errorf("seeding diagram %q: %w", slug, err)
	}
	w.diagrams.put(diagram)
	return nil
}

func (w *world) createDiagram(body generated.CreateDiagramRequest) error {
	resp, err := w.handler.CreateDiagram(w.ctx(), generated.CreateDiagramRequestObject{Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) diagramClassification(skills, concepts string) generated.DiagramClassificationInput {
	return generated.DiagramClassificationInput{SkillIds: w.skillIDsFor(skills), ConceptIds: w.conceptIDsFor(concepts)}
}

// cell returns the value under column in a Gherkin table row, by header name.
func cell(table *godog.Table, row int, column string) (string, error) {
	for i, c := range table.Rows[0].Cells {
		if c.Value == column {
			return table.Rows[row].Cells[i].Value, nil
		}
	}
	return "", fmt.Errorf("table has no %q column", column)
}

func (w *world) createsDiagramWithFrettedPositions(_, name, instrument, skills, concepts string, table *godog.Table) error {
	positions := make([]generated.DiagramPosition, 0, len(table.Rows)-1)
	for row := 1; row < len(table.Rows); row++ {
		interval, err := cell(table, row, "interval")
		if err != nil {
			return err
		}
		note, err := cell(table, row, "note_name")
		if err != nil {
			return err
		}
		stringCell, err := cell(table, row, "string")
		if err != nil {
			return err
		}
		fretCell, err := cell(table, row, "fret")
		if err != nil {
			return err
		}
		str, err := strconv.Atoi(stringCell)
		if err != nil {
			return fmt.Errorf("string %q is not a number: %w", stringCell, err)
		}
		fret, err := strconv.Atoi(fretCell)
		if err != nil {
			return fmt.Errorf("fret %q is not a number: %w", fretCell, err)
		}
		positions = append(positions, generated.DiagramPosition{Interval: interval, NoteName: note, String: &str, Fret: &fret})
	}
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID(instrument), Name: name, Positions: positions,
		Classification: w.diagramClassification(skills, concepts),
	})
}

func (w *world) createsDiagramWithKeyboardPositions(_, name, instrument, skills, concepts string, table *godog.Table) error {
	positions := make([]generated.DiagramPosition, 0, len(table.Rows)-1)
	for row := 1; row < len(table.Rows); row++ {
		interval, err := cell(table, row, "interval")
		if err != nil {
			return err
		}
		note, err := cell(table, row, "note_name")
		if err != nil {
			return err
		}
		key, err := cell(table, row, "key")
		if err != nil {
			return err
		}
		positions = append(positions, generated.DiagramPosition{Interval: interval, NoteName: note, Key: &key})
	}
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID(instrument), Name: name, Positions: positions,
		Classification: w.diagramClassification(skills, concepts),
	})
}

func onePositionOnGuitar() []generated.DiagramPosition {
	str, fret := 6, 5
	return []generated.DiagramPosition{{Interval: "R", NoteName: "A", String: &str, Fret: &fret}}
}

func (w *world) submitsDiagramWithoutSkills(_, instrument string) error {
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID(instrument), Name: "No skills", Positions: onePositionOnGuitar(),
		Classification: generated.DiagramClassificationInput{ConceptIds: w.conceptIDsFor("scale-construction")},
	})
}

func (w *world) submitsDiagramOnMissingInstrument(string) error {
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: deterministicUUID("instrument", "does-not-exist"), Name: "Orphan", Positions: onePositionOnGuitar(),
		Classification: w.diagramClassification("minor-pentatonic-scale", "scale-construction"),
	})
}

func (w *world) attemptsCreateDiagram(string) error {
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID("guitar"), Name: "Attempted", Positions: onePositionOnGuitar(),
		Classification: w.diagramClassification("minor-pentatonic-scale", "scale-construction"),
	})
}

func (w *world) unauthCreatesDiagram() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateDiagram("")
}

func (w *world) diagramCreated() error {
	resp, ok := w.lastResp.(generated.CreateDiagram201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.DiagramId == uuid.Nil {
		return fmt.Errorf("expected a diagram_id in the response")
	}
	return nil
}

func (w *world) diagramHasPositions(count string) error {
	want, err := strconv.Atoi(count)
	if err != nil {
		return fmt.Errorf("position count %q is not a number: %w", count, err)
	}
	resp, ok := w.lastResp.(generated.CreateDiagram201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(resp.Positions) != want {
		return fmt.Errorf("expected %d positions, got %d", want, len(resp.Positions))
	}
	for i, p := range resp.Positions {
		if p.PositionId == nil {
			return fmt.Errorf("position %d has no position_id", i)
		}
	}
	return nil
}

func (w *world) listsDiagramsByInstrument(_, instrument string) error {
	id := instrumentID(instrument)
	resp, err := w.handler.ListDiagrams(w.ctx(), generated.ListDiagramsRequestObject{
		Params: generated.ListDiagramsParams{InstrumentId: &id},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsAllDiagrams(string) error {
	resp, err := w.handler.ListDiagrams(w.ctx(), generated.ListDiagramsRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesMissingDiagram(string) error {
	resp, err := w.handler.GetDiagram(w.ctx(), generated.GetDiagramRequestObject{DiagramId: diagramID("does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}
