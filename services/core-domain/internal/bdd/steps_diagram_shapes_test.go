//go:build integration

package bdd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// basicGuitarCatalogFile is the basic-guitar diagram catalog as its generator
// writes it, next to the reference data migration that installs it.
const basicGuitarCatalogFile = "../../../../catalog/basic-guitar-v1/catalog.json"

// catalogFamilyClassification is the skill and concept each catalog family's
// diagrams are classified under, as the diagram catalog installs them. Only
// the families a drill shape family takes its diagrams from are listed.
var catalogFamilyClassification = map[string][2]string{
	"caged":          {"map-fretboard-caged", "caged-system"},
	"caged-window":   {"play-scale-positions", "scales"},
	"pentatonic-box": {"play-pentatonic-positions", "pentatonic-shapes"},
	"arpeggio":       {"play-arpeggios", "chords"},
}

// shapeFamilyEntry is one diagram_shapes family of the practice drill
// catalog.
type shapeFamilyEntry struct {
	Family   string            `yaml:"family"`
	Diagrams string            `yaml:"diagrams"`
	Names    map[string]string `yaml:"names"`
	Members  []shapeMember     `yaml:"members"`
}

// shapeMember is one member of a shapeFamilyEntry.
type shapeMember struct {
	Shape string            `yaml:"shape"`
	Names map[string]string `yaml:"names"`
}

// domainFamily is f as the drill catalog installs it.
func (f shapeFamilyEntry) domainFamily() domain.DiagramShapeFamily {
	members := make([]domain.DiagramShapeMember, len(f.Members))
	for i, m := range f.Members {
		members[i] = domain.DiagramShapeMember{Shape: m.Shape, Names: m.Names}
	}
	return domain.DiagramShapeFamily{ID: deterministicUUID("diagram-shape-family", f.Family).String(), Key: f.Family, Names: f.Names, Members: members}
}

// catalogDiagram is the part of a basic-guitar catalog entry the shape steps
// read.
type catalogDiagram struct {
	Key         string            `json:"key"`
	DiagramID   string            `json:"diagram_id"`
	Family      string            `json:"family"`
	Names       map[string]string `json:"names"`
	Instruments []string          `json:"instruments"`
	Positions   []struct {
		String   int    `json:"string"`
		Fret     int    `json:"fret"`
		Interval string `json:"interval"`
	} `json:"positions"`
}

// catalogShape is a catalog diagram a family takes: the member it is.
type catalogShape struct {
	diagram catalogDiagram
	family  shapeFamilyEntry
	shape   string
}

func (s catalogShape) itemKey() string { return domain.DiagramShapeItemKey(s.diagram.DiagramID) }

// diagramShapeWorld is what the diagram shape steps share in a scenario.
type diagramShapeWorld struct {
	families  []shapeFamilyEntry
	templates []string
	catalog   []catalogDiagram
	shapes    []catalogShape
	// listed holds the shapes a step listed; picked the one picked for
	// practice.
	listed []catalogShape
	picked *catalogShape
	// pending is a drill catalog a scenario declares, matched on install.
	pending    []shapeFamilyEntry
	installErr error
	// asked is the shape a session asked, from "is asked" steps, and
	// askedShape the catalog shape it asked.
	asked      *generated.PracticeSessionItem
	askedShape catalogShape
	standing   domain.NodeStanding
}

func registerDiagramShapeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the basic guitar diagram catalog is installed$`, w.basicGuitarDiagramCatalogIsInstalled)
	sc.Step(`^the shapes of family "([^"]+)" are listed$`, w.shapesOfFamilyListed)
	sc.Step(`^the shapes of every family are listed$`, w.shapesOfEveryFamilyListed)
	sc.Step(`^the shape "([^"]+)" is listed$`, w.shapeListed)
	sc.Step(`^the shapes "([^"]+)" and "([^"]+)" are listed$`, w.twoShapesListed)
	sc.Step(`^a shape of family "([^"]+)" is picked for practice$`, w.shapeOfFamilyPicked)
	sc.Step(`^there are (\d+) shapes, one per catalog diagram "([^"]+)"$`, w.thereAreShapesOnePerDiagram)
	sc.Step(`^each shape's item key names its diagram$`, w.eachShapeKeyNamesItsDiagram)
	sc.Step(`^the diagram "([^"]+)" is the member "([^"]+)"$`, w.diagramIsTheMember)
	sc.Step(`^they have different item keys$`, w.theyHaveDifferentItemKeys)
	sc.Step(`^the diagram "([^"]+)" is not among them$`, w.diagramIsNotAmongThem)
	sc.Step(`^a drill catalog listing the family "([^"]+)" with the pattern "([^"]+)"$`, w.catalogListingFamilyPattern)
	sc.Step(`^a drill catalog listing the family "([^"]+)" with only the members "([^"]+)", "([^"]+)", "([^"]+)" and "([^"]+)"$`, w.catalogListingFamilyMembers)
	sc.Step(`^the installation fails, naming the family "([^"]+)"$`, w.installationFailsNamingFamily)
	sc.Step(`^the installation fails, naming the family "([^"]+)" and the member "([^"]+)"$`, w.installationFailsNamingFamilyAndMember)

	sc.Step(`^the shape "([^"]+)" is asked as (name_the_shape|find_the_degree)$`, w.shapeIsAskedAs)
	sc.Step(`^the shape "([^"]+)" is asked$`, w.shapeIsAsked)
	sc.Step(`^the item's shape is "([^"]+)"$`, w.theItemsShapeIs)
	sc.Step(`^the item is drawn on the diagram's layout instrument$`, w.theItemIsDrawnOnLayoutInstrument)
	sc.Step(`^the options are "([^"]+)", "([^"]+)", "([^"]+)", "([^"]+)" and "([^"]+)", in that order$`, w.theOptionsAre)
	sc.Step(`^the asked degree is one of "([^"]+)", "([^"]+)", "([^"]+)" and "([^"]+)"$`, w.theAskedDegreeIsOneOf)
	sc.Step(`^the options include "([^"]+)", although B has no C-shape grip inside frets 0–12$`, w.theOptionsIncludeMissingMember)
	sc.Step(`^it suits "([^"]+)" and "([^"]+)" and no other instrument$`, w.itSuitsOnly)
	sc.Step(`^the only practice items of skill "([^"]+)" on guitar are (\d+) shapes$`, w.onlyItemsOfSkillAreShapes)
	sc.Step(`^"([^"]+)" is fluent on (\d+) of those shapes$`, w.isFluentOnShapes)
	sc.Step(`^"([^"]+)"'s level for "([^"]+)" on "?([^"]+?)"? is derived$`, w.levelForNodeIsDerived)
	sc.Step(`^it is "([^"]+)"$`, w.derivedLevelIs)

	sc.Step(`^"([^"]+)" has new shapes of skill "([^"]+)" on guitar$`, w.hasNewShapesOfSkill)
	sc.Step(`^"([^"]+)" has nothing due or weak on guitar and new shapes of skill "([^"]+)"$`, w.hasNewShapesOfSkill)
	sc.Step(`^"([^"]+)" has named the shape "([^"]+)" correctly (\d+) times and found its degrees correctly once$`, w.namedShapeAndFoundOnce)
	sc.Step(`^"([^"]+)" has never answered the shape "([^"]+)" correctly$`, w.neverAnsweredShapeRight)
	sc.Step(`^the shape is picked for "([^"]+)"'s session in the head$`, w.cellPickedInTheHead)
	sc.Step(`^the session may include diagram shapes linked to "([^"]+)"$`, w.everyShapeIsLinkedTo)
	sc.Step(`^the session includes diagram shapes linked to "([^"]+)"$`, w.sessionIncludesShapesLinkedTo)
	sc.Step(`^no diagram shape in the session is linked only to instruments "([^"]+)" doesn't play$`, w.noShapeLinkedOnlyToUnplayed)
	sc.Step(`^every diagram shape in the session is estimated at (\d+) seconds$`, w.everyShapeEstimatedAt)
	sc.Step(`^"([^"]+)"'s next session will practise only diagram shapes$`, w.nextSessionPractisesOnlyShapes)
}

func (w *world) diagramShapes() *diagramShapeWorld {
	if w.shapeWorld == nil {
		w.shapeWorld = &diagramShapeWorld{}
	}
	return w.shapeWorld
}

// installShapeFamilies keeps the drill catalog's shape families and drill
// templates, matched against the diagram catalog once it is installed.
func (w *world) installShapeFamilies(file drillCatalogFile) {
	s := w.diagramShapes()
	s.families = file.DiagramShapes
	for _, t := range file.Templates {
		if t.ItemKind == string(domain.PracticeItemKindDiagramShape) {
			s.templates = append(s.templates, t.Key)
		}
	}
}

// basicGuitarDiagramCatalogIsInstalled reads the basic-guitar catalog,
// matches the drill catalog's families against it and installs every shape.
func (w *world) basicGuitarDiagramCatalogIsInstalled() error {
	if err := w.loadShapeCatalog(); err != nil {
		return err
	}
	for _, shape := range w.diagramShapes().shapes {
		if err := w.putCatalogShape(shape); err != nil {
			return err
		}
	}
	return nil
}

// loadShapeCatalog reads the basic-guitar catalog, and the drill catalog's
// families when no step installed them, and matches them, once a scenario.
func (w *world) loadShapeCatalog() error {
	s := w.diagramShapes()
	if s.catalog != nil {
		return nil
	}
	if s.families == nil {
		var file drillCatalogFile
		if err := readDrillCatalog(&file); err != nil {
			return err
		}
		w.installShapeFamilies(file)
	}
	raw, err := os.ReadFile(basicGuitarCatalogFile)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &s.catalog); err != nil {
		return err
	}
	s.shapes, err = matchShapes(s.families, s.catalog)
	return err
}

// putCatalogShape installs a catalog shape as the diagram fakes hold it:
// linked to its catalog instruments, classified as the diagram catalog
// classifies its family, and the member of its drill family. It is left
// without playback, so it is a shape and never also a play-along.
func (w *world) putCatalogShape(shape catalogShape) error {
	classification, ok := catalogFamilyClassification[shape.diagram.Family]
	if !ok {
		return fmt.Errorf("no classification for catalog family %q", shape.diagram.Family)
	}
	var instrumentIDs []string
	for _, name := range shape.diagram.Instruments {
		instrument, err := w.ensureCatalogInstrument(name)
		if err != nil {
			return err
		}
		instrumentIDs = append(instrumentIDs, instrument.ID)
	}
	positions := make([]domain.Position, len(shape.diagram.Positions))
	for i, p := range shape.diagram.Positions {
		positions[i] = domain.Position{ID: fmt.Sprintf("%s-p%d", shape.diagram.DiagramID, i), Interval: p.Interval, String: &p.String, Fret: &p.Fret, Shape: domain.PositionShapeDot}
	}
	w.diagrams.put(domain.Diagram{
		ID: shape.diagram.DiagramID, InstrumentID: instrumentIDs[0], InstrumentIDs: instrumentIDs,
		Names: shape.diagram.Names, Kind: domain.DiagramKindBasic, CreatedBy: w.curatorID(),
		Positions: positions, CreatedAt: fixedNow,
		Skills:   []domain.KnowledgeNode{{ID: w.skillIDFor(classification[0]).String()}},
		Concepts: []domain.KnowledgeNode{{ID: w.conceptIDFor(classification[1]).String()}},
		Shape:    &domain.DiagramShape{Family: shape.family.domainFamily(), Shape: shape.shape},
	})
	return nil
}

// matchShapes makes each catalog diagram whose key matches a family's
// pattern a shape of the member its {shape} names, refusing what the drill
// catalog's installer refuses: a family matching no diagram and a shape its
// family doesn't list.
func matchShapes(families []shapeFamilyEntry, catalog []catalogDiagram) ([]catalogShape, error) {
	placeholder := regexp.MustCompile(`\\\{([a-z]+)\\\}`)
	var shapes []catalogShape
	for _, f := range families {
		pattern := regexp.MustCompile("^" + placeholder.ReplaceAllString(regexp.QuoteMeta(f.Diagrams), "(?P<$1>[^/]+)") + "$")
		members := make([]string, len(f.Members))
		for i, m := range f.Members {
			members[i] = m.Shape
		}
		matched := 0
		for _, d := range catalog {
			m := pattern.FindStringSubmatch(d.Key)
			if m == nil {
				continue
			}
			shape := m[pattern.SubexpIndex("shape")]
			if !slices.Contains(members, shape) {
				return nil, fmt.Errorf("%w: diagram %q is shape %q, which is not a member of family %q", domain.ErrInvalidDrillCatalog, d.Key, shape, f.Family)
			}
			shapes = append(shapes, catalogShape{diagram: d, family: f, shape: shape})
			matched++
		}
		if matched == 0 {
			return nil, fmt.Errorf("%w: shape family %q matches no catalog diagram", domain.ErrInvalidDrillCatalog, f.Family)
		}
	}
	return shapes, nil
}

func (w *world) shapeNamed(name string) (catalogShape, error) {
	for _, s := range w.diagramShapes().shapes {
		if s.diagram.Names["en"] == name {
			return s, nil
		}
	}
	return catalogShape{}, fmt.Errorf("no catalog shape is called %q", name)
}

func (w *world) shapesOfFamilyListed(family string) error {
	s := w.diagramShapes()
	s.listed = nil
	for _, shape := range s.shapes {
		if shape.family.Family == family {
			s.listed = append(s.listed, shape)
		}
	}
	return nil
}

func (w *world) shapesOfEveryFamilyListed() error {
	s := w.diagramShapes()
	s.listed = slices.Clone(s.shapes)
	return nil
}

func (w *world) shapeListed(name string) error {
	shape, err := w.shapeNamed(name)
	if err != nil {
		return err
	}
	w.diagramShapes().listed = []catalogShape{shape}
	return nil
}

func (w *world) twoShapesListed(first, second string) error {
	s := w.diagramShapes()
	s.listed = nil
	for _, name := range []string{first, second} {
		shape, err := w.shapeNamed(name)
		if err != nil {
			return err
		}
		s.listed = append(s.listed, shape)
	}
	return nil
}

func (w *world) shapeOfFamilyPicked(family string) error {
	s := w.diagramShapes()
	for _, shape := range s.shapes {
		if shape.family.Family == family {
			s.picked = &shape
			return nil
		}
	}
	return fmt.Errorf("family %q has no shapes", family)
}

func (w *world) thereAreShapesOnePerDiagram(count int, diagrams string) error {
	s := w.diagramShapes()
	if len(s.listed) != count {
		return fmt.Errorf("%d shapes are listed, want %d", len(s.listed), count)
	}
	pattern := regexp.MustCompile("^" + regexp.MustCompile(`\\\{[a-z]+\\\}`).ReplaceAllString(regexp.QuoteMeta(diagrams), "[^/]+") + "$")
	seen := map[string]bool{}
	for _, shape := range s.listed {
		if !pattern.MatchString(shape.diagram.Key) {
			return fmt.Errorf("shape diagram %q doesn't match %q", shape.diagram.Key, diagrams)
		}
		if seen[shape.diagram.DiagramID] {
			return fmt.Errorf("diagram %q is listed twice", shape.diagram.Key)
		}
		seen[shape.diagram.DiagramID] = true
	}
	matching := 0
	for _, d := range s.catalog {
		if pattern.MatchString(d.Key) {
			matching++
		}
	}
	if matching != count {
		return fmt.Errorf("%d catalog diagrams match %q, want %d", matching, diagrams, count)
	}
	return nil
}

func (w *world) eachShapeKeyNamesItsDiagram() error {
	for _, shape := range w.diagramShapes().listed {
		if shape.itemKey() != "diagram_shape:"+shape.diagram.DiagramID {
			return fmt.Errorf("shape %q has item key %q", shape.diagram.Key, shape.itemKey())
		}
	}
	return nil
}

func (w *world) diagramIsTheMember(name, member string) error {
	for _, shape := range w.diagramShapes().listed {
		if shape.diagram.Names["en"] == name {
			if shape.shape != member {
				return fmt.Errorf("%q is the member %q, want %q", name, shape.shape, member)
			}
			return nil
		}
	}
	return fmt.Errorf("%q is not among the listed shapes", name)
}

func (w *world) theyHaveDifferentItemKeys() error {
	listed := w.diagramShapes().listed
	if len(listed) != 2 || listed[0].itemKey() == listed[1].itemKey() {
		return fmt.Errorf("the listed shapes don't have two different item keys")
	}
	return nil
}

func (w *world) diagramIsNotAmongThem(name string) error {
	s := w.diagramShapes()
	if !slices.ContainsFunc(s.catalog, func(d catalogDiagram) bool { return d.Names["en"] == name }) {
		return fmt.Errorf("the catalog has no diagram called %q", name)
	}
	if slices.ContainsFunc(s.listed, func(shape catalogShape) bool { return shape.diagram.Names["en"] == name }) {
		return fmt.Errorf("%q is a shape", name)
	}
	return nil
}

// catalogListingFamilyPattern declares the installed drill catalog plus one
// more family.
func (w *world) catalogListingFamilyPattern(family, pattern string) error {
	s := w.diagramShapes()
	s.pending = append(slices.Clone(s.families), shapeFamilyEntry{Family: family, Diagrams: pattern, Members: []shapeMember{{Shape: "C"}}})
	return nil
}

// catalogListingFamilyMembers declares the installed drill catalog with the
// family's members cut down to the listed ones.
func (w *world) catalogListingFamilyMembers(family, first, second, third, fourth string) error {
	members := []string{first, second, third, fourth}
	s := w.diagramShapes()
	s.pending = slices.Clone(s.families)
	for i, f := range s.pending {
		if f.Family != family {
			continue
		}
		f.Members = nil
		for _, m := range members {
			f.Members = append(f.Members, shapeMember{Shape: m})
		}
		s.pending[i] = f
		return nil
	}
	return fmt.Errorf("the drill catalog has no family %q", family)
}

func (w *world) installationFailsNamingFamily(family string) error {
	return w.installationFailsNaming(fmt.Sprintf("%q", family))
}

func (w *world) installationFailsNamingFamilyAndMember(family, member string) error {
	return w.installationFailsNaming(fmt.Sprintf("%q", family), fmt.Sprintf("shape %q", member))
}

func (w *world) installationFailsNaming(fragments ...string) error {
	err := w.diagramShapes().installErr
	if !errors.Is(err, domain.ErrInvalidDrillCatalog) {
		return fmt.Errorf("installation error = %v, want an invalid drill catalog", err)
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			return fmt.Errorf("installation error %q doesn't name %s", err, fragment)
		}
	}
	return nil
}

// shapeIsAskedAs has alice, who plays guitar, owe a review of the shape on
// her path, with right answers that make drill its next way of asking, and
// composes a session in the head.
func (w *world) shapeIsAskedAs(name, drill string) error {
	if err := w.loadShapeCatalog(); err != nil {
		return err
	}
	shape, err := w.shapeNamed(name)
	if err != nil {
		return err
	}
	if err := w.studentPlaysOne("alice", "guitar"); err != nil {
		return err
	}
	right := map[string]int(nil)
	if drill == string(domain.DiagramShapeDrillFindTheDegree) {
		right = map[string]int{string(domain.DiagramShapeDrillNameTheShape): 1}
	}
	if err := w.shapeDue("alice", shape, right); err != nil {
		return err
	}
	if err := w.composeSession(pickMinutes, nil); err != nil {
		return err
	}
	item, err := w.plannedShape(shape.itemKey())
	if err != nil {
		return err
	}
	if string(item.DiagramShape.Drill) != drill {
		return fmt.Errorf("the shape is asked as %s, want %s", item.DiagramShape.Drill, drill)
	}
	w.diagramShapes().asked = &item
	w.diagramShapes().askedShape = shape
	return nil
}

// shapeIsAsked asks the catalog shape the way a shape never practised is
// asked.
func (w *world) shapeIsAsked(name string) error {
	return w.shapeIsAskedAs(name, string(domain.DiagramShapeDrillNameTheShape))
}

func (w *world) theItemsShapeIs(want string) error {
	asked, err := w.askedShape()
	if err != nil {
		return err
	}
	if asked.DiagramShape.Shape != want {
		return fmt.Errorf("the item's shape is %q, want %q", asked.DiagramShape.Shape, want)
	}
	return nil
}

// theItemIsDrawnOnLayoutInstrument checks the item's layout instrument is
// the asked diagram's, its first linked instrument.
func (w *world) theItemIsDrawnOnLayoutInstrument() error {
	asked, err := w.askedShape()
	if err != nil {
		return err
	}
	layout, err := w.ensureCatalogInstrument(w.diagramShapes().askedShape.diagram.Instruments[0])
	if err != nil {
		return err
	}
	if got := asked.DiagramShape.LayoutInstrumentId.String(); got != layout.ID {
		return fmt.Errorf("the item is drawn on instrument %s, want the diagram's layout instrument %s", got, layout.ID)
	}
	return nil
}

// shapeDue puts the catalog shape on name's path, owing a review after the
// right answers by drill and one wrong one, and remembers it for "the shape"
// steps.
func (w *world) shapeDue(name string, shape catalogShape, right map[string]int) error {
	if err := w.putCatalogShape(shape); err != nil {
		return err
	}
	w.addGuitarPathSkill(name, catalogFamilyClassification[shape.diagram.Family][0])
	counted := 1
	for _, n := range right {
		counted += n
	}
	due := fixedNow.Add(-time.Hour)
	w.practiceStates.put(w.ensureRegistered(name, domain.RoleStudent).String(), domain.PracticeItemState{
		ItemKey: shape.itemKey(), RulesVersion: domain.PracticeRulesVersion, Level: domain.KnowledgeLevelLearning,
		Counted: counted, Box: 1, DueAt: &due, LastAt: &fixedNow, RightByResponse: right,
	})
	w.lastShapeKey = shape.itemKey()
	return nil
}

func (w *world) namedShapeAndFoundOnce(student, name string, named int) error {
	if err := w.loadShapeCatalog(); err != nil {
		return err
	}
	shape, err := w.shapeNamed(name)
	if err != nil {
		return err
	}
	return w.shapeDue(student, shape, map[string]int{string(domain.DiagramShapeDrillNameTheShape): named, string(domain.DiagramShapeDrillFindTheDegree): 1})
}

func (w *world) neverAnsweredShapeRight(student, name string) error {
	if err := w.loadShapeCatalog(); err != nil {
		return err
	}
	shape, err := w.shapeNamed(name)
	if err != nil {
		return err
	}
	return w.shapeDue(student, shape, nil)
}

// plannedShapes lists the composed plan's diagram shapes.
func (w *world) plannedShapes() ([]generated.PracticeSessionItem, error) {
	plan, err := w.composedPlan()
	if err != nil {
		return nil, err
	}
	var shapes []generated.PracticeSessionItem
	for _, item := range plan.Items {
		if item.DiagramShape != nil {
			shapes = append(shapes, item)
		}
	}
	return shapes, nil
}

func (w *world) plannedShape(itemKey string) (generated.PracticeSessionItem, error) {
	shapes, err := w.plannedShapes()
	if err != nil {
		return generated.PracticeSessionItem{}, err
	}
	for _, item := range shapes {
		if item.ItemKey == itemKey {
			return item, nil
		}
	}
	return generated.PracticeSessionItem{}, fmt.Errorf("expected %s in the session, got %d shapes", itemKey, len(shapes))
}

// shapeIsAskedAsDrill checks the drill the remembered shape is asked
// through.
func (w *world) shapeIsAskedAsDrill(drill string) error {
	item, err := w.plannedShape(w.lastShapeKey)
	if err != nil {
		return err
	}
	if string(item.DiagramShape.Drill) != drill {
		return fmt.Errorf("expected %s to be asked as %s, got %s", item.ItemKey, drill, item.DiagramShape.Drill)
	}
	return nil
}

func (w *world) askedShape() (*generated.PracticeSessionItem, error) {
	asked := w.diagramShapes().asked
	if asked == nil {
		return nil, fmt.Errorf("no shape was asked")
	}
	return asked, nil
}

func (w *world) optionNames() ([]string, error) {
	asked, err := w.askedShape()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(asked.DiagramShape.Options))
	for i, o := range asked.DiagramShape.Options {
		names[i] = o.Name
	}
	return names, nil
}

func (w *world) theOptionsAre(first, second, third, fourth, fifth string) error {
	got, err := w.optionNames()
	if err != nil {
		return err
	}
	if want := []string{first, second, third, fourth, fifth}; !slices.Equal(got, want) {
		return fmt.Errorf("the options are %v, want %v", got, want)
	}
	return nil
}

func (w *world) theAskedDegreeIsOneOf(first, second, third, fourth string) error {
	asked, err := w.askedShape()
	if err != nil {
		return err
	}
	degree := asked.DiagramShape.AskedInterval
	if degree == nil || !slices.Contains([]string{first, second, third, fourth}, string(*degree)) {
		return fmt.Errorf("the asked degree is %v, want one of %s, %s, %s or %s", degree, first, second, third, fourth)
	}
	if len(asked.DiagramShape.Options) != 0 {
		return fmt.Errorf("a degree to find offers %d options, want none", len(asked.DiagramShape.Options))
	}
	return nil
}

// theOptionsIncludeMissingMember checks the option is offered although no
// catalog shape of the asked one's family is that member at its root.
func (w *world) theOptionsIncludeMissingMember(option string) error {
	got, err := w.optionNames()
	if err != nil {
		return err
	}
	if !slices.Contains(got, option) {
		return fmt.Errorf("the options %v don't include %q", got, option)
	}
	for _, shape := range w.diagramShapes().shapes {
		if shape.family.Family == "caged-grip" && strings.HasPrefix(shape.diagram.Key, "caged/B/C/") {
			return fmt.Errorf("the catalog has a C-shape grip of B: %s", shape.diagram.Key)
		}
	}
	return nil
}

// itSuitsOnly checks the listed shape is a practice item on exactly the
// named instruments, among every catalog fretted instrument.
func (w *world) itSuitsOnly(first, second string) error {
	listed := w.diagramShapes().listed
	if len(listed) != 1 {
		return fmt.Errorf("no single shape was listed")
	}
	var suits []string
	for name := range catalogTunings {
		instrument, err := w.ensureCatalogInstrument(name)
		if err != nil {
			return err
		}
		items, err := w.practiceItems.ClassifiedItems(w.ctx(), instrument.ID)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(items, func(item domain.ClassifiedItem) bool { return item.ItemKey == listed[0].itemKey() }) {
			suits = append(suits, name)
		}
	}
	slices.Sort(suits)
	want := []string{first, second}
	slices.Sort(want)
	if !slices.Equal(suits, want) {
		return fmt.Errorf("the shape suits %v, want %v", suits, want)
	}
	return nil
}

// onlyItemsOfSkillAreShapes keeps count of the skill's catalog shapes and
// uninstalls the rest, so they are the skill's only items on guitar.
func (w *world) onlyItemsOfSkillAreShapes(skill string, count int) error {
	s := w.diagramShapes()
	s.listed = nil
	for _, shape := range s.shapes {
		if catalogFamilyClassification[shape.diagram.Family][0] != skill {
			continue
		}
		if len(s.listed) < count {
			s.listed = append(s.listed, shape)
			continue
		}
		w.diagrams.mu.Lock()
		delete(w.diagrams.byID, shape.diagram.DiagramID)
		w.diagrams.mu.Unlock()
	}
	if len(s.listed) != count {
		return fmt.Errorf("skill %q has %d catalog shapes, not %d", skill, len(s.listed), count)
	}
	return nil
}

func (w *world) isFluentOnShapes(name string, count int) error {
	listed := w.diagramShapes().listed
	if count > len(listed) {
		return fmt.Errorf("only %d shapes are the skill's items, not %d", len(listed), count)
	}
	items := make([]domain.ClassifiedItem, count)
	for i, shape := range listed[:count] {
		items[i] = domain.ClassifiedItem{ItemKey: shape.itemKey()}
	}
	w.holdCells(name, items, domain.KnowledgeLevelFluent)
	return nil
}

func (w *world) levelForNodeIsDerived(name, nodeName, instrument string) error {
	standing, err := w.standingOn(name, nodeName, instrument)
	if err != nil {
		return err
	}
	w.diagramShapes().standing = standing
	return nil
}

func (w *world) derivedLevelIs(want string) error {
	level := w.diagramShapes().standing.Level
	if level == nil {
		return fmt.Errorf("no level was derived, want %q", want)
	}
	if string(*level) != want {
		return fmt.Errorf("the derived level is %q, want %q", *level, want)
	}
	return nil
}

// hasNewShapesOfSkill puts every catalog shape classified under skill on
// name's path, never practised.
func (w *world) hasNewShapesOfSkill(name, skill string) error {
	if err := w.loadShapeCatalog(); err != nil {
		return err
	}
	w.addGuitarPathSkill(name, skill)
	found := 0
	for _, shape := range w.diagramShapes().shapes {
		if catalogFamilyClassification[shape.diagram.Family][0] == skill {
			if err := w.putCatalogShape(shape); err != nil {
				return err
			}
			found++
		}
	}
	if found == 0 {
		return fmt.Errorf("the catalog has no shapes of skill %q", skill)
	}
	return nil
}

// addGuitarPathSkill gives name a guitar path teaching skill: catalog
// shapes are guitar shapes.
func (w *world) addGuitarPathSkill(name, skill string) {
	w.addPathSkill(name, skill)
	w.paths.put(domain.LearningPath{ID: pathID("practice-path-" + skill).String(), InstrumentIDs: []string{instrumentID("guitar").String()}})
}

// shapeDiagram is the installed diagram the planned shape item asks.
func (w *world) shapeDiagram(item generated.PracticeSessionItem) (domain.Diagram, error) {
	return w.diagrams.GetByID(w.ctx(), item.DiagramShape.DiagramId.String())
}

func (w *world) everyShapeIsLinkedTo(instrument string) error {
	shapes, err := w.plannedShapes()
	if err != nil {
		return err
	}
	for _, item := range shapes {
		d, err := w.shapeDiagram(item)
		if err != nil {
			return err
		}
		if !slices.Contains(d.InstrumentIDs, instrumentID(instrument).String()) {
			return fmt.Errorf("shape %s isn't linked to %q", item.ItemKey, instrument)
		}
	}
	return nil
}

func (w *world) sessionIncludesShapesLinkedTo(instrument string) error {
	shapes, err := w.plannedShapes()
	if err != nil {
		return err
	}
	if len(shapes) == 0 {
		return fmt.Errorf("expected diagram shapes in the session")
	}
	return w.everyShapeIsLinkedTo(instrument)
}

// noShapeLinkedOnlyToUnplayed checks every planned shape is linked to an
// instrument the session covers: one of the Background's path instruments.
func (w *world) noShapeLinkedOnlyToUnplayed(string) error {
	shapes, err := w.plannedShapes()
	if err != nil {
		return err
	}
	played := []string{instrumentID("guitar").String(), instrumentID("electric-bass").String()}
	for _, item := range shapes {
		d, err := w.shapeDiagram(item)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(d.InstrumentIDs, func(id string) bool { return slices.Contains(played, id) }) {
			return fmt.Errorf("shape %s is linked only to %v", item.ItemKey, d.InstrumentIDs)
		}
	}
	return nil
}

func (w *world) everyShapeEstimatedAt(seconds int) error {
	shapes, err := w.plannedShapes()
	if err != nil {
		return err
	}
	if len(shapes) == 0 {
		return fmt.Errorf("expected diagram shapes in the session")
	}
	for _, item := range shapes {
		if item.EstimatedSeconds != seconds {
			return fmt.Errorf("expected %s at %d seconds, got %d", item.ItemKey, seconds, item.EstimatedSeconds)
		}
	}
	return nil
}

// nextSessionPractisesOnlyShapes swaps the Background's fretboard cells for
// a guitar shape of the path's skill.
func (w *world) nextSessionPractisesOnlyShapes(string) error {
	clear(w.practiceItems.cells)
	w.putShape("tap-check-grip", "guitar", w.tapCheckSkill)
	return nil
}

// putShape seeds a drill shape on instrument, classified under skill: the
// A grip of a two-member family, with a root, a 3 and a 5, without
// playback.
func (w *world) putShape(slug, instrument, skill string) {
	names := func(en string) domain.LocalizedText { return domain.LocalizedText{"en": en} }
	at := func(n int) *int { return &n }
	w.diagrams.put(domain.Diagram{
		ID: diagramID(slug).String(), InstrumentID: instrumentID(instrument).String(), InstrumentIDs: []string{instrumentID(instrument).String()},
		Names: names(slug), Kind: domain.DiagramKindBasic, CreatedBy: w.curatorID(), CreatedAt: fixedNow,
		Skills: []domain.KnowledgeNode{{ID: w.skillIDFor(skill).String()}},
		Positions: []domain.Position{
			{ID: slug + "-r", Interval: "R", String: at(5), Fret: at(3), Shape: domain.PositionShapeDot},
			{ID: slug + "-3", Interval: "3", String: at(2), Fret: at(5), Shape: domain.PositionShapeDot},
			{ID: slug + "-5", Interval: "5", String: at(4), Fret: at(5), Shape: domain.PositionShapeDot},
		},
		Shape: &domain.DiagramShape{Shape: "A", Family: domain.DiagramShapeFamily{
			ID: deterministicUUID("diagram-shape-family", "two-grips").String(), Key: "two-grips", Names: names("Two grips"),
			Members: []domain.DiagramShapeMember{{Shape: "C", Names: names("C shape")}, {Shape: "A", Names: names("A shape")}},
		}},
	})
}
