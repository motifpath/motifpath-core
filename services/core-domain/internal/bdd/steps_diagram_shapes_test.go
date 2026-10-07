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

	"github.com/cucumber/godog"

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
	Family   string `yaml:"family"`
	Diagrams string `yaml:"diagrams"`
	Members  []struct {
		Shape string `yaml:"shape"`
	} `yaml:"members"`
}

// catalogDiagram is the part of a basic-guitar catalog entry the shape steps
// read.
type catalogDiagram struct {
	Key         string            `json:"key"`
	DiagramID   string            `json:"diagram_id"`
	Family      string            `json:"family"`
	Names       map[string]string `json:"names"`
	Instruments []string          `json:"instruments"`
}

// catalogShape is a catalog diagram a family takes: the member it is.
type catalogShape struct {
	diagram catalogDiagram
	family  string
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
	sc.Step(`^it suits "([^"]+)" and "([^"]+)" and no other instrument$`, w.itSuitsOnly)
	sc.Step(`^the only practice items of skill "([^"]+)" on guitar are (\d+) shapes$`, w.onlyItemsOfSkillAreShapes)
	sc.Step(`^"([^"]+)" is fluent on (\d+) of those shapes$`, w.isFluentOnShapes)
	sc.Step(`^"([^"]+)"'s level for "([^"]+)" on "?([^"]+?)"? is derived$`, w.levelForNodeIsDerived)
	sc.Step(`^it is "([^"]+)"$`, w.derivedLevelIs)
	sc.Step(`^a drill catalog listing the family "([^"]+)" with the pattern "([^"]+)"$`, w.catalogListingFamilyPattern)
	sc.Step(`^a drill catalog listing the family "([^"]+)" with only the members "([^"]+)", "([^"]+)", "([^"]+)" and "([^"]+)"$`, w.catalogListingFamilyMembers)
	sc.Step(`^the installation fails, naming the family "([^"]+)"$`, w.installationFailsNamingFamily)
	sc.Step(`^the installation fails, naming the family "([^"]+)" and the member "([^"]+)"$`, w.installationFailsNamingFamilyAndMember)
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

// basicGuitarDiagramCatalogIsInstalled reads the basic-guitar catalog and
// makes every diagram a family takes a practice item of its diagram's
// skill and concept on each instrument the diagram is linked to. The
// diagrams themselves stay out of the diagram fakes: their playbacks would
// make them play-alongs too, which no shape scenario is about.
func (w *world) basicGuitarDiagramCatalogIsInstalled() error {
	raw, err := os.ReadFile(basicGuitarCatalogFile)
	if err != nil {
		return err
	}
	s := w.diagramShapes()
	if err := json.Unmarshal(raw, &s.catalog); err != nil {
		return err
	}
	if s.shapes, err = matchShapes(s.families, s.catalog); err != nil {
		return err
	}
	for _, shape := range s.shapes {
		classification, ok := catalogFamilyClassification[shape.diagram.Family]
		if !ok {
			return fmt.Errorf("no classification for catalog family %q", shape.diagram.Family)
		}
		item := domain.ClassifiedItem{
			ItemKey: shape.itemKey(),
			NodeIDs: []string{w.skillIDFor(classification[0]).String(), w.conceptIDFor(classification[1]).String()},
		}
		for _, name := range shape.diagram.Instruments {
			instrument, err := w.ensureCatalogInstrument(name)
			if err != nil {
				return err
			}
			w.practiceItems.shapes[instrument.ID] = append(w.practiceItems.shapes[instrument.ID], item)
		}
	}
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
			shapes = append(shapes, catalogShape{diagram: d, family: f.Family, shape: shape})
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
		if shape.family == family {
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
		if shape.family == family {
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

// onlyItemsOfSkillAreShapes keeps count of the skill's guitar shapes and
// drops the rest, so they are the skill's only items on guitar.
func (w *world) onlyItemsOfSkillAreShapes(skill string, count int) error {
	guitar := instrumentID("guitar").String()
	skillID := w.skillIDFor(skill).String()
	var kept []domain.ClassifiedItem
	ofSkill := 0
	for _, item := range w.practiceItems.shapes[guitar] {
		if !slices.Contains(item.NodeIDs, skillID) {
			kept = append(kept, item)
			continue
		}
		if ofSkill < count {
			kept = append(kept, item)
			ofSkill++
		}
	}
	if ofSkill != count {
		return fmt.Errorf("skill %q has %d guitar shapes, not %d", skill, ofSkill, count)
	}
	w.practiceItems.shapes[guitar] = kept
	if electric := instrumentID("electric-guitar").String(); w.practiceItems.shapes[electric] != nil {
		w.practiceItems.shapes[electric] = slices.Clone(kept)
	}
	w.diagramShapes().listed = nil
	for _, item := range kept {
		if slices.Contains(item.NodeIDs, skillID) {
			w.diagramShapes().listed = append(w.diagramShapes().listed, catalogShape{diagram: catalogDiagram{DiagramID: strings.TrimPrefix(item.ItemKey, "diagram_shape:")}})
		}
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

// catalogListingFamilyPattern declares the installed drill catalog plus one
// more family.
func (w *world) catalogListingFamilyPattern(family, pattern string) error {
	s := w.diagramShapes()
	s.pending = append(slices.Clone(s.families), shapeFamilyEntry{Family: family, Diagrams: pattern, Members: []struct {
		Shape string `yaml:"shape"`
	}{{Shape: "C"}}})
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
			f.Members = append(f.Members, struct {
				Shape string `yaml:"shape"`
			}{Shape: m})
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
