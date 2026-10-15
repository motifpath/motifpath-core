package application

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// harmonyDrills is what the harmony drills read: the progression catalog,
// the chord catalog the guitar reading draws boxes from, and the
// fretboard layouts that tell guitar from bass.
type harmonyDrills struct {
	progressions ports.ProgressionRepository
	chords       ports.ChordCatalogRepository
	layouts      ports.FretboardLayoutReader
}

// WithHarmonyDrills installs the harmony drills: diatonic chords, chord
// functions and chart progressions, asked in the head and read as chord
// boxes on guitar and chord symbols on bass.
func (s *PracticeSessionService) WithHarmonyDrills(progressions ports.ProgressionRepository, chords ports.ChordCatalogRepository, layouts ports.FretboardLayoutReader) *PracticeSessionService {
	s.harmony = &harmonyDrills{progressions: progressions, chords: chords, layouts: layouts}
	return s.WithDrillSources(harmonySource{progressions: progressions})
}

// harmonySource finds the harmony drill items the knowledge map classifies
// under a skill, the new keys in the order they are introduced.
type harmonySource struct {
	progressions ports.ProgressionRepository
}

func (src harmonySource) SkillItems(ctx context.Context, skillID, _ string, _ bool, knowledge KnowledgeMap) ([]domain.DrillItem, error) {
	var items []domain.DrillItem
	var catalog []domain.Progression
	for _, classified := range knowledge.Items {
		if !slices.Contains(classified.NodeIDs, skillID) {
			continue
		}
		item, ok, err := src.item(ctx, classified.ItemKey, &catalog)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, item)
		}
	}
	slices.SortStableFunc(items, func(a, b domain.DrillItem) int { return harmonyOrder(a) - harmonyOrder(b) })
	return items, nil
}

func (src harmonySource) Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error) {
	var catalog []domain.Progression
	return src.item(ctx, itemKey, &catalog)
}

// item reads itemKey as a harmony item, loading the progression catalog
// into catalog the first time a chart progression needs it.
func (src harmonySource) item(ctx context.Context, itemKey string, catalog *[]domain.Progression) (domain.DrillItem, bool, error) {
	if item, ok := domain.ParseDiatonicChordItemKey(itemKey); ok {
		return item, true, nil
	}
	if item, ok := domain.ParseChordFunctionItemKey(itemKey); ok {
		return item, true, nil
	}
	key, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindChartProgression)+":")
	if !ok {
		return nil, false, nil
	}
	if *catalog == nil {
		list, err := src.progressions.List(ctx)
		if err != nil {
			return nil, false, err
		}
		*catalog = list
	}
	i := slices.IndexFunc(*catalog, func(p domain.Progression) bool { return p.Key == key })
	if i < 0 {
		return nil, false, nil
	}
	return domain.ChartProgressionItem{Progression: (*catalog)[i], Catalog: *catalog}, true, nil
}

// harmonyOrder places a numeral item by its key around the circle of
// fifths, then its numeral; a chart keeps its catalog place after them.
func harmonyOrder(item domain.DrillItem) int {
	var key, numeral string
	switch i := item.(type) {
	case domain.DiatonicChordItem:
		key, numeral = i.Key, i.Numeral
	case domain.ChordFunctionItem:
		key, numeral = i.Key, i.Numeral
	default:
		return len(domain.NewHarmonyKeyOrder) * len(domain.ChordNumerals)
	}
	return slices.Index(domain.NewHarmonyKeyOrder, key)*len(domain.ChordNumerals) + slices.Index(domain.ChordNumerals, numeral)
}

// reading is how a session's harmony questions draw chords. Only a session
// in the head asks them, so it reads the student's fretboard layouts:
// boxes on the guitar layout (the chord catalog's), symbols on any other;
// a student with both takes turns, boxes on an even turn. Symbols when the
// student has no fretboard layout at all.
func (h *harmonyDrills) reading(ctx context.Context, instrumentIDs []string, oddTurn bool) (domain.GeneratedChordDrawing, error) {
	guitar, err := h.guitarLayoutID(ctx)
	if err != nil {
		return "", err
	}
	var onGuitar, onOther bool
	for _, id := range instrumentIDs {
		layouts, err := h.layouts.FretboardLayouts(ctx, id)
		if err != nil {
			return "", err
		}
		for _, l := range layouts {
			if l.ID == guitar {
				onGuitar = true
			} else {
				onOther = true
			}
		}
	}
	if onGuitar && (!onOther || !oddTurn) {
		return domain.GeneratedChordBoxes, nil
	}
	return domain.GeneratedChordSymbols, nil
}

// guitarLayoutID is the layout the chord catalog's voicings are on, read
// from C major's; empty when the catalog has none.
func (h *harmonyDrills) guitarLayoutID(ctx context.Context) (string, error) {
	c, err := h.chords.FindChord(ctx, 0, domain.ChordQualityMajor, nil)
	if errors.Is(err, domain.ErrNotFound) || len(c.Voicings) == 0 {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return c.Voicings[0].InstrumentID, nil
}

// chordBoxes finds the box each chord is drawn with, once per chord, and
// keeps the first error the chord catalog gave.
type chordBoxes struct {
	ctx    context.Context
	chords ports.ChordCatalogRepository
	mu     sync.Mutex
	found  map[string]string
	err    error
}

func (h *harmonyDrills) boxes(ctx context.Context) *chordBoxes {
	return &chordBoxes{ctx: ctx, chords: h.chords, found: map[string]string{}}
}

// diagramOf is chord's box diagram; false when it has none or the catalog
// failed, which err then says.
func (b *chordBoxes) diagramOf(chord string) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if id, ok := b.found[chord]; ok {
		return id, id != ""
	}
	reading := domain.ParseChordSymbol(chord)
	if reading.Parsed == nil {
		b.found[chord] = ""
		return "", false
	}
	c, err := b.chords.FindChord(b.ctx, reading.Parsed.RootPitchClass, reading.Parsed.Quality, nil)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		if b.err == nil {
			b.err = err
		}
		return "", false
	}
	voicing, ok := domain.PreferredChordBox(c.Voicings)
	if !ok {
		b.found[chord] = ""
		return "", false
	}
	b.found[chord] = voicing.DiagramID
	return voicing.DiagramID, true
}
