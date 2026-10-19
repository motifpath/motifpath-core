//go:build integration

package bdd

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// Steps for a diagram image_recognition exercise whose answers are the
// fretboard cells of the diagram's answer window.
func registerDiagramAnswerSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" creates an image_recognition exercise titled "([^"]+)" from diagram "([^"]+)"(?: labelled by "([^"]+)")? with the positions? at (.+?) correct(?:, hiding the position at (.+))?$`, w.createsCellExercise)
	sc.Step(`^"([^"]+)" creates an image_recognition exercise from diagram "([^"]+)" with (no correct position|a correct position not in the diagram)$`, w.createsCellExerciseWithUnusableAnswers)
	sc.Step(`^the exercise has (\d+) options, one per cell from fret (\d+) to fret (\d+) on each of (\d+) strings$`, w.exerciseHasCellOptions)
	sc.Step(`^the only correct options are the cells at (.+)$`, w.onlyCorrectCellsAre)
	sc.Step(`^the option at string (\d+) fret (\d+) names its diagram position and is not correct$`, w.cellOptionNamesPositionAndIsWrong)
	sc.Step(`^no option is an open string$`, w.noOpenStringOption)
	sc.Step(`^the exercise's stimulus records the positions at (.+) as correct$`, w.stimulusRecordsCorrectPositions)
	sc.Step(`^the exercise's stimulus is labelled by "([^"]+)" and hides the position at string (\d+) fret (\d+)$`, w.stimulusLabelledAndHiding)
}

var stringFretClause = regexp.MustCompile(`string (\d+) fret (\d+)`)

// cellsIn reads every "string N fret M" in text.
func cellsIn(text string) ([]domain.FretCell, error) {
	var cells []domain.FretCell
	for _, m := range stringFretClause.FindAllStringSubmatch(text, -1) {
		str, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, err
		}
		fret, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, err
		}
		cells = append(cells, domain.FretCell{String: str, Fret: fret})
	}
	if len(cells) == 0 {
		return nil, fmt.Errorf("no \"string N fret M\" in %q", text)
	}
	return cells, nil
}

// positionOnCell is the id of diagramSlug's position on cell.
func (w *world) positionOnCell(diagramSlug string, cell domain.FretCell) (openapi_types.UUID, error) {
	diagram, err := w.diagrams.GetByID(context.Background(), diagramID(diagramSlug).String())
	if err != nil {
		return openapi_types.UUID{}, err
	}
	for _, p := range diagram.Positions {
		if p.String != nil && p.Fret != nil && *p.String == cell.String && *p.Fret == cell.Fret {
			return uuid.MustParse(p.ID), nil
		}
	}
	return openapi_types.UUID{}, fmt.Errorf("diagram %q has no position at string %d fret %d", diagramSlug, cell.String, cell.Fret)
}

func (w *world) positionsAt(diagramSlug, text string) ([]openapi_types.UUID, error) {
	cells, err := cellsIn(text)
	if err != nil {
		return nil, err
	}
	ids := make([]openapi_types.UUID, len(cells))
	for i, cell := range cells {
		if ids[i], err = w.positionOnCell(diagramSlug, cell); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func (w *world) createCellExercise(title string, ref generated.DiagramRef) error {
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{
		Body: &generated.CreateExerciseRequest{
			SkillIds: w.skillIDsFor("skill-1"), ConceptIds: w.conceptIDsFor("concept-1"),
			Title: title, Prompt: promptDocFor(title), ExerciseType: generated.CreateExerciseRequestExerciseTypeImageRecognition,
			DiagramRef:    &ref,
			LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsCellExercise(_, title, diagramSlug, label, correctText, hiddenText string) error {
	var ref generated.DiagramRef
	ref.DiagramId = diagramID(diagramSlug)
	if label != "" {
		l := generated.DiagramRefLayersLabel(label)
		ref.Layers.Label = &l
	}
	correct, err := w.positionsAt(diagramSlug, correctText)
	if err != nil {
		return err
	}
	ref.CorrectPositionIds = &correct
	if hiddenText != "" {
		hidden, err := w.positionsAt(diagramSlug, hiddenText)
		if err != nil {
			return err
		}
		ref.Layers.HiddenPositionIds = &hidden
	}
	return w.createCellExercise(title, ref)
}

func (w *world) createsCellExerciseWithUnusableAnswers(_, diagramSlug, which string) error {
	var ref generated.DiagramRef
	ref.DiagramId = diagramID(diagramSlug)
	correct := []openapi_types.UUID{}
	if which == "a correct position not in the diagram" {
		correct = append(correct, deterministicUUID("position", "elsewhere"))
	}
	ref.CorrectPositionIds = &correct
	return w.createCellExercise("Unusable answers", ref)
}

func (w *world) createdExercise() (generated.CreateExercise201JSONResponse, error) {
	resp, ok := w.lastResp.(generated.CreateExercise201JSONResponse)
	if !ok {
		return resp, fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return resp, nil
}

func (w *world) exerciseHasCellOptions(countStr, fromStr, toStr, stringsStr string) error {
	resp, err := w.createdExercise()
	if err != nil {
		return err
	}
	count, _ := strconv.Atoi(countStr)
	from, _ := strconv.Atoi(fromStr)
	to, _ := strconv.Atoi(toStr)
	strs, _ := strconv.Atoi(stringsStr)
	if len(resp.Options) != count || count != (to-from+1)*strs {
		return fmt.Errorf("expected %d options (%d frets × %d strings), got %d", count, to-from+1, strs, len(resp.Options))
	}
	seen := map[domain.FretCell]bool{}
	for _, opt := range resp.Options {
		if opt.FretCell == nil {
			return fmt.Errorf("expected every option to be a cell, got %+v", opt)
		}
		c := domain.FretCell{String: opt.FretCell.String, Fret: opt.FretCell.Fret}
		if c.Fret < from || c.Fret > to || c.String < 1 || c.String > strs || seen[c] {
			return fmt.Errorf("unexpected or repeated cell %+v", c)
		}
		seen[c] = true
	}
	return nil
}

func (w *world) onlyCorrectCellsAre(text string) error {
	resp, err := w.createdExercise()
	if err != nil {
		return err
	}
	want, err := cellsIn(text)
	if err != nil {
		return err
	}
	var got []domain.FretCell
	for _, opt := range resp.Options {
		if opt.IsCorrect && opt.FretCell != nil {
			got = append(got, domain.FretCell{String: opt.FretCell.String, Fret: opt.FretCell.Fret})
		}
	}
	for _, c := range want {
		if !slices.Contains(got, c) {
			return fmt.Errorf("expected the cell %+v to be correct; correct cells: %+v", c, got)
		}
	}
	if len(got) != len(want) {
		return fmt.Errorf("expected exactly %d correct cells, got %+v", len(want), got)
	}
	return nil
}

func (w *world) cellOptionNamesPositionAndIsWrong(strStr, fretStr string) error {
	resp, err := w.createdExercise()
	if err != nil {
		return err
	}
	str, _ := strconv.Atoi(strStr)
	fret, _ := strconv.Atoi(fretStr)
	for _, opt := range resp.Options {
		if opt.FretCell != nil && opt.FretCell.String == str && opt.FretCell.Fret == fret {
			if opt.DiagramPositionId == nil || opt.IsCorrect {
				return fmt.Errorf("expected the cell to name its position and be wrong, got %+v", opt)
			}
			return nil
		}
	}
	return fmt.Errorf("no option at string %d fret %d", str, fret)
}

func (w *world) noOpenStringOption() error {
	resp, err := w.createdExercise()
	if err != nil {
		return err
	}
	for _, opt := range resp.Options {
		if opt.FretCell != nil && opt.FretCell.Fret == 0 {
			return fmt.Errorf("expected no open-string option, got %+v", opt)
		}
	}
	return nil
}

// stimulusRef is the diagram stimulus the created exercise was stored with.
func (w *world) stimulusRef() (generated.DiagramRef, error) {
	resp, err := w.createdExercise()
	if err != nil {
		return generated.DiagramRef{}, err
	}
	if resp.DiagramRef == nil {
		return generated.DiagramRef{}, fmt.Errorf("expected the exercise to keep its diagram stimulus")
	}
	return *resp.DiagramRef, nil
}

func (w *world) stimulusRecordsCorrectPositions(text string) error {
	ref, err := w.stimulusRef()
	if err != nil {
		return err
	}
	want, err := w.positionsAtRef(ref, text)
	if err != nil {
		return err
	}
	if ref.CorrectPositionIds == nil || !sameUUIDs(*ref.CorrectPositionIds, want) {
		return fmt.Errorf("expected correct positions %v, got %v", want, ref.CorrectPositionIds)
	}
	if ref.CorrectIntervals != nil {
		return fmt.Errorf("expected correct_intervals to be converted away, got %v", *ref.CorrectIntervals)
	}
	return nil
}

func (w *world) stimulusLabelledAndHiding(label, strStr, fretStr string) error {
	ref, err := w.stimulusRef()
	if err != nil {
		return err
	}
	if ref.Layers.Label == nil || string(*ref.Layers.Label) != label {
		return fmt.Errorf("expected label %q, got %v", label, ref.Layers.Label)
	}
	want, err := w.positionsAtRef(ref, "string "+strStr+" fret "+fretStr)
	if err != nil {
		return err
	}
	if ref.Layers.HiddenPositionIds == nil || !sameUUIDs(*ref.Layers.HiddenPositionIds, want) {
		return fmt.Errorf("expected hidden positions %v, got %v", want, ref.Layers.HiddenPositionIds)
	}
	return nil
}

// positionsAtRef is positionsAt for the diagram a stimulus ref points at.
func (w *world) positionsAtRef(ref generated.DiagramRef, text string) ([]openapi_types.UUID, error) {
	diagram, err := w.diagrams.GetByID(context.Background(), ref.DiagramId.String())
	if err != nil {
		return nil, err
	}
	cells, err := cellsIn(text)
	if err != nil {
		return nil, err
	}
	ids := make([]openapi_types.UUID, 0, len(cells))
	for _, cell := range cells {
		for _, p := range diagram.Positions {
			if p.String != nil && p.Fret != nil && *p.String == cell.String && *p.Fret == cell.Fret {
				ids = append(ids, uuid.MustParse(p.ID))
			}
		}
	}
	return ids, nil
}

func sameUUIDs(a, b []openapi_types.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for _, id := range b {
		if !slices.Contains(a, id) {
			return false
		}
	}
	return true
}
