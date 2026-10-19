package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// fakeFretboardLayouts is a ports.FretboardLayoutReader: the fretboard
// layouts each instrument shares its geometry with.
type fakeFretboardLayouts map[string][]domain.Instrument

func (f fakeFretboardLayouts) FretboardLayouts(_ context.Context, instrumentID string) ([]domain.Instrument, error) {
	return f[instrumentID], nil
}

// harmonyChordCatalog holds every triad and dominant seventh in every key,
// each with one voicing on the guitar layout drawn as "box-<symbol>".
func harmonyChordCatalog() *fakeChordCatalogRepository {
	qualities := map[string]domain.ChordQuality{"": domain.ChordQualityMajor, "m": domain.ChordQualityMinor, "dim": domain.ChordQualityDiminished, "7": domain.ChordQualityDominant7}
	catalog := &fakeChordCatalogRepository{}
	for pc, root := range domain.MusicKeys {
		for suffix, quality := range qualities {
			symbol := root + suffix
			catalog.chords = append(catalog.chords, domain.ChordDefinition{
				ID: "chord-" + symbol, CanonicalSymbol: symbol, Root: root, RootPitchClass: pc, Quality: quality,
				Voicings: []domain.ChordVoicing{{ID: "voicing-" + symbol, DiagramID: "box-" + symbol, InstrumentID: practiceGuitar, Status: domain.ChordVoicingActive}},
			})
		}
	}
	return catalog
}

var harmonyProgressions = fakeProgressions{
	{Key: "I-V-vi-IV", Bars: []string{"I", "V", "vi", "IV"}, Confusable: []string{"I-vi-IV-V"}, NodeIDs: []string{"skill-progressions"}, Names: domain.LocalizedText{"en": "I–V–vi–IV (pop)"}},
	{Key: "I-vi-IV-V", Bars: []string{"I", "vi", "IV", "V"}, Confusable: []string{"I-V-vi-IV"}, NodeIDs: []string{"skill-progressions"}, Names: domain.LocalizedText{"en": "I–vi–IV–V (doo-wop)"}},
}

// harmonyFixture is a practice fixture with the harmony drills installed,
// diatonic chords of every key under skill-diatonic and the progressions
// under skill-charts, classified for both instruments in reverse key order
// so the source has to order new keys itself.
func harmonyFixture(t *testing.T, plays ...string) *practiceFixture {
	f := newPracticeFixture(t)
	f.skill("skill-diatonic")
	f.skill("skill-charts")
	f.onPathFor(plays, "skill-diatonic", "skill-charts")
	f.svc.WithHarmonyDrills(harmonyProgressions, harmonyChordCatalog(), fakeFretboardLayouts{
		practiceGuitar: {{ID: practiceGuitar}},
		practiceBass:   {{ID: practiceBass}},
	})
	f.templates.extra = []domain.DrillTemplate{
		{Key: "diatonic_chord:numeral_to_chord", ItemKind: domain.PracticeItemKindDiatonicChord, Rung: domain.PracticeRungKnow, Timed: true},
		{Key: "diatonic_chord:chord_to_numeral", ItemKind: domain.PracticeItemKindDiatonicChord, Rung: domain.PracticeRungKnow, Timed: true},
		{Key: "chart_progression:chart_to_numbers", ItemKind: domain.PracticeItemKindChartProgression, Rung: domain.PracticeRungKnow, Timed: true},
		{Key: "chart_progression:numbers_to_chart", ItemKind: domain.PracticeItemKindChartProgression, Rung: domain.PracticeRungKnow, Timed: true},
	}
	var items []domain.ClassifiedItem
	for _, key := range slices.Backward(domain.MusicKeys) {
		for _, numeral := range domain.HarmonyItemNumerals {
			items = append(items, domain.ClassifiedItem{ItemKey: domain.DiatonicChordItem{Key: key, Numeral: numeral}.ItemKey(), NodeIDs: []string{"skill-diatonic"}})
		}
	}
	for _, p := range harmonyProgressions {
		items = append(items, domain.ClassifiedItem{ItemKey: domain.ChartProgressionItemKey(p.Key), NodeIDs: []string{"skill-charts"}})
	}
	for _, id := range []string{practiceGuitar, practiceBass} {
		f.cells[id] = append(f.cells[id], items...)
	}
	return f
}

func TestPracticeSessionService_ComposePlanHarmonyDrills(t *testing.T) {
	t.Run("a session in the head asks diatonic chords as generated questions", func(t *testing.T) {
		f := harmonyFixture(t, practiceGuitar)

		plan := f.composeInTheHead(t, 10)

		items := itemsOfKind(plan, domain.PracticeItemKindDiatonicChord)
		require.NotEmpty(t, items)
		for _, item := range items {
			require.NotNil(t, item.GeneratedChoice, item.ItemKey)
			assert.Equal(t, item.Way, item.GeneratedChoice.Way())
			assert.Equal(t, "diatonic_chord:numeral_to_chord", item.DrillTemplateKey(), "never answered, it is asked numeral to chord first")
			assert.Equal(t, domain.HarmonyItemSeconds, item.EstimatedSeconds)
		}
	})

	t.Run("new diatonic chords come key by key around the circle of fifths, however new items are shuffled", func(t *testing.T) {
		f := harmonyFixture(t, practiceGuitar)
		f.svc.WithShuffle(reverseOrder)

		plan := f.composeInTheHead(t, 20)

		var order []string
		for _, key := range domain.NewHarmonyKeyOrder {
			for _, numeral := range domain.HarmonyItemNumerals {
				order = append(order, domain.DiatonicChordItem{Key: key, Numeral: numeral}.ItemKey())
			}
		}
		var fresh []string
		for _, item := range itemsOfKind(plan, domain.PracticeItemKindDiatonicChord) {
			fresh = append(fresh, item.ItemKey)
		}
		require.NotEmpty(t, fresh)
		slices.SortFunc(fresh, func(a, b string) int { return slices.Index(order, a) - slices.Index(order, b) })
		assert.Equal(t, order[:len(fresh)], fresh, "the new chords picked are the first ones in key order")
		assert.True(t, strings.HasPrefix(fresh[0], "diatonic_chord:C:"))
	})

	t.Run("a due chart progression is offered, asked in a key", func(t *testing.T) {
		f := harmonyFixture(t, practiceGuitar)
		chart := domain.ChartProgressionItemKey("I-V-vi-IV")
		f.stateOf(chart, domain.PracticeItemState{
			Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(1),
			RightByResponse: map[string]int{"chart_to_numbers": 2},
		})

		plan := f.composeInTheHead(t, 10)

		i := slices.IndexFunc(plan.Items, func(item domain.PracticeSessionItem) bool { return item.ItemKey == chart })
		require.GreaterOrEqual(t, i, 0)
		item := plan.Items[i]
		assert.Equal(t, domain.PracticePickDue, item.Reason)
		assert.Equal(t, "chart_progression:numbers_to_chart", item.DrillTemplateKey())
		require.NotNil(t, item.GeneratedChoice)
		assert.Contains(t, domain.EasyChartKeys, item.GeneratedChoice.MusicKey)
	})
}

func TestPracticeSessionService_ComposePlanHarmonyReading(t *testing.T) {
	readingOf := func(t *testing.T, plan domain.PracticeSessionPlan) domain.GeneratedChordDrawing {
		t.Helper()
		items := itemsOfKind(plan, domain.PracticeItemKindDiatonicChord)
		require.NotEmpty(t, items)
		reading := items[0].GeneratedChoice.DrawnAs
		for _, item := range items {
			assert.Equal(t, reading, item.GeneratedChoice.DrawnAs, "every harmony question of a session reads chords the same way")
		}
		return reading
	}

	tests := []struct {
		name    string
		plays   []string
		started int
		want    domain.GeneratedChordDrawing
	}{
		{name: "a guitarist reads chord boxes", plays: []string{practiceGuitar}, started: 3, want: domain.GeneratedChordBoxes},
		{name: "a bassist reads chord symbols", plays: []string{practiceBass}, started: 2, want: domain.GeneratedChordSymbols},
		{name: "a player of both reads boxes on an even turn in the head", plays: []string{practiceGuitar, practiceBass}, started: 2, want: domain.GeneratedChordBoxes},
		{name: "a player of both reads symbols on an odd turn in the head", plays: []string{practiceGuitar, practiceBass}, started: 3, want: domain.GeneratedChordSymbols},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := harmonyFixture(t, tt.plays...)
			f.headSessions.started[studentCaller().ID] = tt.started

			plan := f.composeInTheHead(t, 10)

			assert.Equal(t, tt.want, readingOf(t, plan))
		})
	}

	t.Run("chord boxes carry the voicing of every chord drawn", func(t *testing.T) {
		f := harmonyFixture(t, practiceGuitar)

		plan := f.composeInTheHead(t, 10)

		for _, item := range itemsOfKind(plan, domain.PracticeItemKindDiatonicChord) {
			q := item.GeneratedChoice
			require.Len(t, q.Voicings, len(q.Options), item.ItemKey)
			for _, box := range q.Voicings {
				assert.Equal(t, "box-"+box.Chord, box.DiagramID)
			}
		}
	})
}

// planningRecorder is a drill item that remembers how it was planned.
type planningRecorder struct {
	fakeDrillItem
	planned *[]domain.DrillPlanning
}

func (r planningRecorder) Plan(way string, planning domain.DrillPlanning) domain.PlannedDrill {
	*r.planned = append(*r.planned, planning)
	return r.fakeDrillItem.Plan(way, planning)
}

type recordingSource struct {
	item planningRecorder
}

func (s recordingSource) SkillItems(_ context.Context, skillID, _ string, _ bool, _ application.KnowledgeMap) ([]domain.DrillItem, error) {
	if skillID != "skill-fake" {
		return nil, nil
	}
	return []domain.DrillItem{s.item}, nil
}

func (s recordingSource) Item(_ context.Context, itemKey string) (domain.DrillItem, bool, error) {
	return s.item, itemKey == s.item.ItemKey(), nil
}

func TestPracticeSessionService_DrillPlanningKnowsTheItemAndTheSession(t *testing.T) {
	f := harmonyFixture(t, practiceBass)
	f.skill("skill-fake")
	f.onPathFor([]string{practiceBass}, "skill-fake")
	var planned []domain.DrillPlanning
	item := planningRecorder{fakeDrillItem: fakeDrillItem{id: "00"}, planned: &planned}
	f.svc.WithDrillSources(recordingSource{item: item})
	f.cells[practiceBass] = append(f.cells[practiceBass], domain.ClassifiedItem{ItemKey: item.ItemKey(), NodeIDs: []string{"skill-fake"}})
	f.templates.extra = append(f.templates.extra, domain.DrillTemplate{Key: "fake_drill:first", ItemKind: fakeDrillKind, Rung: domain.PracticeRungKnow, Timed: true})
	f.stateOf(item.ItemKey(), domain.PracticeItemState{Level: domain.KnowledgeLevelAccurate, Counted: 4, Box: 2, DueAt: f.daysAgo(1)})

	f.composeInTheHead(t, 10)

	require.NotEmpty(t, planned)
	assert.Equal(t, domain.KnowledgeLevelAccurate, planned[0].Level, "the item's level picks its variant")
	assert.Equal(t, domain.GeneratedChordSymbols, planned[0].Reading, "the session's reading")
}

// failingChordCatalog is a chord catalog that can't be read.
type failingChordCatalog struct{ fakeChordCatalogRepository }

func (failingChordCatalog) FindChord(context.Context, int, domain.ChordQuality, *int) (domain.ChordDefinition, error) {
	return domain.ChordDefinition{}, errChordCatalogDown
}

var errChordCatalogDown = errors.New("chord catalog unavailable")

func TestPracticeSessionService_ComposePlanHarmonyCatalogFailure(t *testing.T) {
	f := harmonyFixture(t, practiceGuitar)
	f.svc.WithHarmonyDrills(harmonyProgressions, &failingChordCatalog{}, fakeFretboardLayouts{practiceGuitar: {{ID: practiceGuitar}}})

	_, err := f.svc.ComposePlan(context.Background(), studentCaller(), nil, nil, 10)

	assert.ErrorIs(t, err, errChordCatalogDown, "a catalog outage is an error, never a silent switch to chord symbols")
}
