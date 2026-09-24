//go:build integration

package bdd

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerDiagramSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a diagram "([^"]+)" exists on instrument "([^"]+)"$`, w.aDiagramExistsOn)
	sc.Step(`^a basic diagram "([^"]+)" exists on instrument "([^"]+)"$`, w.aDiagramExistsOn)
	sc.Step(`^a custom diagram "([^"]+)" exists on instrument "([^"]+)", created by "([^"]+)"$`, w.aCustomDiagramExistsOn)
	sc.Step(`^(\d+) basic diagrams exist on instrument "([^"]+)"$`, w.bulkBasicDiagrams)
	sc.Step(`^a diagram "([^"]+)" exists on instrument "([^"]+)" with positions:$`, w.aDiagramExistsOnWithPositions)

	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" classified under skills "([^"]+)", concepts "([^"]+)" with fretted positions:$`, w.createsDiagramWithFrettedPositions)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" classified under skills "([^"]+)", concepts "([^"]+)" with keyboard positions:$`, w.createsDiagramWithKeyboardPositions)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" with root note "([^"]+)", label display "([^"]+)" classified under skills "([^"]+)", concepts "([^"]+)" with fretted positions:$`, w.createsDiagramWithRootAndLabelDisplay)
	sc.Step(`^"([^"]+)" creates a basic diagram named "([^"]+)" on instrument "([^"]+)" classified under skills "([^"]+)", concepts "([^"]+)" with fretted positions:$`, w.createsBasicDiagramWithFrettedPositions)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" with kind "([^"]*)" classified under skills "([^"]+)", concepts "([^"]+)" with fretted positions:$`, w.createsDiagramWithKind)
	sc.Step(`^"([^"]+)" attempts to create a basic diagram$`, w.attemptsCreateBasicDiagram)
	sc.Step(`^"([^"]+)" saves a copy of diagram "([^"]+)" named "([^"]+)"$`, w.savesCopyOfDiagram)
	sc.Step(`^"([^"]+)" saves a copy of diagram "([^"]+)" as a basic diagram named "([^"]+)"$`, w.savesCopyOfDiagramAsBasic)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" with color "([^"]*)" classified under skills "([^"]+)", concepts "([^"]+)" with fretted positions:$`, w.createsDiagramWithColor)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" setting color "([^"]+)" and position (\d+) color "([^"]+)"$`, w.updatesDiagramColors)
	sc.Step(`^"([^"]+)" submits a create diagram request on instrument "([^"]+)" with the skill_ids field omitted$`, w.submitsDiagramWithoutSkills)
	sc.Step(`^"([^"]+)" submits a create diagram request with an instrument id that does not exist$`, w.submitsDiagramOnMissingInstrument)
	sc.Step(`^"([^"]+)" attempts to create a diagram$`, w.attemptsCreateDiagram)
	sc.Step(`^an unauthenticated request attempts to create a diagram$`, w.unauthCreatesDiagram)
	sc.Step(`^"([^"]+)" updates diagram "([^"]+)" setting root note "([^"]+)" and label display "([^"]+)"$`, w.updatesDiagramRootAndLabelDisplay)

	sc.Step(`^the diagram is created and assigned a stable identifier$`, w.diagramCreated)
	sc.Step(`^the diagram has (\d+) positions$`, w.diagramHasPositions)
	sc.Step(`^the diagram's root note is "([^"]+)"$`, w.diagramRootNoteIs)
	sc.Step(`^the diagram has no recorded root note$`, w.diagramHasNoRootNote)
	sc.Step(`^the diagram's label display is "([^"]+)"$`, w.diagramLabelDisplayIs)
	sc.Step(`^position (\d+) has shape "([^"]+)"$`, w.positionHasShape)
	sc.Step(`^the diagram's color is "([^"]+)"$`, w.diagramColorIs)
	sc.Step(`^the diagram has no general color$`, w.diagramHasNoColor)
	sc.Step(`^position (\d+) has color "([^"]+)"$`, w.positionHasColor)
	sc.Step(`^position (\d+) has no color of its own$`, w.positionHasNoColor)
	sc.Step(`^the (?:new )?diagram's kind is "([^"]+)"$`, w.diagramKindIs)
	sc.Step(`^the (?:new )?diagram records "([^"]+)" as the creator$`, w.diagramRecordsCreator)
	sc.Step(`^a new diagram is created with the same positions as "([^"]+)"$`, w.newDiagramCopiesPositionsOf)
	sc.Step(`^diagram "([^"]+)" is unchanged$`, w.diagramIsUnchanged)
	sc.Step(`^the response is diagram "([^"]+)"$`, w.responseIsDiagram)
	sc.Step(`^the response includes "([^"]+)", "([^"]+)" and "([^"]+)"$`, w.responseIncludesThree)

	sc.Step(`^"([^"]+)" lists diagrams filtered by instrument "([^"]+)"$`, w.listsDiagramsByInstrument)
	sc.Step(`^"([^"]+)" lists all diagrams$`, w.listsAllDiagrams)
	sc.Step(`^"([^"]+)" lists diagrams of kind "([^"]+)"$`, w.listsDiagramsOfKind)
	sc.Step(`^"([^"]+)" lists diagrams filtered by creator "([^"]+)"$`, w.listsDiagramsByCreator)
	sc.Step(`^"([^"]+)" lists diagrams with (.+)$`, w.listsDiagramsPaged)
	sc.Step(`^"([^"]+)" retrieves diagram "([^"]+)"$`, w.retrievesDiagram)
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

// curatorID is the owner of every basic diagram a step seeds: the admin
// named "admin", registered on first use.
func (w *world) curatorID() string {
	return w.ensureRegistered("admin", domain.RoleAdmin).String()
}

// aDiagramExistsOn seeds a one-position basic diagram directly, owned by
// the curator admin and shaped to match whichever family the named
// instrument has. A diagram whose kind a scenario doesn't state is basic:
// that's what every teacher can find and use.
func (w *world) aDiagramExistsOn(slug, instrumentName string) error {
	return w.seedOnePositionDiagram(slug, slug, instrumentName, domain.DiagramKindBasic, w.curatorID())
}

// aCustomDiagramExistsOn seeds a one-position custom diagram owned by the
// teacher named creator.
func (w *world) aCustomDiagramExistsOn(slug, instrumentName, creator string) error {
	owner := w.ensureRegistered(creator, domain.RoleTeacher).String()
	return w.seedOnePositionDiagram(slug, slug, instrumentName, domain.DiagramKindCustom, owner)
}

// bulkBasicDiagrams seeds count basic diagrams with zero-padded names, so
// name order and seeding order agree.
func (w *world) bulkBasicDiagrams(count int, instrumentName string) error {
	curator := w.curatorID()
	for i := 1; i <= count; i++ {
		name := padded("bulk-diagram-", i, 3)
		if err := w.seedOnePositionDiagram(name, name, instrumentName, domain.DiagramKindBasic, curator); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) seedOnePositionDiagram(slug, name, instrumentName string, kind domain.DiagramKind, owner string) error {
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
	diagram, err := domain.NewDiagram(diagramID(slug).String(), owner, instrument, name, []domain.Position{position}, []string{skillID.String()}, []string{conceptID.String()}, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval, Kind: kind}, fixedNow)
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
	diagram, err := domain.NewDiagram(diagramID(slug).String(), w.curatorID(), instrument, slug, positions, []string{skillID.String()}, []string{conceptID.String()}, domain.DiagramOptions{LabelDisplay: domain.LabelDisplayInterval, Kind: domain.DiagramKindBasic}, fixedNow)
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
		position := generated.DiagramPosition{Interval: interval, NoteName: note, String: &str, Fret: &fret}
		if colorCell := optionalCell(table, row, "color"); colorCell != "" {
			position.Color = &colorCell
		}
		positions = append(positions, position)
	}
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID(instrument), Name: name, Positions: positions,
		Classification: w.diagramClassification(skills, concepts),
	})
}

// createsDiagramWithColor is createsDiagramWithFrettedPositions plus a
// general marker color; per-position colors come from the table's optional
// color column, which the plain step honors too.
func (w *world) createsDiagramWithColor(_, name, instrument, color, skills, concepts string, table *godog.Table) error {
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
		str, fret, err := frettedCoordinates(table, row)
		if err != nil {
			return err
		}
		position := generated.DiagramPosition{Interval: interval, NoteName: note, String: &str, Fret: &fret}
		if colorCell := optionalCell(table, row, "color"); colorCell != "" {
			position.Color = &colorCell
		}
		positions = append(positions, position)
	}
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID(instrument), Name: name, Positions: positions, Color: &color,
		Classification: w.diagramClassification(skills, concepts),
	})
}

func frettedCoordinates(table *godog.Table, row int) (str, fret int, err error) {
	stringCell, err := cell(table, row, "string")
	if err != nil {
		return 0, 0, err
	}
	fretCell, err := cell(table, row, "fret")
	if err != nil {
		return 0, 0, err
	}
	if str, err = strconv.Atoi(stringCell); err != nil {
		return 0, 0, fmt.Errorf("string %q is not a number: %w", stringCell, err)
	}
	if fret, err = strconv.Atoi(fretCell); err != nil {
		return 0, 0, fmt.Errorf("fret %q is not a number: %w", fretCell, err)
	}
	return str, fret, nil
}

// updatesDiagramColors sets the general color and one position's color,
// resending the diagram's current positions since an update replaces the
// whole set.
func (w *world) updatesDiagramColors(_, slug, color, index, positionColor string) error {
	i, err := strconv.Atoi(index)
	if err != nil {
		return fmt.Errorf("position index %q is not a number: %w", index, err)
	}
	current, err := w.diagrams.GetByID(w.ctx(), diagramID(slug).String())
	if err != nil {
		return fmt.Errorf("loading diagram %q: %w", slug, err)
	}
	if i < 1 || i > len(current.Positions) {
		return fmt.Errorf("no position %d (diagram has %d)", i, len(current.Positions))
	}
	positions := make([]generated.DiagramPosition, len(current.Positions))
	for j, p := range current.Positions {
		id := uuid.MustParse(p.ID)
		positions[j] = generated.DiagramPosition{PositionId: &id, Interval: p.Interval, NoteName: p.NoteName, String: p.String, Fret: p.Fret, Key: p.Key}
	}
	positions[i-1].Color = &positionColor
	body := generated.UpdateDiagramRequest{Color: &color, Positions: &positions}
	resp, err := w.handler.UpdateDiagram(w.ctx(), generated.UpdateDiagramRequestObject{DiagramId: diagramID(slug), Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

// optionalCell is cell's counterpart for a column a table may or may not
// carry, or may leave blank on a given row — both mean "not supplied".
func optionalCell(table *godog.Table, row int, column string) string {
	for i, c := range table.Rows[0].Cells {
		if c.Value == column {
			return table.Rows[row].Cells[i].Value
		}
	}
	return ""
}

func (w *world) createsDiagramWithRootAndLabelDisplay(_, name, instrument, rootNote, labelDisplay, skills, concepts string, table *godog.Table) error {
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
		position := generated.DiagramPosition{Interval: interval, NoteName: note, String: &str, Fret: &fret}
		if shapeCell := optionalCell(table, row, "shape"); shapeCell != "" {
			shape := generated.DiagramPositionShape(shapeCell)
			position.Shape = &shape
		}
		positions = append(positions, position)
	}
	display := generated.CreateDiagramRequestLabelDisplay(labelDisplay)
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID(instrument), Name: name, Positions: positions,
		RootNote: &rootNote, LabelDisplay: &display,
		Classification: w.diagramClassification(skills, concepts),
	})
}

func (w *world) updatesDiagramRootAndLabelDisplay(_, slug, rootNote, labelDisplay string) error {
	display := generated.UpdateDiagramRequestLabelDisplay(labelDisplay)
	body := generated.UpdateDiagramRequest{RootNote: &rootNote, LabelDisplay: &display}
	resp, err := w.handler.UpdateDiagram(w.ctx(), generated.UpdateDiagramRequestObject{
		DiagramId: diagramID(slug), Body: &body,
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

// currentDiagram returns the Diagram carried by whichever create/update/get
// response the last step produced, regardless of which one it was — every
// Then step about a diagram's own fields (root note, label display, a
// position's shape) needs to read it the same way after any of them.
func (w *world) currentDiagram() (generated.Diagram, error) {
	switch resp := w.lastResp.(type) {
	case generated.CreateDiagram201JSONResponse:
		return generated.Diagram(resp), nil
	case generated.UpdateDiagram200JSONResponse:
		return generated.Diagram(resp), nil
	case generated.GetDiagram200JSONResponse:
		return generated.Diagram(resp), nil
	default:
		return generated.Diagram{}, fmt.Errorf("expected a diagram response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) diagramRootNoteIs(want string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.RootNote == nil {
		return fmt.Errorf("expected root_note %q, got none recorded", want)
	}
	if *diagram.RootNote != want {
		return fmt.Errorf("expected root_note %q, got %q", want, *diagram.RootNote)
	}
	return nil
}

func (w *world) diagramHasNoRootNote() error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.RootNote != nil {
		return fmt.Errorf("expected no recorded root_note, got %q", *diagram.RootNote)
	}
	return nil
}

func (w *world) diagramLabelDisplayIs(want string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if string(diagram.LabelDisplay) != want {
		return fmt.Errorf("expected label_display %q, got %q", want, diagram.LabelDisplay)
	}
	return nil
}

func (w *world) diagramColorIs(want string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.Color == nil || *diagram.Color != want {
		return fmt.Errorf("expected color %q, got %v", want, diagram.Color)
	}
	return nil
}

func (w *world) diagramHasNoColor() error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if diagram.Color != nil {
		return fmt.Errorf("expected no general color, got %q", *diagram.Color)
	}
	return nil
}

func (w *world) positionAt(index string) (generated.DiagramPosition, error) {
	i, err := strconv.Atoi(index)
	if err != nil {
		return generated.DiagramPosition{}, fmt.Errorf("position index %q is not a number: %w", index, err)
	}
	diagram, err := w.currentDiagram()
	if err != nil {
		return generated.DiagramPosition{}, err
	}
	if i < 1 || i > len(diagram.Positions) {
		return generated.DiagramPosition{}, fmt.Errorf("no position %d (diagram has %d)", i, len(diagram.Positions))
	}
	return diagram.Positions[i-1], nil
}

func (w *world) positionHasColor(index, want string) error {
	p, err := w.positionAt(index)
	if err != nil {
		return err
	}
	if p.Color == nil || *p.Color != want {
		return fmt.Errorf("expected position %s to have color %q, got %v", index, want, p.Color)
	}
	return nil
}

func (w *world) positionHasNoColor(index string) error {
	p, err := w.positionAt(index)
	if err != nil {
		return err
	}
	if p.Color != nil {
		return fmt.Errorf("expected position %s to have no color, got %q", index, *p.Color)
	}
	return nil
}

func (w *world) positionHasShape(index, want string) error {
	i, err := strconv.Atoi(index)
	if err != nil {
		return fmt.Errorf("position index %q is not a number: %w", index, err)
	}
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if i < 1 || i > len(diagram.Positions) {
		return fmt.Errorf("no position %d (diagram has %d)", i, len(diagram.Positions))
	}
	shape := diagram.Positions[i-1].Shape
	if shape == nil {
		return fmt.Errorf("expected position %d to have shape %q, got none", i, want)
	}
	if string(*shape) != want {
		return fmt.Errorf("expected position %d to have shape %q, got %q", i, want, *shape)
	}
	return nil
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

// frettedPositionsFromTable reads interval/note_name/string/fret rows.
func frettedPositionsFromTable(table *godog.Table) ([]generated.DiagramPosition, error) {
	positions := make([]generated.DiagramPosition, 0, len(table.Rows)-1)
	for row := 1; row < len(table.Rows); row++ {
		interval, err := cell(table, row, "interval")
		if err != nil {
			return nil, err
		}
		note, err := cell(table, row, "note_name")
		if err != nil {
			return nil, err
		}
		str, fret, err := frettedCoordinates(table, row)
		if err != nil {
			return nil, err
		}
		positions = append(positions, generated.DiagramPosition{Interval: interval, NoteName: note, String: &str, Fret: &fret})
	}
	return positions, nil
}

func (w *world) createsBasicDiagramWithFrettedPositions(_, name, instrument, skills, concepts string, table *godog.Table) error {
	return w.createsDiagramWithKind("", name, instrument, string(generated.CreateDiagramRequestKindBasic), skills, concepts, table)
}

func (w *world) createsDiagramWithKind(_, name, instrument, kind, skills, concepts string, table *godog.Table) error {
	positions, err := frettedPositionsFromTable(table)
	if err != nil {
		return err
	}
	k := generated.CreateDiagramRequestKind(kind)
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID(instrument), Name: name, Positions: positions, Kind: &k,
		Classification: w.diagramClassification(skills, concepts),
	})
}

func (w *world) attemptsCreateBasicDiagram(string) error {
	basic := generated.CreateDiagramRequestKindBasic
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID("guitar"), Name: "Attempted template", Positions: onePositionOnGuitar(), Kind: &basic,
		Classification: w.diagramClassification("minor-pentatonic-scale", "scale-construction"),
	})
}

func (w *world) savesCopyOfDiagram(_, slug, name string) error {
	return w.saveCopy(slug, name, nil)
}

func (w *world) savesCopyOfDiagramAsBasic(_, slug, name string) error {
	basic := generated.CreateDiagramRequestKindBasic
	return w.saveCopy(slug, name, &basic)
}

// saveCopy does what the diagram editor's "Save as" does: it reads the
// source diagram as the caller, then creates a new one from what it read —
// every authored field carried over, position ids left for the server to
// assign. The source as read is kept so later steps can compare against it.
func (w *world) saveCopy(slug, name string, kind *generated.CreateDiagramRequestKind) error {
	resp, err := w.handler.GetDiagram(w.ctx(), generated.GetDiagramRequestObject{DiagramId: diagramID(slug)})
	if err != nil {
		return err
	}
	source, ok := resp.(generated.GetDiagram200JSONResponse)
	if !ok {
		return fmt.Errorf("expected to read diagram %q, got %#v", slug, resp)
	}
	w.copySource = generated.Diagram(source)

	positions := make([]generated.DiagramPosition, len(source.Positions))
	for i, p := range source.Positions {
		p.PositionId = nil
		positions[i] = p
	}
	skills := make([]uuid.UUID, len(source.Classification.Skills))
	for i, sk := range source.Classification.Skills {
		skills[i] = sk.SkillId
	}
	concepts := make([]uuid.UUID, len(source.Classification.Concepts))
	for i, c := range source.Classification.Concepts {
		concepts[i] = c.ConceptId
	}
	labelDisplay := generated.CreateDiagramRequestLabelDisplay(source.LabelDisplay)
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: source.InstrumentId, Name: name, Kind: kind, Positions: positions,
		RootNote: source.RootNote, LabelDisplay: &labelDisplay, Color: source.Color,
		Classification: generated.DiagramClassificationInput{SkillIds: skills, ConceptIds: concepts},
	})
}

func (w *world) diagramKindIs(want string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if string(diagram.Kind) != want {
		return fmt.Errorf("expected kind %q, got %q", want, diagram.Kind)
	}
	return nil
}

func (w *world) diagramRecordsCreator(name string) error {
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	want, ok := w.userMotifID[name]
	if !ok {
		return fmt.Errorf("no user %q has been registered in this scenario", name)
	}
	if diagram.CreatedBy != want {
		return fmt.Errorf("expected created_by %s (%s), got %s", want, name, diagram.CreatedBy)
	}
	return nil
}

// authoredPosition is a position's authored content, without its id.
type authoredPosition struct {
	interval, note, shape, color, key string
	str, fret                         int
}

func authoredPositions(positions []generated.DiagramPosition) []authoredPosition {
	out := make([]authoredPosition, len(positions))
	for i, p := range positions {
		a := authoredPosition{interval: p.Interval, note: p.NoteName}
		if p.Shape != nil {
			a.shape = string(*p.Shape)
		}
		if p.Color != nil {
			a.color = *p.Color
		}
		if p.Key != nil {
			a.key = *p.Key
		}
		if p.String != nil {
			a.str = *p.String
		}
		if p.Fret != nil {
			a.fret = *p.Fret
		}
		out[i] = a
	}
	return out
}

func (w *world) newDiagramCopiesPositionsOf(slug string) error {
	created, ok := w.lastResp.(generated.CreateDiagram201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a new diagram, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if created.DiagramId == diagramID(slug) {
		return fmt.Errorf("expected a new diagram, got %q itself", slug)
	}
	want, got := authoredPositions(w.copySource.Positions), authoredPositions(created.Positions)
	if !reflect.DeepEqual(want, got) {
		return fmt.Errorf("expected positions %+v, got %+v", want, got)
	}
	return nil
}

func (w *world) diagramIsUnchanged(slug string) error {
	stored, err := w.handler.GetDiagram(w.ctx(), generated.GetDiagramRequestObject{DiagramId: diagramID(slug)})
	if err != nil {
		return err
	}
	current, ok := stored.(generated.GetDiagram200JSONResponse)
	if !ok {
		return fmt.Errorf("expected to read diagram %q, got %#v", slug, stored)
	}
	if !reflect.DeepEqual(w.copySource, generated.Diagram(current)) {
		return fmt.Errorf("expected diagram %q unchanged:\nbefore %+v\nafter  %+v", slug, w.copySource, current)
	}
	return nil
}

func (w *world) responseIsDiagram(slug string) error {
	resp, ok := w.lastResp.(generated.GetDiagram200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a diagram, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.DiagramId != diagramID(slug) {
		return fmt.Errorf("expected diagram %s, got %s", diagramID(slug), resp.DiagramId)
	}
	return nil
}

func (w *world) responseIncludesThree(a, b, c string) error {
	for _, slug := range []string{a, b, c} {
		if err := w.responseIncludes(slug); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) listDiagrams(params generated.ListDiagramsParams) error {
	resp, err := w.handler.ListDiagrams(w.ctx(), generated.ListDiagramsRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsDiagramsOfKind(_, kind string) error {
	k := generated.ListDiagramsParamsKind(kind)
	return w.listDiagrams(generated.ListDiagramsParams{Kind: &k})
}

// listsDiagramsByCreator resolves creator by name, registering them as a
// teacher if no earlier step has, so that filtering by someone with no
// diagrams is still a filter by a real user id.
func (w *world) listsDiagramsByCreator(_, creator string) error {
	id := w.ensureRegistered(creator, domain.RoleTeacher)
	return w.listDiagrams(generated.ListDiagramsParams{CreatedBy: &id})
}

func (w *world) listsDiagramsPaged(_, tail string) error {
	limit, offset, err := pageParams(tail)
	if err != nil {
		return err
	}
	return w.listDiagrams(generated.ListDiagramsParams{Limit: limit, Offset: offset})
}

func (w *world) retrievesDiagram(_, slug string) error {
	resp, err := w.handler.GetDiagram(w.ctx(), generated.GetDiagramRequestObject{DiagramId: diagramID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}
