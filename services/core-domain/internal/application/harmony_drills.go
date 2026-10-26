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
	// guitar keeps the guitar layout for one composition; nil reads it each
	// time.
	guitar *guitarLayoutRead
}

// WithHarmonyDrills installs the harmony and ear drills: diatonic chords,
// chord functions, chart progressions, heard chords and heard
// progressions, asked in the head and read as chord boxes on guitar and
// chord symbols on bass.
func (s *PracticeSessionService) WithHarmonyDrills(progressions ports.ProgressionRepository, chords ports.ChordCatalogRepository, layouts ports.FretboardLayoutReader) *PracticeSessionService {
	s.harmony = &harmonyDrills{progressions: progressions, chords: chords, layouts: layouts}
	return s.WithDrillSources(harmonySource{progressions: progressions})
}

// harmonySource finds the harmony drill items the knowledge map classifies
// under a skill, the new keys in the order they are introduced.
type harmonySource struct {
	progressions ports.ProgressionRepository
}

func (src harmonySource) composing(s *PracticeSessionService) DrillSource {
	return harmonySource{progressions: s.harmony.progressions}
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
	return items, nil
}

func (src harmonySource) Item(ctx context.Context, itemKey string) (domain.DrillItem, bool, error) {
	var catalog []domain.Progression
	return src.item(ctx, itemKey, &catalog)
}

// item reads itemKey as a harmony or ear item, loading the progression
// catalog into catalog the first time a progression item needs it. A
// heard progression is only one of 4 bars.
func (src harmonySource) item(ctx context.Context, itemKey string, catalog *[]domain.Progression) (domain.DrillItem, bool, error) {
	if item, ok := domain.ParseDiatonicChordItemKey(itemKey); ok {
		return item, true, nil
	}
	if item, ok := domain.ParseChordFunctionItemKey(itemKey); ok {
		return item, true, nil
	}
	if item, ok := domain.ParseHeardChordItemKey(itemKey); ok {
		return item, true, nil
	}
	if key, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindChartProgression)+":"); ok {
		p, found, err := src.progression(ctx, key, catalog)
		if !found || err != nil {
			return nil, false, err
		}
		return domain.ChartProgressionItem{Progression: p, Catalog: *catalog}, true, nil
	}
	if key, ok := strings.CutPrefix(itemKey, string(domain.PracticeItemKindHeardProgression)+":"); ok {
		p, found, err := src.progression(ctx, key, catalog)
		if !found || err != nil || len(p.Bars) != domain.HeardProgressionBars {
			return nil, false, err
		}
		return domain.HeardProgressionItem{Progression: p, Catalog: *catalog}, true, nil
	}
	return nil, false, nil
}

// progression finds the progression with key in the catalog, loading it
// into catalog the first time; false when the catalog has none.
func (src harmonySource) progression(ctx context.Context, key string, catalog *[]domain.Progression) (domain.Progression, bool, error) {
	if *catalog == nil {
		list, err := src.progressions.List(ctx)
		if err != nil {
			return domain.Progression{}, false, err
		}
		*catalog = list
	}
	i := slices.IndexFunc(*catalog, func(p domain.Progression) bool { return p.Key == key })
	if i < 0 {
		return domain.Progression{}, false, nil
	}
	return (*catalog)[i], true, nil
}

// reading is how a session's harmony questions draw chords, and the bass
// whose voice plays a root line under its questions asked by ear. Only a
// session in the head asks them, so it reads the student's fretboard
// layouts: boxes on the guitar layout (the chord catalog's), symbols and a
// bass line on the first other; a student with both takes turns, boxes on
// an even turn. Symbols and no bass line when the student has no fretboard
// layout at all.
func (h *harmonyDrills) reading(ctx context.Context, instrumentIDs []string, oddTurn bool) (domain.GeneratedChordDrawing, string, error) {
	guitar, err := h.guitarLayoutID(ctx)
	if err != nil {
		return "", "", err
	}
	var onGuitar bool
	var bass string
	for _, id := range instrumentIDs {
		layouts, err := h.layouts.FretboardLayouts(ctx, id)
		if err != nil {
			return "", "", err
		}
		for _, l := range layouts {
			if l.ID == guitar {
				onGuitar = true
			} else if bass == "" {
				bass = l.ID
			}
		}
	}
	if onGuitar && (bass == "" || !oddTurn) {
		return domain.GeneratedChordBoxes, "", nil
	}
	return domain.GeneratedChordSymbols, bass, nil
}

// guitarLayoutID is the layout the chord catalog's voicings are on, read
// from C major's; empty when the catalog has none.
func (h *harmonyDrills) guitarLayoutID(ctx context.Context) (string, error) {
	if h.guitar == nil {
		return h.readGuitarLayoutID(ctx)
	}
	h.guitar.once.Do(func() { h.guitar.id, h.guitar.err = h.readGuitarLayoutID(ctx) })
	return h.guitar.id, h.guitar.err
}

func (h *harmonyDrills) readGuitarLayoutID(ctx context.Context) (string, error) {
	c, err := h.chords.FindChord(ctx, 0, domain.ChordQualityMajor, nil)
	if errors.Is(err, domain.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(c.Voicings) == 0 {
		return "", nil
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

// failure is the error a chord box lookup kept, if any; nil when no boxes
// were looked up.
func (b *chordBoxes) failure() error {
	if b == nil {
		return nil
	}
	return b.err
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
