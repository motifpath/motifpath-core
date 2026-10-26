package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func guideToneItemKey(layout, progression string) string {
	return "guide_tone:" + layout + ":" + progression
}

var guideToneProgressions = fakeProgressions{
	{Key: "ii-V-I", Bars: []string{"ii", "V", "I", "I"}, Names: domain.LocalizedText{"en": "ii–V–I"}},
	{Key: "IV-V-I", Bars: []string{"IV", "V", "I", "I"}, Names: domain.LocalizedText{"en": "IV–V–I"}},
	{Key: "I-V-vi-IV", Bars: []string{"I", "V", "vi", "IV"}, Names: domain.LocalizedText{"en": "I–V–vi–IV (pop)"}},
	{Key: "I-vi-IV-V", Bars: []string{"I", "vi", "IV", "V"}, Names: domain.LocalizedText{"en": "I–vi–IV–V (doo-wop)"}},
	{Key: "12-bar-blues", Bars: []string{"I7", "I7", "I7", "I7", "IV7", "IV7", "I7", "I7", "V7", "IV7", "I7", "V7"}, Names: domain.LocalizedText{"en": "12-bar blues"}},
}

// guideToneFixture is a practice fixture with the guide tones installed: each of
// progressions on the guitar and bass layouts under target-chord-tones, on both
// instruments' paths.
func guideToneFixture(t *testing.T, progressions ...string) *practiceFixture {
	f := newPracticeFixture(t)
	f.instruments.put(domain.Instrument{ID: practiceGuitar, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}})
	f.instruments.put(domain.Instrument{ID: practiceBass, Tuning: []string{"E1", "A1", "D2", "G2"}})
	f.skill(targetChordTones)
	f.onPathFor([]string{practiceGuitar, practiceBass}, targetChordTones)
	for _, p := range progressions {
		f.cells[practiceGuitar] = append(f.cells[practiceGuitar], domain.ClassifiedItem{ItemKey: guideToneItemKey(practiceGuitar, p), NodeIDs: []string{targetChordTones}})
		f.cells[practiceBass] = append(f.cells[practiceBass], domain.ClassifiedItem{ItemKey: guideToneItemKey(practiceBass, p), NodeIDs: []string{targetChordTones}})
	}
	f.svc.WithHarmonyDrills(guideToneProgressions, harmonyChordCatalog(), fakeFretboardLayouts{
		practiceGuitar: {{ID: practiceGuitar}},
		practiceBass:   {{ID: practiceBass}},
	})
	f.svc.WithGuideTones(f.instruments)
	return f
}

const targetChordTones = "target-chord-tones"

func TestPracticeSessionService_ComposePlanGuideTones(t *testing.T) {
	t.Run("a session in the head plans a new guide tone at the target rung, the third at 60 BPM", func(t *testing.T) {
		f := guideToneFixture(t, "ii-V-I")

		plan := f.composeInTheHead(t, 20)

		tones := itemsOfKind(plan, domain.PracticeItemKindGuideTone)
		require.Len(t, tones, 1)
		item := tones[0]
		assert.Equal(t, domain.PracticePickNew, item.Reason)
		assert.Equal(t, domain.PracticeRungTarget, item.Rung)
		assert.Equal(t, 3*(5*4+5), item.EstimatedSeconds)
		require.NotNil(t, item.GuideTone)
		g := *item.GuideTone
		assert.Equal(t, "3", g.Tone)
		assert.Equal(t, 60, g.TempoBPM)
		assert.Equal(t, domain.FretWindow{FromFret: 0}, g.FretWindow)
		require.Len(t, g.Bars, 4)
		assert.NotEmpty(t, g.Voicings, "the playback sounds the chords")
	})

	t.Run("a session plans at most one guide tone", func(t *testing.T) {
		f := guideToneFixture(t, "ii-V-I", "IV-V-I", "I-V-vi-IV", "I-vi-IV-V")

		plan := f.composeInTheHead(t, 30)

		assert.Len(t, itemsOfKind(plan, domain.PracticeItemKindGuideTone), 1)
	})

	t.Run("an in-hand session never plans a guide tone", func(t *testing.T) {
		f := guideToneFixture(t, "ii-V-I")

		// With nothing else to play, there is nothing to practise in hand.
		guitar := practiceGuitar
		_, err := f.svc.ComposePlan(context.Background(), studentCaller(), &guitar, nil, 20)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("once accurate on the item, it asks the seventh at 72 BPM, never in its last take's key", func(t *testing.T) {
		f := guideToneFixture(t, "ii-V-I")
		f.stateOf(guideToneItemKey(practiceGuitar, "ii-V-I"), domain.PracticeItemState{
			Level: domain.KnowledgeLevelAccurate, Counted: 3, Accuracy: 1, Box: 2, DueAt: f.daysAgo(1), LastMusicKey: "C",
		})
		f.stateOf(guideToneItemKey(practiceBass, "ii-V-I"), domain.PracticeItemState{
			Level: domain.KnowledgeLevelAccurate, Counted: 3, Accuracy: 1, Box: 2, DueAt: f.daysAgo(1), LastMusicKey: "C",
		})

		for range 10 {
			plan := f.composeInTheHead(t, 20)

			tones := itemsOfKind(plan, domain.PracticeItemKindGuideTone)
			require.Len(t, tones, 1)
			g := *tones[0].GuideTone
			assert.Equal(t, "7", g.Tone)
			assert.Equal(t, 72, g.TempoBPM)
			assert.NotEqual(t, "C", g.MusicKey)
		}
	})

	t.Run("a 12-bar blues isn't planned before the student is accurate on target-chord-tones", func(t *testing.T) {
		f := guideToneFixture(t, "12-bar-blues")

		// The blues is all there is, so a session in the head finds nothing to practise.
		_, err := f.svc.ComposePlan(context.Background(), studentCaller(), nil, nil, 30)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("once accurate on target-chord-tones, a 12-bar blues can be planned", func(t *testing.T) {
		f := guideToneFixture(t, "ii-V-I", "IV-V-I", "I-V-vi-IV", "I-vi-IV-V", "12-bar-blues")
		for _, layout := range []string{practiceGuitar, practiceBass} {
			for _, p := range []string{"ii-V-I", "IV-V-I", "I-V-vi-IV", "I-vi-IV-V"} {
				// Fluent and not due, so nothing is weak or due and the blues is the one new item.
				f.stateOf(guideToneItemKey(layout, p), domain.PracticeItemState{
					Level: domain.KnowledgeLevelFluent, Counted: 5, Accuracy: 1, Fluency: 1, Box: 3, DueAt: f.inDays(10),
				})
			}
		}

		plan := f.composeInTheHead(t, 30)

		tones := itemsOfKind(plan, domain.PracticeItemKindGuideTone)
		require.Len(t, tones, 1)
		assert.Equal(t, "12-bar-blues", tones[0].GuideTone.ProgressionKey)
		assert.Len(t, tones[0].GuideTone.Bars, 12)
	})
}
