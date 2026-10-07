//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/domain"
)

// fakeNodeItemSource is an in-memory ports.NodeItemSource. It reads the
// play-alongs and exercises from the world's diagram and exercise fakes,
// plus the fretboard cells a scenario declares, which belong to a layout
// instrument and suit every instrument sharing that layout, and the diagram
// shapes the catalog steps install, per instrument.
type fakeNodeItemSource struct {
	diagrams  *fakeDiagramRepo
	exercises *fakeExerciseRepo
	// layouts maps an instrument to the layout instrument whose fretboard
	// it shares; cells holds the declared cells per layout instrument.
	layouts map[string]string
	cells   map[string][]domain.ClassifiedItem
	shapes  map[string][]domain.ClassifiedItem
}

func newFakeNodeItemSource(diagrams *fakeDiagramRepo, exercises *fakeExerciseRepo) *fakeNodeItemSource {
	return &fakeNodeItemSource{diagrams: diagrams, exercises: exercises, layouts: map[string]string{}, cells: map[string][]domain.ClassifiedItem{}, shapes: map[string][]domain.ClassifiedItem{}}
}

func (f *fakeNodeItemSource) ClassifiedItems(_ context.Context, instrumentID string) ([]domain.ClassifiedItem, error) {
	var items []domain.ClassifiedItem
	f.diagrams.mu.Lock()
	for _, d := range f.diagrams.byID {
		if d.Kind == domain.DiagramKindBasic && domain.DiagramPurposeFilterGeneral.Matches(d.Purpose) && d.DefaultPlaybackID != nil && slices.Contains(d.InstrumentIDs, instrumentID) {
			items = append(items, domain.ClassifiedItem{ItemKey: domain.PlayAlongItemKey(d.ID), NodeIDs: append(d.SkillIDs(), d.ConceptIDs()...)})
		}
	}
	f.diagrams.mu.Unlock()
	f.exercises.mu.Lock()
	for _, e := range f.exercises.byID {
		if len(e.InstrumentIDs) == 0 || slices.Contains(e.InstrumentIDs, instrumentID) {
			items = append(items, domain.ClassifiedItem{ItemKey: domain.ExerciseItemKey(e.ID), NodeIDs: knowledgeNodeIDs(append(slices.Clone(e.Skills), e.Concepts...))})
		}
	}
	f.exercises.mu.Unlock()
	if layout, ok := f.layouts[instrumentID]; ok {
		items = append(items, f.cells[layout]...)
	}
	items = append(items, f.shapes[instrumentID]...)
	return items, nil
}

func knowledgeNodeIDs(nodes []domain.KnowledgeNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

func registerNodeLevelSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the skill "([^"]+)" whose items are the (\d+) guitar fretboard cells on strings (\d+) and (\d+), frets (\d+) to (\d+)$`, w.skillWithGuitarCells)
	sc.Step(`^the instruments "([^"]+)" and "([^"]+)" share the guitar fretboard layout$`, w.instrumentsShareGuitarLayout)
	sc.Step(`^the instrument "([^"]+)" with its own fretboard layout$`, w.instrumentWithOwnLayout)

	sc.Step(`^the concept "([^"]+)" has (\d+) exercises for every instrument$`, w.conceptHasExercisesForEveryInstrument)
	sc.Step(`^"([^"]+)" is fluent on (\d+) of the (\d+) cells and accurate on the other (\d+)$`, w.isFluentOnCellsAccurateOnRest)
	sc.Step(`^"([^"]+)" is accurate on all (\d+) cells(?:, practised on "[^"]+")?$`, w.isAccurateOnAllCells)
	sc.Step(`^"([^"]+)" is fluent on (\d+) of the (\d+) cells and has never practised the rest$`, w.isFluentOnCellsNeverPractisedRest)
	sc.Step(`^"([^"]+)" is fluent on every guitar cell and has never practised a bass cell$`, w.isFluentOnEveryGuitarCell)
	sc.Step(`^the skill "([^"]+)" also has bass fretboard cells on strings (\d+) and (\d+)$`, w.skillAlsoHasBassCells)
	sc.Step(`^the skill "([^"]+)" has the children "([^"]+)" and "([^"]+)"$`, w.skillHasTheChildren)
	sc.Step(`^"([^"]+)" is accurate on all (\d+) cells of "([^"]+)"$`, w.isAccurateOnAllCellsOf)
	sc.Step(`^"([^"]+)"'s "([^"]+)" on "?([^"]+?)"? shows (\d+) of (\d+) items met and the levels of its (\d+) children$`, w.nodeShowsCoverageAndChildren)
	sc.Step(`^"([^"]+)"'s "([^"]+)" on "?([^"]+?)"? shows no level of its own$`, w.nodeShowsNoLevelOfItsOwn)
	sc.Step(`^"([^"]+)" is accurate on all (\d+)$`, w.isAccurateOnAll)
	sc.Step(`^the skill "([^"]+)" has no practice items$`, w.skillHasNoPracticeItems)
	sc.Step(`^the skill "([^"]+)" requires "([^"]+)" at level "([^"]+)"$`, w.skillRequiresSkill)

	sc.Step(`^"([^"]+)"'s level for "([^"]+)" on "?([^"]+?)"? is "([^"]+)"$`, w.levelForNodeIs)
	sc.Step(`^"([^"]+)" has no level for "([^"]+)" on "?([^"]+?)"?$`, w.hasNoLevelForNode)
	sc.Step(`^"([^"]+)"'s readiness for "([^"]+)" on "?([^"]+?)"? is (\d+) of (\d+) requirements met$`, w.readinessForNodeIs)
}

// skillWithGuitarCells declares the skill's items: the fretboard cells of
// the guitar layout on two strings over a range of frets.
func (w *world) skillWithGuitarCells(name string, count, firstString, secondString, fromFret, toFret int) error {
	skillID := w.skillIDFor(name).String()
	layout := instrumentID("guitar").String()
	var cells []domain.ClassifiedItem
	for _, s := range []int{firstString, secondString} {
		for fret := fromFret; fret <= toFret; fret++ {
			key := fmt.Sprintf("fretboard_cell:%s:%d:%d", layout, s, fret)
			cells = append(cells, domain.ClassifiedItem{ItemKey: key, NodeIDs: []string{skillID}})
		}
	}
	if len(cells) != count {
		return fmt.Errorf("strings %d and %d over frets %d to %d make %d cells, not %d", firstString, secondString, fromFret, toFret, len(cells), count)
	}
	w.practiceItems.cells[layout] = append(w.practiceItems.cells[layout], cells...)
	return nil
}

func (w *world) instrumentsShareGuitarLayout(first, second string) error {
	for _, name := range []string{first, second} {
		if _, err := w.ensureInstrumentSeeded(name); err != nil {
			return err
		}
		w.practiceItems.layouts[instrumentID(name).String()] = instrumentID("guitar").String()
	}
	return nil
}

func (w *world) instrumentWithOwnLayout(name string) error {
	if _, err := w.ensureInstrumentSeeded(name); err != nil {
		return err
	}
	w.practiceItems.layouts[instrumentID(name).String()] = instrumentID(name).String()
	return nil
}

// conceptHasExercisesForEveryInstrument seeds count exercises for every
// instrument classified under the concept; they are the items a following
// "is accurate on all" step refers to.
func (w *world) conceptHasExercisesForEveryInstrument(name string, count int) error {
	conceptID := w.conceptIDFor(name).String()
	w.lastPracticeItemKeys = nil
	for i := range count {
		id := exerciseID(fmt.Sprintf("%s-%d", name, i+1)).String()
		w.exercises.put(domain.Exercise{
			ID: id, Title: fmt.Sprintf("%s %d", name, i+1), ExerciseType: domain.ExerciseTypeTextResponse,
			Concepts: []domain.KnowledgeNode{{ID: conceptID}}, CreatedAt: fixedNow,
		})
		w.lastPracticeItemKeys = append(w.lastPracticeItemKeys, domain.ExerciseItemKey(id))
	}
	return nil
}

func (w *world) isAccurateOnAll(name string, count int) error {
	if len(w.lastPracticeItemKeys) != count {
		return fmt.Errorf("the scenario set up %d items, not %d", len(w.lastPracticeItemKeys), count)
	}
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	dueAt := fixedNow.AddDate(0, 0, 3)
	for _, key := range w.lastPracticeItemKeys {
		w.practiceStates.put(studentID, domain.PracticeItemState{
			ItemKey: key, RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelAccurate,
			Counted: 3, Box: 2, DueAt: &dueAt, LastAt: &fixedNow,
		})
	}
	return nil
}

func (w *world) skillHasNoPracticeItems(name string) error {
	w.skillIDFor(name)
	return nil
}

func (w *world) skillRequiresSkill(from, to, level string) error {
	return w.nodeRequiresNode(string(domain.KnowledgeNodeKindSkill), from, string(domain.KnowledgeNodeKindSkill), to, level)
}

// standingOn rolls the student's item states up for instrument and
// returns their standing on the skill or concept called nodeName.
func (w *world) standingOn(name, nodeName, instrument string) (domain.NodeStanding, error) {
	id, ok := w.skillIDByName[nodeName]
	if !ok {
		if id, ok = w.conceptIDByName[nodeName]; !ok {
			return domain.NodeStanding{}, fmt.Errorf("no skill or concept called %q", nodeName)
		}
	}
	standings, err := w.rollup.Standings(w.ctx(), w.ensureRegistered(name, domain.RoleStudent).String(), instrumentID(instrument).String())
	if err != nil {
		return domain.NodeStanding{}, err
	}
	standing, ok := standings[id.String()]
	if !ok {
		return domain.NodeStanding{}, fmt.Errorf("%q is not for %q", nodeName, instrument)
	}
	return standing, nil
}

func (w *world) levelForNodeIs(name, nodeName, instrument, want string) error {
	standing, err := w.standingOn(name, nodeName, instrument)
	if err != nil {
		return err
	}
	if standing.Level == nil {
		return fmt.Errorf("%q has no level on %q, want %q", nodeName, instrument, want)
	}
	if string(*standing.Level) != want {
		return fmt.Errorf("%q's level on %q is %q, want %q", nodeName, instrument, *standing.Level, want)
	}
	return nil
}

func (w *world) hasNoLevelForNode(name, nodeName, instrument string) error {
	standing, err := w.standingOn(name, nodeName, instrument)
	if err != nil {
		return err
	}
	if standing.Level != nil {
		return fmt.Errorf("%q's level on %q is %q, want none", nodeName, instrument, *standing.Level)
	}
	return nil
}

func (w *world) readinessForNodeIs(name, nodeName, instrument string, met, total int) error {
	standing, err := w.standingOn(name, nodeName, instrument)
	if err != nil {
		return err
	}
	if want := (domain.Readiness{Met: met, Total: total}); standing.Readiness != want {
		return fmt.Errorf("%q's readiness on %q is %d of %d, want %d of %d", nodeName, instrument, standing.Readiness.Met, standing.Readiness.Total, met, total)
	}
	return nil
}

// guitarCells returns the cells declared on the guitar layout, checking the
// scenario declared as many as it counts on.
func (w *world) guitarCells(count int) ([]domain.ClassifiedItem, error) {
	cells := w.practiceItems.cells[instrumentID("guitar").String()]
	if count >= 0 && len(cells) != count {
		return nil, fmt.Errorf("the scenario declared %d guitar cells, not %d", len(cells), count)
	}
	return cells, nil
}

// holdCells gives the student a state at level on each of cells, practised
// and due again in a few days, so the level shown is the level earned.
func (w *world) holdCells(name string, cells []domain.ClassifiedItem, level domain.KnowledgeLevel) {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	dueAt := fixedNow.AddDate(0, 0, 3)
	counted := map[domain.KnowledgeLevel]int{domain.KnowledgeLevelAccurate: 3, domain.KnowledgeLevelFluent: 5}[level]
	for _, cell := range cells {
		w.practiceStates.put(studentID, domain.PracticeItemState{
			ItemKey: cell.ItemKey, RulesVersion: domain.PracticeRulesVersion, Level: level,
			Counted: counted, Box: 2, DueAt: &dueAt, LastAt: &fixedNow,
		})
	}
}

func (w *world) isFluentOnCellsAccurateOnRest(name string, fluent, total, accurate int) error {
	cells, err := w.guitarCells(total)
	if err != nil {
		return err
	}
	if fluent+accurate != total {
		return fmt.Errorf("%d fluent and %d accurate cells don't make %d", fluent, accurate, total)
	}
	w.holdCells(name, cells[:fluent], domain.KnowledgeLevelFluent)
	w.holdCells(name, cells[fluent:], domain.KnowledgeLevelAccurate)
	return nil
}

func (w *world) isAccurateOnAllCells(name string, total int) error {
	cells, err := w.guitarCells(total)
	if err != nil {
		return err
	}
	w.holdCells(name, cells, domain.KnowledgeLevelAccurate)
	return nil
}

func (w *world) isFluentOnCellsNeverPractisedRest(name string, fluent, total int) error {
	cells, err := w.guitarCells(total)
	if err != nil {
		return err
	}
	w.holdCells(name, cells[:fluent], domain.KnowledgeLevelFluent)
	return nil
}

func (w *world) isFluentOnEveryGuitarCell(name string) error {
	cells, err := w.guitarCells(-1)
	if err != nil {
		return err
	}
	w.holdCells(name, cells, domain.KnowledgeLevelFluent)
	return nil
}

// skillAlsoHasBassCells declares the skill's cells on the bass layout, frets
// 0 to 12 like the guitar cells of the background.
func (w *world) skillAlsoHasBassCells(name string, first, second int) error {
	skillID := w.skillIDFor(name).String()
	layout := instrumentID("electric-bass").String()
	for _, s := range []int{first, second} {
		for fret := 0; fret <= 12; fret++ {
			w.practiceItems.cells[layout] = append(w.practiceItems.cells[layout],
				domain.ClassifiedItem{ItemKey: domain.FretboardCellItemKey(layout, s, fret), NodeIDs: []string{skillID}})
		}
	}
	return nil
}

func (w *world) skillHasTheChildren(parent, first, second string) error {
	parentID := w.skillIDFor(parent)
	for _, child := range []string{first, second} {
		w.putSkill(child, &parentID)
	}
	return nil
}

func (w *world) isAccurateOnAllCellsOf(name string, count int, skill string) error {
	skillID := w.skillIDFor(skill).String()
	cells, err := w.guitarCells(-1)
	if err != nil {
		return err
	}
	var mine []domain.ClassifiedItem
	for _, cell := range cells {
		if slices.Contains(cell.NodeIDs, skillID) {
			mine = append(mine, cell)
		}
	}
	if len(mine) != count {
		return fmt.Errorf("%q has %d guitar cells, not %d", skill, len(mine), count)
	}
	w.holdCells(name, mine, domain.KnowledgeLevelAccurate)
	return nil
}

func (w *world) nodeShowsCoverageAndChildren(name, nodeName, instrument string, met, total, children int) error {
	standing, err := w.standingOn(name, nodeName, instrument)
	if err != nil {
		return err
	}
	if standing.Covered != met || standing.Total != total {
		return fmt.Errorf("%q shows %d of %d items met, want %d of %d", nodeName, standing.Covered, standing.Total, met, total)
	}
	kids, err := w.knowledge.Children(w.ctx(), w.skillIDFor(nodeName).String())
	if err != nil {
		return err
	}
	withLevel := 0
	for _, kid := range kids {
		kidStanding, err := w.standingOn(name, w.nodeNameOf(kid.ID), instrument)
		if err != nil {
			return err
		}
		if kidStanding.Level != nil {
			withLevel++
		}
	}
	if withLevel != children {
		return fmt.Errorf("%q has %d children with a level, want %d", nodeName, withLevel, children)
	}
	return nil
}

// nodeNameOf is the scenario's name for the skill or concept with id.
func (w *world) nodeNameOf(id string) string {
	for name, nodeID := range w.skillIDByName {
		if nodeID.String() == id {
			return name
		}
	}
	for name, nodeID := range w.conceptIDByName {
		if nodeID.String() == id {
			return name
		}
	}
	return id
}

// nodeShowsNoLevelOfItsOwn checks the student is shown the node through its
// coverage and children: its level still serves requirements on it, but a
// wide node is never shown by it.
func (w *world) nodeShowsNoLevelOfItsOwn(name, nodeName, instrument string) error {
	view, err := w.rollup.Map(w.ctx(), w.ensureRegistered(name, domain.RoleStudent).String(), instrumentID(instrument).String())
	if err != nil {
		return err
	}
	if !view.Wide(w.skillIDFor(nodeName).String()) {
		return fmt.Errorf("%q is shown by a level of its own, want its coverage and children", nodeName)
	}
	return nil
}
