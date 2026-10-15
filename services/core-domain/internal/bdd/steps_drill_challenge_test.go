//go:build integration

package bdd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/domain"
)

// drillChallengeWorld is what the drill challenge steps share in a
// scenario.
type drillChallengeWorld struct {
	// planned counts the questions planned by kind; failures lists each
	// that didn't challenge.
	planned  map[domain.PracticeItemKind]int
	failures []string
	// stimulus is the one question a step planned, with the shape's
	// positions it was planned from.
	stimulus  domain.DrillStimulus
	positions []domain.Position
}

// degreeOrdinals are the degrees a step may name, as the shape catalog's
// intervals.
var degreeOrdinals = map[string]string{"2nd": "2", "3rd": "3", "4th": "4", "5th": "5", "6th": "6", "7th": "7"}

func registerDrillChallengeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^every item of every installed drill family is planned, in every way and every variant$`, w.everyDrillItemIsPlanned)
	sc.Step(`^no planned question sets a right answer apart from the other places or options by a label, marker shape, colour, highlight or size$`, w.noPlannedQuestionMarksItsAnswer)
	sc.Step(`^the shape "([^"]+)" is planned as find_the_degree for the (\w+)$`, w.shapePlannedToFindDegree)
	sc.Step(`^the root's places are marked as the root$`, w.rootPlacesMarkedAsRoot)
	sc.Step(`^every other place of the shape is drawn the same, the (\w+)'s included$`, w.otherPlacesDrawnTheSame)
}

func (w *world) drillChallenge() *drillChallengeWorld {
	if w.drillChallenges == nil {
		w.drillChallenges = &drillChallengeWorld{planned: map[domain.PracticeItemKind]int{}}
	}
	return w.drillChallenges
}

// everyDrillItemIsPlanned installs the drill catalog and the shape catalog,
// then plans every fretboard cell in each way and each window holding it,
// every shape in each way and for each degree it can be asked, and every
// harmony item in each way, level and reading, checking each question.
func (w *world) everyDrillItemIsPlanned() error {
	if err := w.practiceDrillCatalogIsInstalled(); err != nil {
		return err
	}
	if err := w.loadShapeCatalog(); err != nil {
		return err
	}
	for _, cells := range w.practiceItems.cells {
		for _, classified := range cells {
			cell, ok := domain.ParseFretboardCellItemKey(classified.ItemKey)
			if !ok {
				return fmt.Errorf("%q is no fretboard cell", classified.ItemKey)
			}
			for _, window := range domain.FretWindows() {
				if !window.Contains(cell.Fret) {
					continue
				}
				for _, way := range cell.Ways() {
					w.checkPlanned(cell, way, domain.DrillPlanning{Window: &window})
				}
			}
		}
	}
	for _, shape := range w.diagramShapes().shapes {
		item := domain.DiagramShapeItem{Diagram: catalogShapeDiagram(shape)}
		for _, way := range item.Ways() {
			for _, pick := range everyPick(item, way) {
				w.checkPlanned(item, way, domain.DrillPlanning{Pick: pick})
			}
		}
	}
	return w.planEveryHarmonyItem()
}

func (w *world) checkPlanned(item domain.DrillItem, way string, planning domain.DrillPlanning) {
	c := w.drillChallenge()
	c.planned[item.Kind()]++
	if err := domain.CheckChallenge(item.Plan(way, planning).Stimulus()); err != nil {
		c.failures = append(c.failures, fmt.Sprintf("%s as %s: %v", item.ItemKey(), way, err))
	}
}

// everyPick is a pick of each of the choices planning item through way
// chooses among, or one pick when it chooses nothing.
func everyPick(item domain.DrillItem, way string) []func(int) int {
	choices := 1
	item.Plan(way, domain.DrillPlanning{Pick: func(n int) int { choices = n; return 0 }})
	picks := make([]func(int) int, choices)
	for i := range picks {
		picks[i] = func(int) int { return i }
	}
	return picks
}

// catalogShapeDiagram is shape as the drill plans it: its positions and its
// family member.
func catalogShapeDiagram(shape catalogShape) domain.Diagram {
	positions := make([]domain.Position, len(shape.diagram.Positions))
	for i, p := range shape.diagram.Positions {
		positions[i] = domain.Position{ID: fmt.Sprintf("%s-p%d", shape.diagram.DiagramID, i), Interval: p.Interval, String: &p.String, Fret: &p.Fret}
	}
	return domain.Diagram{
		ID: shape.diagram.DiagramID, Names: shape.diagram.Names, Positions: positions,
		Shape: &domain.DiagramShape{Family: shape.family.domainFamily(), Shape: shape.shape},
	}
}

func (w *world) noPlannedQuestionMarksItsAnswer() error {
	c := w.drillChallenge()
	for _, kind := range []domain.PracticeItemKind{
		domain.PracticeItemKindFretboardCell, domain.PracticeItemKindDiagramShape,
		domain.PracticeItemKindDiatonicChord, domain.PracticeItemKindChordFunction, domain.PracticeItemKindChartProgression,
	} {
		if c.planned[kind] == 0 {
			return fmt.Errorf("no %s question was planned", kind)
		}
	}
	if len(c.failures) > 0 {
		return fmt.Errorf("%d questions don't challenge, such as %s", len(c.failures), strings.Join(c.failures[:min(len(c.failures), 5)], "; "))
	}
	return nil
}

func (w *world) shapePlannedToFindDegree(name, ordinal string) error {
	degree, ok := degreeOrdinals[ordinal]
	if !ok {
		return fmt.Errorf("no degree is called %q", ordinal)
	}
	if err := w.loadShapeCatalog(); err != nil {
		return err
	}
	shape, err := w.shapeNamed(name)
	if err != nil {
		return err
	}
	item := domain.DiagramShapeItem{Diagram: catalogShapeDiagram(shape)}
	var degrees []string
	for _, p := range item.Diagram.Positions {
		if p.Interval != "R" && !slices.Contains(degrees, p.Interval) {
			degrees = append(degrees, p.Interval)
		}
	}
	i := slices.Index(degrees, degree)
	if i < 0 {
		return fmt.Errorf("%q has no %s", name, ordinal)
	}
	planned := item.Plan(string(domain.DiagramShapeDrillFindTheDegree), domain.DrillPlanning{Pick: func(int) int { return i }})
	c := w.drillChallenge()
	c.stimulus, c.positions = planned.Stimulus(), item.Diagram.Positions
	if !slices.Equal(c.stimulus.Prompt, []string{degree}) {
		return fmt.Errorf("the question asks %v, not the %s", c.stimulus.Prompt, ordinal)
	}
	return nil
}

func (w *world) rootPlacesMarkedAsRoot() error {
	c := w.drillChallenge()
	for i, p := range c.stimulus.Places {
		root := c.positions[i].Interval == "R"
		if root != p.Given || root != (p.Mark == domain.PlaceMark{Label: "R"}) {
			return fmt.Errorf("string %d fret %d (%s) is drawn %+v, given %t", p.String, p.Fret, c.positions[i].Interval, p.Mark, p.Given)
		}
	}
	return nil
}

func (w *world) otherPlacesDrawnTheSame(ordinal string) error {
	c := w.drillChallenge()
	degree := degreeOrdinals[ordinal]
	var marks []domain.PlaceMark
	drawn := 0
	for i, p := range c.stimulus.Places {
		if p.Given {
			continue
		}
		if !slices.Contains(marks, p.Mark) {
			marks = append(marks, p.Mark)
		}
		if c.positions[i].Interval == degree {
			drawn++
		}
	}
	if len(marks) != 1 {
		return fmt.Errorf("the shape's other places are drawn %d ways: %+v", len(marks), marks)
	}
	if drawn == 0 {
		return fmt.Errorf("the %s isn't drawn", ordinal)
	}
	return domain.CheckChallenge(c.stimulus)
}
