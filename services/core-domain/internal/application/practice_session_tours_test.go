package application_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func tourItemKey(layout, progression string) string {
	return "progression_keys:" + layout + ":" + progression
}

// tourFixture is a practice fixture with the progression key tours installed:
// each of progressions toured on the guitar and bass layouts under
// common-progressions, on both instruments' paths. The chord catalog's
// voicings are on the guitar layout, which makes it the guitar one.
func tourFixture(t *testing.T, progressions ...string) *practiceFixture {
	f := newPracticeFixture(t)
	f.instruments.put(domain.Instrument{ID: practiceGuitar, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}})
	f.instruments.put(domain.Instrument{ID: practiceBass, Tuning: []string{"E1", "A1", "D2", "G2"}})
	f.skill("common-progressions")
	f.onPathFor([]string{practiceGuitar, practiceBass}, "common-progressions")
	for _, p := range progressions {
		f.cells[practiceGuitar] = append(f.cells[practiceGuitar], domain.ClassifiedItem{ItemKey: tourItemKey(practiceGuitar, p), NodeIDs: []string{"common-progressions"}})
		f.cells[practiceBass] = append(f.cells[practiceBass], domain.ClassifiedItem{ItemKey: tourItemKey(practiceBass, p), NodeIDs: []string{"common-progressions"}})
	}
	f.svc.WithHarmonyDrills(harmonyProgressions, harmonyChordCatalog(), fakeFretboardLayouts{
		practiceGuitar: {{ID: practiceGuitar}},
		practiceBass:   {{ID: practiceBass}},
	})
	f.svc.WithProgressionKeyTours(f.instruments)
	return f
}

func TestPracticeSessionService_ComposePlanProgressionKeyTours(t *testing.T) {
	t.Run("an in-hand guitar session plays a new tour at the move rung, from C at 60 BPM", func(t *testing.T) {
		f := tourFixture(t, "I-V-vi-IV")

		// A tour takes about 4 minutes: the new share of a 30-minute session
		// holds it.
		plan := f.compose(t, practiceGuitar, 30)

		tours := itemsOfKind(plan, domain.PracticeItemKindProgressionKeys)
		require.Len(t, tours, 1)
		item := tours[0]
		assert.Equal(t, tourItemKey(practiceGuitar, "I-V-vi-IV"), item.ItemKey)
		assert.Equal(t, domain.PracticePickNew, item.Reason)
		assert.Equal(t, domain.PracticeRungMove, item.Rung)
		assert.Equal(t, 3*(17*4+10), item.EstimatedSeconds)
		require.NotNil(t, item.ProgressionKeys)
		tour := *item.ProgressionKeys
		assert.Equal(t, "C", tour.StartMusicKey)
		assert.Equal(t, 60, tour.TourTempoBPM)
		assert.Equal(t, domain.LocalizedText{"en": "I–V–vi–IV (pop)"}, tour.ProgressionNames)
		require.Len(t, tour.Keys, 12)
		a := tour.Keys[9]
		assert.Equal(t, "A", a.MusicKey)
		assert.Equal(t, []string{"A", "E", "F#m", "D"}, a.Chart)
		assert.Equal(t, domain.ChordBox{Chord: "F#m", DiagramID: "box-F#m"}, a.Voicings[2], "an in-hand session finds the chords' voicings too")
		assert.Empty(t, a.ChordTones)
	})

	t.Run("on bass, the tour marks each chord's root, third and fifth", func(t *testing.T) {
		f := tourFixture(t, "I-V-vi-IV")

		plan := f.compose(t, practiceBass, 20)

		tours := itemsOfKind(plan, domain.PracticeItemKindProgressionKeys)
		require.Len(t, tours, 1)
		g := tours[0].ProgressionKeys.Keys[11]
		assert.Equal(t, "G", g.MusicKey)
		require.Len(t, g.ChordTones, 4)
		assert.Equal(t, domain.ChordTonePlaces{Chord: "G", Root: domain.Place{String: 1, Fret: 0}, Third: domain.Place{String: 3, Fret: 2}, Fifth: domain.Place{String: 2, Fret: 0}}, g.ChordTones[0])
		assert.NotEmpty(t, g.Voicings, "the backing sounds the chords")
	})

	t.Run("a session plays at most one tour", func(t *testing.T) {
		f := tourFixture(t, "I-V-vi-IV", "I-vi-IV-V")

		plan := f.compose(t, practiceGuitar, 30)

		assert.Len(t, itemsOfKind(plan, domain.PracticeItemKindProgressionKeys), 1)
	})

	t.Run("a session in the head never plays a tour", func(t *testing.T) {
		f := tourFixture(t, "I-V-vi-IV")
		f.cells[practiceGuitar] = append(f.cells[practiceGuitar], domain.ClassifiedItem{ItemKey: "chart_progression:I-V-vi-IV", NodeIDs: []string{"common-progressions"}})

		plan := f.composeInTheHead(t, 20)

		assert.Empty(t, itemsOfKind(plan, domain.PracticeItemKindProgressionKeys))
	})

	t.Run("the tour starts at the first key not yet clean, at its tempo", func(t *testing.T) {
		f := tourFixture(t, "I-V-vi-IV")
		f.stateOf(tourItemKey(practiceGuitar, "I-V-vi-IV"), domain.PracticeItemState{
			Level: domain.KnowledgeLevelLearning, Counted: 2, Box: 1, DueAt: f.daysAgo(1),
			KeyTour: &domain.KeyTourState{TempoBPM: 70, CleanKeys: []string{"C", "F"}},
		})

		plan := f.compose(t, practiceGuitar, 20)

		tours := itemsOfKind(plan, domain.PracticeItemKindProgressionKeys)
		require.Len(t, tours, 1)
		assert.Equal(t, "Bb", tours[0].ProgressionKeys.StartMusicKey)
		assert.Equal(t, 70, tours[0].ProgressionKeys.TourTempoBPM)
	})
}
