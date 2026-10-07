//go:build integration

package bdd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"

	"github.com/motifpath/core-domain/internal/domain"
)

// drillCatalogFile is the part of motifpath-specs' practice drill catalog
// the fretboard cell and diagram shape steps read: its templates, its cell
// ranges and its shape families.
type drillCatalogFile struct {
	Templates []struct {
		Key      string `yaml:"key"`
		ItemKind string `yaml:"item_kind"`
	} `yaml:"templates"`
	FretboardCells []struct {
		Skill   string `yaml:"skill"`
		Layouts map[string]struct {
			Strings []int `yaml:"strings"`
			Frets   []int `yaml:"frets"`
		} `yaml:"layouts"`
	} `yaml:"fretboard_cells"`
	DiagramShapes []shapeFamilyEntry `yaml:"diagram_shapes"`
}

// fretboardCellWorld is what the fretboard cell steps share in a scenario.
type fretboardCellWorld struct {
	// ranges holds the installed catalog's ranges by skill, then layout.
	ranges map[string]map[string]domain.FretboardCellRange
	// templates lists the catalog's fretboard cell drill templates.
	templates []string
	// cells holds the cells a step generated.
	cells []domain.ClassifiedItem
	// pending is a catalog entry a scenario declares, checked on install.
	pending     *domain.FretboardCellRange
	pendingNode domain.KnowledgeNode
	installErr  error
}

// catalogTunings are the catalog instruments' strings, lowest first, so a
// cell range is checked against the real string counts.
var catalogTunings = map[string][]string{
	"guitar":          {"E2", "A2", "D3", "G3", "B3", "E4"},
	"electric-guitar": {"E2", "A2", "D3", "G3", "B3", "E4"},
	"electric-bass":   {"E1", "A1", "D2", "G2"},
}

func registerFretboardCellSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the practice drill catalog is installed$`, w.practiceDrillCatalogIsInstalled)
	sc.Step(`^the cells for skill "([^"]+)" on the "([^"]+)" layout are generated$`, w.cellsGeneratedOnLayout)
	sc.Step(`^the cells for skill "([^"]+)" are generated for "([^"]+)"$`, w.cellsGeneratedForInstrument)
	sc.Step(`^the cells for skills "([^"]+)" and "([^"]+)" on the "([^"]+)" layout are generated$`, w.cellsForTwoSkillsGenerated)
	sc.Step(`^a cell of skill "([^"]+)" is picked for practice$`, w.cellOfSkillPicked)
	sc.Step(`^there are (\d+) cells, on strings (\d+) and (\d+) at frets (\d+) to (\d+)$`, w.thereAreCellsOnStrings)
	sc.Step(`^each cell's item key names the "([^"]+)" layout instrument, its string and its fret$`, w.eachCellKeyNamesLayout)
	sc.Step(`^they are the cells of the "([^"]+)" layout, with the same item keys$`, w.theyAreTheLayoutsCells)
	sc.Step(`^there are (\d+) cells and no cell belongs to both skills$`, w.cellsBelongToOneSkill)
	sc.Step(`^it can be asked through "([^"]+)" or "([^"]+)"$`, w.cellCanBeAskedThrough)
	sc.Step(`^a drill catalog listing string (\d+) for skill "([^"]+)" on the "([^"]+)" layout$`, w.catalogListingString)
	sc.Step(`^a drill catalog listing a guitar-only skill on the "([^"]+)" layout$`, w.catalogListingGuitarOnlySkill)
	sc.Step(`^the drill catalog is installed$`, w.drillCatalogIsInstalled)
	sc.Step(`^the installation fails, naming the "([^"]+)" layout and string (\d+)$`, w.installationFailsNamingString)
	sc.Step(`^the installation fails, naming the skill and the "([^"]+)" layout$`, w.installationFailsNamingSkill)
}

func (w *world) fretboard() *fretboardCellWorld {
	if w.fretboardCells == nil {
		w.fretboardCells = &fretboardCellWorld{ranges: map[string]map[string]domain.FretboardCellRange{}}
	}
	return w.fretboardCells
}

// seedCatalogInstrument seeds one of the catalog's fretted instruments with
// its real strings and tuning.
func (w *world) seedCatalogInstrument(name string) (domain.Instrument, error) {
	tuning, ok := catalogTunings[name]
	if !ok {
		return domain.Instrument{}, fmt.Errorf("%q is not a catalog fretted instrument", name)
	}
	count := len(tuning)
	instrument := domain.Instrument{
		ID: instrumentID(name).String(), Names: domain.LocalizedText{"en": name}, Family: domain.InstrumentFamilyFretted,
		StringCount: &count, Tuning: tuning, DefaultVoiceID: defaultVoiceOf[domain.InstrumentFamilyFretted],
	}
	w.instruments.put(instrument)
	return instrument, nil
}

// practiceDrillCatalogIsInstalled reads motifpath-specs' drill catalog and
// installs its cell ranges, each checked against its layout instrument, as
// the practice items of their layouts.
func (w *world) practiceDrillCatalogIsInstalled() error {
	var file drillCatalogFile
	if err := readDrillCatalog(&file); err != nil {
		return err
	}
	w.installShapeFamilies(file)
	f := w.fretboard()
	for _, t := range file.Templates {
		if t.ItemKind == string(domain.PracticeItemKindFretboardCell) {
			f.templates = append(f.templates, t.Key)
		}
	}
	for _, entry := range file.FretboardCells {
		for layoutName, spec := range entry.Layouts {
			layout, err := w.seedCatalogInstrument(layoutName)
			if err != nil {
				return err
			}
			if len(spec.Frets) != 2 {
				return fmt.Errorf("skill %q on %q needs frets [from, to]", entry.Skill, layoutName)
			}
			r := domain.FretboardCellRange{
				SkillID: w.skillIDFor(entry.Skill).String(), LayoutInstrumentID: layout.ID,
				Strings: spec.Strings, FromFret: spec.Frets[0], ToFret: spec.Frets[1],
			}
			if err := r.CheckFits(layout, domain.KnowledgeNode{ID: r.SkillID}); err != nil {
				return err
			}
			if f.ranges[entry.Skill] == nil {
				f.ranges[entry.Skill] = map[string]domain.FretboardCellRange{}
			}
			f.ranges[entry.Skill][layoutName] = r
			w.practiceItems.cells[layout.ID] = append(w.practiceItems.cells[layout.ID], r.Items()...)
			w.practiceItems.layouts[layout.ID] = layout.ID
		}
	}
	return nil
}

// readDrillCatalog reads motifpath-specs' practice drill catalog into file.
func readDrillCatalog(file *drillCatalogFile) error {
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(featuresBase), "catalogs", "practice-drills.yaml"))
	if err != nil {
		return err
	}
	return yaml.Unmarshal(raw, file)
}

func (w *world) installedRange(skill, layout string) (domain.FretboardCellRange, error) {
	r, ok := w.fretboard().ranges[skill][layout]
	if !ok {
		return domain.FretboardCellRange{}, fmt.Errorf("the catalog has no cells for %q on the %q layout", skill, layout)
	}
	return r, nil
}

func (w *world) cellsGeneratedOnLayout(skill, layout string) error {
	r, err := w.installedRange(skill, layout)
	if err != nil {
		return err
	}
	w.fretboard().cells = r.Items()
	return nil
}

// cellsGeneratedForInstrument generates the skill's cells for an instrument
// from the ranges on every layout of the same geometry, as practice does.
func (w *world) cellsGeneratedForInstrument(skill, instrumentName string) error {
	played, err := w.seedCatalogInstrument(instrumentName)
	if err != nil {
		return err
	}
	f := w.fretboard()
	f.cells = nil
	for _, r := range f.ranges[skill] {
		layout, err := w.instruments.GetByID(w.ctx(), r.LayoutInstrumentID)
		if err != nil {
			return err
		}
		if played.SameGeometry(layout) {
			f.cells = append(f.cells, r.Items()...)
		}
	}
	return nil
}

func (w *world) cellsForTwoSkillsGenerated(first, second, layout string) error {
	f := w.fretboard()
	f.cells = nil
	for _, skill := range []string{first, second} {
		r, err := w.installedRange(skill, layout)
		if err != nil {
			return err
		}
		f.cells = append(f.cells, r.Items()...)
	}
	return nil
}

func (w *world) cellOfSkillPicked(skill string) error {
	r, err := w.installedRange(skill, "guitar")
	if err != nil {
		return err
	}
	w.fretboard().cells = r.Items()[:1]
	return nil
}

// cellCoordinates parses a fretboard cell item key into its layout
// instrument, string and fret.
func cellCoordinates(key string) (string, int, int, error) {
	parts := strings.Split(key, ":")
	if len(parts) != 4 || parts[0] != string(domain.PracticeItemKindFretboardCell) {
		return "", 0, 0, fmt.Errorf("%q is not a fretboard cell item key", key)
	}
	str, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, 0, err
	}
	fret, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", 0, 0, err
	}
	return parts[1], str, fret, nil
}

func (w *world) thereAreCellsOnStrings(count, first, second, fromFret, toFret int) error {
	cells := w.fretboard().cells
	if len(cells) != count {
		return fmt.Errorf("got %d cells, want %d", len(cells), count)
	}
	for _, cell := range cells {
		_, str, fret, err := cellCoordinates(cell.ItemKey)
		if err != nil {
			return err
		}
		if (str != first && str != second) || fret < fromFret || fret > toFret {
			return fmt.Errorf("cell %q is outside strings %d and %d, frets %d to %d", cell.ItemKey, first, second, fromFret, toFret)
		}
	}
	return nil
}

func (w *world) eachCellKeyNamesLayout(layout string) error {
	for _, cell := range w.fretboard().cells {
		layoutID, str, fret, err := cellCoordinates(cell.ItemKey)
		if err != nil {
			return err
		}
		if layoutID != instrumentID(layout).String() || cell.ItemKey != domain.FretboardCellItemKey(layoutID, str, fret) {
			return fmt.Errorf("cell %q doesn't name the %q layout instrument", cell.ItemKey, layout)
		}
	}
	return nil
}

func (w *world) theyAreTheLayoutsCells(layout string) error {
	f := w.fretboard()
	var want []string
	for skill := range f.ranges {
		if r, ok := f.ranges[skill][layout]; ok && len(f.cells) > 0 && slices.Equal(r.Items()[0].NodeIDs, f.cells[0].NodeIDs) {
			for _, item := range r.Items() {
				want = append(want, item.ItemKey)
			}
		}
	}
	got := make([]string, len(f.cells))
	for i, cell := range f.cells {
		got[i] = cell.ItemKey
	}
	slices.Sort(want)
	slices.Sort(got)
	if len(want) == 0 || !slices.Equal(want, got) {
		return fmt.Errorf("got cells %v, want the %q layout's %v", got, layout, want)
	}
	return nil
}

func (w *world) cellsBelongToOneSkill(count int) error {
	cells := w.fretboard().cells
	seen := map[string]bool{}
	for _, cell := range cells {
		if seen[cell.ItemKey] {
			return fmt.Errorf("cell %q belongs to both skills", cell.ItemKey)
		}
		seen[cell.ItemKey] = true
	}
	if len(cells) != count {
		return fmt.Errorf("got %d cells, want %d", len(cells), count)
	}
	return nil
}

// cellCanBeAskedThrough checks the drill templates of the item picked for
// practice: a diagram shape when one was picked, a fretboard cell otherwise.
func (w *world) cellCanBeAskedThrough(first, second string) error {
	f := w.fretboard()
	var got []string
	switch {
	case w.shapeWorld != nil && w.shapeWorld.picked != nil:
		got = slices.Clone(w.shapeWorld.templates)
	case len(f.cells) == 1:
		got = slices.Clone(f.templates)
	default:
		return fmt.Errorf("no cell or shape was picked")
	}
	want := []string{first, second}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return fmt.Errorf("the picked item is asked through %v, want %v", got, want)
	}
	return nil
}

func (w *world) catalogListingString(str int, skill, layout string) error {
	f := w.fretboard()
	f.pending = &domain.FretboardCellRange{
		SkillID: w.skillIDFor(skill).String(), LayoutInstrumentID: instrumentID(layout).String(),
		Strings: []int{str}, FromFret: 0, ToFret: 11,
	}
	f.pendingNode = domain.KnowledgeNode{ID: f.pending.SkillID}
	return nil
}

func (w *world) catalogListingGuitarOnlySkill(layout string) error {
	f := w.fretboard()
	skillID := w.skillIDFor("guitar-only-skill").String()
	f.pending = &domain.FretboardCellRange{
		SkillID: skillID, LayoutInstrumentID: instrumentID(layout).String(), Strings: []int{1}, FromFret: 0, ToFret: 11,
	}
	f.pendingNode = domain.KnowledgeNode{ID: skillID, InstrumentIDs: []string{instrumentID("guitar").String(), instrumentID("electric-guitar").String()}}
	return nil
}

func (w *world) drillCatalogIsInstalled() error {
	if s := w.shapeWorld; s != nil && s.pending != nil {
		_, s.installErr = matchShapes(s.pending, s.catalog)
		return nil
	}
	f := w.fretboard()
	if f.pending == nil {
		return fmt.Errorf("the scenario declared no catalog entry")
	}
	layout, err := w.instruments.GetByID(w.ctx(), f.pending.LayoutInstrumentID)
	if err != nil {
		return err
	}
	f.installErr = f.pending.CheckFits(layout, f.pendingNode)
	return nil
}

func (w *world) installationFailsNamingString(layout string, str int) error {
	err := w.fretboard().installErr
	if !errors.Is(err, domain.ErrInvalidDrillCatalog) {
		return fmt.Errorf("installation error = %v, want an invalid drill catalog", err)
	}
	if !strings.Contains(err.Error(), instrumentID(layout).String()) || !strings.Contains(err.Error(), fmt.Sprintf("string %d", str)) {
		return fmt.Errorf("installation error %q doesn't name the %q layout and string %d", err, layout, str)
	}
	return nil
}

func (w *world) installationFailsNamingSkill(layout string) error {
	f := w.fretboard()
	err := f.installErr
	if !errors.Is(err, domain.ErrInvalidDrillCatalog) {
		return fmt.Errorf("installation error = %v, want an invalid drill catalog", err)
	}
	if !strings.Contains(err.Error(), f.pending.SkillID) || !strings.Contains(err.Error(), instrumentID(layout).String()) {
		return fmt.Errorf("installation error %q doesn't name the skill and the %q layout", err, layout)
	}
	return nil
}
