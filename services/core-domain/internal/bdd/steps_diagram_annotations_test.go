//go:build integration

package bdd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerDiagramAnnotationSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" with one position whose note is (\d+) characters long$`, w.createsDiagramWithLongNote)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" with one position and regions:$`, w.createsDiagramWithFretRegions)
	sc.Step(`^"([^"]+)" creates a diagram named "([^"]+)" on instrument "([^"]+)" with one position and keyboard regions:$`, w.createsDiagramWithKeyRegions)
	sc.Step(`^a custom diagram "([^"]+)" exists on instrument "([^"]+)", created by "([^"]+)", with a region spanning frets (\d+) to (\d+)$`, w.aCustomDiagramWithRegion)
	sc.Step(`^"([^"]+)" removes every region from diagram "([^"]+)"$`, w.removesEveryRegion)

	sc.Step(`^position (\d+)'s custom label in "([^"]+)" is "([^"]+)"$`, w.positionCustomLabelIs)
	sc.Step(`^position (\d+)'s note in "([^"]+)" is "([^"]+)"$`, w.positionNoteIs)
	sc.Step(`^position (\d+) has no custom label or note$`, w.positionHasNoAnnotations)
	sc.Step(`^the diagram has (\d+) regions?$`, w.diagramHasRegions)
	sc.Step(`^region (\d+) spans frets (\d+) to (\d+) on every string$`, w.regionSpansFretsOnEveryString)
	sc.Step(`^region (\d+) spans frets (\d+) to (\d+) on strings (\d+) to (\d+)$`, w.regionSpansFretsOnStrings)
	sc.Step(`^region (\d+) spans keys "([^"]+)" to "([^"]+)"$`, w.regionSpansKeys)
	sc.Step(`^region (\d+)'s description in "([^"]+)" is "([^"]+)"$`, w.regionDescriptionIs)
	sc.Step(`^region (\d+) has color "([^"]+)"$`, w.regionHasColor)
}

// withEnglishAnnotations sets position's custom label and note from a
// table row's optional custom_label and note columns, which hold English
// text; an empty cell means none.
func withEnglishAnnotations(position *generated.DiagramPosition, table *godog.Table, row int) {
	if label := optionalCell(table, row, "custom_label"); label != "" {
		position.CustomLabel = &generated.LocalizedMarkerLabel{"en": label}
	}
	if note := optionalCell(table, row, "note"); note != "" {
		position.Note = &generated.LocalizedNote{"en": note}
	}
}

// createOnePositionDiagram creates an English-named diagram with a single
// position shaped for instrumentName's family, adjusted by withPosition,
// and regions (nil for none).
func (w *world) createOnePositionDiagram(name, instrumentName string, withPosition func(*generated.DiagramPosition), regions *[]generated.DiagramRegion) error {
	instrument, err := w.ensureInstrumentSeeded(instrumentName)
	if err != nil {
		return err
	}
	position := onePositionOnGuitar()[0]
	if instrument.Family == domain.InstrumentFamilyKeyboard {
		key := "C4"
		position = generated.DiagramPosition{Interval: "R", NoteName: "C", Key: &key}
	}
	if withPosition != nil {
		withPosition(&position)
	}
	return w.createDiagram(generated.CreateDiagramRequest{
		InstrumentId: instrumentID(instrumentName), Names: english(name), Positions: []generated.DiagramPosition{position},
		Regions:        regions,
		Classification: w.diagramClassification("minor-pentatonic-scale", "scale-construction"),
	})
}

func (w *world) createsDiagramWithLongNote(_, name, instrument, length string) error {
	n, err := strconv.Atoi(length)
	if err != nil {
		return fmt.Errorf("note length %q is not a number: %w", length, err)
	}
	return w.createOnePositionDiagram(name, instrument, func(p *generated.DiagramPosition) {
		p.Note = &generated.LocalizedNote{"en": strings.Repeat("x", n)}
	}, nil)
}

// optionalIntCell is optionalCell parsed as a number; an empty cell is nil.
func optionalIntCell(table *godog.Table, row int, column string) (*int, error) {
	value := optionalCell(table, row, column)
	if value == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("%s %q is not a number: %w", column, value, err)
	}
	return &n, nil
}

// regionFromRow reads a region from a table row: optional fret_start,
// fret_end, string_start, string_end, key_start, key_end and color columns,
// and an English description.
func regionFromRow(table *godog.Table, row int) (generated.DiagramRegion, error) {
	region := generated.DiagramRegion{Description: generated.LocalizedCaption{"en": optionalCell(table, row, "description")}}
	for column, target := range map[string]**int{
		"fret_start": &region.FretStart, "fret_end": &region.FretEnd,
		"string_start": &region.StringStart, "string_end": &region.StringEnd,
	} {
		n, err := optionalIntCell(table, row, column)
		if err != nil {
			return generated.DiagramRegion{}, err
		}
		*target = n
	}
	for column, target := range map[string]**string{"key_start": &region.KeyStart, "key_end": &region.KeyEnd, "color": &region.Color} {
		if value := optionalCell(table, row, column); value != "" {
			*target = &value
		}
	}
	return region, nil
}

func (w *world) createsDiagramWithRegions(name, instrument string, table *godog.Table) error {
	regions := make([]generated.DiagramRegion, 0, len(table.Rows)-1)
	for row := 1; row < len(table.Rows); row++ {
		region, err := regionFromRow(table, row)
		if err != nil {
			return err
		}
		regions = append(regions, region)
	}
	return w.createOnePositionDiagram(name, instrument, nil, &regions)
}

func (w *world) createsDiagramWithFretRegions(_, name, instrument string, table *godog.Table) error {
	return w.createsDiagramWithRegions(name, instrument, table)
}

func (w *world) createsDiagramWithKeyRegions(_, name, instrument string, table *godog.Table) error {
	return w.createsDiagramWithRegions(name, instrument, table)
}

// aCustomDiagramWithRegion seeds a custom diagram owned by creator with one
// captioned region across every string between the given frets.
func (w *world) aCustomDiagramWithRegion(slug, instrumentName, creator, fretStart, fretEnd string) error {
	if err := w.aCustomDiagramExistsOn(slug, instrumentName, creator); err != nil {
		return err
	}
	start, err := strconv.Atoi(fretStart)
	if err != nil {
		return fmt.Errorf("fret %q is not a number: %w", fretStart, err)
	}
	end, err := strconv.Atoi(fretEnd)
	if err != nil {
		return fmt.Errorf("fret %q is not a number: %w", fretEnd, err)
	}
	diagram, err := w.diagrams.GetByID(w.ctx(), diagramID(slug).String())
	if err != nil {
		return err
	}
	diagram.Regions = []domain.Region{{
		ID: deterministicUUID("region", slug).String(), FretStart: &start, FretEnd: &end,
		Description: domain.LocalizedText{"en": "Box"},
	}}
	w.diagrams.put(diagram)
	return nil
}

func (w *world) removesEveryRegion(_, slug string) error {
	body := generated.UpdateDiagramRequest{Regions: &[]generated.DiagramRegion{}}
	resp, err := w.handler.UpdateDiagram(w.ctx(), generated.UpdateDiagramRequestObject{DiagramId: diagramID(slug), Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) positionCustomLabelIs(index, language, want string) error {
	position, err := w.positionAt(index)
	if err != nil {
		return err
	}
	if position.CustomLabel == nil || (*position.CustomLabel)[language] != want {
		return fmt.Errorf("expected position %s's custom label in %q to be %q, got %v", index, language, want, position.CustomLabel)
	}
	return nil
}

func (w *world) positionNoteIs(index, language, want string) error {
	position, err := w.positionAt(index)
	if err != nil {
		return err
	}
	if position.Note == nil || (*position.Note)[language] != want {
		return fmt.Errorf("expected position %s's note in %q to be %q, got %v", index, language, want, position.Note)
	}
	return nil
}

func (w *world) positionHasNoAnnotations(index string) error {
	position, err := w.positionAt(index)
	if err != nil {
		return err
	}
	if position.CustomLabel != nil || position.Note != nil {
		return fmt.Errorf("expected position %s to have no custom label or note, got %v and %v", index, position.CustomLabel, position.Note)
	}
	return nil
}

func (w *world) diagramHasRegions(count string) error {
	want, err := strconv.Atoi(count)
	if err != nil {
		return fmt.Errorf("region count %q is not a number: %w", count, err)
	}
	diagram, err := w.currentDiagram()
	if err != nil {
		return err
	}
	if len(diagram.Regions) != want {
		return fmt.Errorf("expected %d regions, got %d", want, len(diagram.Regions))
	}
	for i, r := range diagram.Regions {
		if r.RegionId == nil || *r.RegionId == uuid.Nil {
			return fmt.Errorf("region %d has no region_id", i+1)
		}
	}
	return nil
}

func (w *world) regionAt(index string) (generated.DiagramRegion, error) {
	i, err := strconv.Atoi(index)
	if err != nil {
		return generated.DiagramRegion{}, fmt.Errorf("region index %q is not a number: %w", index, err)
	}
	diagram, err := w.currentDiagram()
	if err != nil {
		return generated.DiagramRegion{}, err
	}
	if i < 1 || i > len(diagram.Regions) {
		return generated.DiagramRegion{}, fmt.Errorf("no region %d (diagram has %d)", i, len(diagram.Regions))
	}
	return diagram.Regions[i-1], nil
}

// intsEqual reports whether got is set and equal to want.
func intsEqual(got *int, want string) bool {
	return got != nil && strconv.Itoa(*got) == want
}

func (w *world) regionSpansFretsOnEveryString(index, fretStart, fretEnd string) error {
	region, err := w.regionAt(index)
	if err != nil {
		return err
	}
	if !intsEqual(region.FretStart, fretStart) || !intsEqual(region.FretEnd, fretEnd) || region.StringStart != nil || region.StringEnd != nil {
		return fmt.Errorf("expected region %s to span frets %s-%s on every string, got %#v", index, fretStart, fretEnd, region)
	}
	return nil
}

func (w *world) regionSpansFretsOnStrings(index, fretStart, fretEnd, stringStart, stringEnd string) error {
	region, err := w.regionAt(index)
	if err != nil {
		return err
	}
	if !intsEqual(region.FretStart, fretStart) || !intsEqual(region.FretEnd, fretEnd) || !intsEqual(region.StringStart, stringStart) || !intsEqual(region.StringEnd, stringEnd) {
		return fmt.Errorf("expected region %s to span frets %s-%s on strings %s-%s, got %#v", index, fretStart, fretEnd, stringStart, stringEnd, region)
	}
	return nil
}

func (w *world) regionSpansKeys(index, keyStart, keyEnd string) error {
	region, err := w.regionAt(index)
	if err != nil {
		return err
	}
	if region.KeyStart == nil || *region.KeyStart != keyStart || region.KeyEnd == nil || *region.KeyEnd != keyEnd {
		return fmt.Errorf("expected region %s to span keys %s-%s, got %#v", index, keyStart, keyEnd, region)
	}
	return nil
}

func (w *world) regionDescriptionIs(index, language, want string) error {
	region, err := w.regionAt(index)
	if err != nil {
		return err
	}
	if region.Description[language] != want {
		return fmt.Errorf("expected region %s's description in %q to be %q, got %v", index, language, want, region.Description)
	}
	return nil
}

func (w *world) regionHasColor(index, want string) error {
	region, err := w.regionAt(index)
	if err != nil {
		return err
	}
	if region.Color == nil || *region.Color != want {
		return fmt.Errorf("expected region %s to have color %q, got %v", index, want, region.Color)
	}
	return nil
}
