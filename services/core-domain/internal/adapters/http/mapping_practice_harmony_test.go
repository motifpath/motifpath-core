package http

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestToGeneratedPracticeSessionItem_GeneratedChoice(t *testing.T) {
	emBox := "1e1f9a7c-3f5b-5d55-9b0c-7a6a1d5f2e10"
	plan := func(item domain.DrillItem, way string, planning domain.DrillPlanning) domain.PracticeSessionItem {
		out := domain.PracticeSessionItem{ItemKey: item.ItemKey(), Reason: domain.PracticePickNew, Level: domain.KnowledgeLevelNew, EstimatedSeconds: item.Seconds()}
		out.Ask(item.Plan(way, planning))
		return out
	}
	first := func(int) int { return 0 }

	t.Run("a numeral question names the numeral, offers chords and the box of each", func(t *testing.T) {
		boxes := domain.DrillPlanning{Pick: first, Reading: domain.GeneratedChordBoxes, Voicing: func(string) (string, bool) { return emBox, true }}
		got := toGeneratedPracticeSessionItem(plan(domain.DiatonicChordItem{Key: "G", Numeral: "vi"}, "numeral_to_chord", boxes), nil, "en")

		assert.Equal(t, generated.PracticeItemKindDiatonicChord, got.Kind)
		q := got.GeneratedChoice
		require.NotNil(t, q)
		assert.Equal(t, generated.NumeralToChord, q.Way)
		assert.Equal(t, generated.MusicKey("G"), q.MusicKey)
		require.NotNil(t, q.Numeral)
		assert.Equal(t, generated.ChordNumeral("vi"), *q.Numeral)
		assert.Nil(t, q.Chord)
		assert.Nil(t, q.Function)
		assert.Nil(t, q.Progression)
		assert.Equal(t, generated.ChordBoxes, q.DrawnAs)
		require.Len(t, q.Options, 4)
		assert.Equal(t, "Em", q.Options[0].Value)
		assert.Nil(t, q.Options[0].Name)
		require.NotEmpty(t, q.Voicings)
		assert.Equal(t, emBox, q.Voicings[0].DiagramId.String())

		body, err := json.Marshal(q)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"chart":[]`, "an unused list is empty, never null")
	})

	t.Run("a function question names its options in the student's language", func(t *testing.T) {
		symbols := domain.DrillPlanning{Pick: first, Reading: domain.GeneratedChordSymbols}
		got := toGeneratedPracticeSessionItem(plan(domain.ChordFunctionItem{Key: "G", Numeral: "V"}, "chord_to_function", symbols), nil, "pt_BR")

		q := got.GeneratedChoice
		require.NotNil(t, q)
		require.NotNil(t, q.Chord)
		assert.Equal(t, "D", *q.Chord)
		assert.Equal(t, generated.ChordSymbols, q.DrawnAs)
		assert.NotNil(t, q.Voicings)
		assert.Empty(t, q.Voicings)
		require.NotNil(t, q.Options[2].Name)
		assert.Equal(t, "dominant", q.Options[2].Value)
		assert.Equal(t, "Dominante — puxa para casa", *q.Options[2].Name)
	})

	t.Run("a pick-the-chart question names the progression and gives each option's chart", func(t *testing.T) {
		pop := domain.Progression{Key: "I-V-vi-IV", Bars: []string{"I", "V", "vi", "IV"}, Names: domain.LocalizedText{"en": "I–V–vi–IV (pop)", "pt_BR": "I–V–vi–IV (pop)"}}
		item := domain.ChartProgressionItem{Progression: pop, Catalog: []domain.Progression{pop, {Key: "x", Bars: []string{"I", "IV", "V", "I"}}}}
		got := toGeneratedPracticeSessionItem(plan(item, "numbers_to_chart", domain.DrillPlanning{Pick: first, Reading: domain.GeneratedChordSymbols}), nil, "en")

		q := got.GeneratedChoice
		require.NotNil(t, q)
		require.NotNil(t, q.Progression)
		assert.Equal(t, "I-V-vi-IV", q.Progression.Key)
		assert.Equal(t, "I–V–vi–IV (pop)", q.Progression.Name)
		assert.Equal(t, "C G Am F", q.Options[0].Value)
		assert.Equal(t, []string{"C", "G", "Am", "F"}, q.Options[0].Chart)
		assert.Nil(t, q.Playback)
	})

	t.Run("a heard chord gives its playback: the tempo, the bass line, and only the key's numerals", func(t *testing.T) {
		planning := domain.DrillPlanning{Pick: first, Reading: domain.GeneratedChordSymbols, BassLine: true, Voicing: func(string) (string, bool) { return emBox, true }}
		got := toGeneratedPracticeSessionItem(plan(domain.HeardChordItem{Numeral: "vi"}, "cadence_then_chord", planning), nil, "en")

		assert.Equal(t, generated.PracticeItemKindHeardChord, got.Kind)
		q := got.GeneratedChoice
		require.NotNil(t, q)
		assert.Equal(t, generated.CadenceThenChord, q.Way)
		assert.Equal(t, generated.MusicKey("C"), q.MusicKey)
		assert.Nil(t, q.Chord)
		require.NotNil(t, q.Playback)
		assert.Equal(t, 70, q.Playback.TempoBpm)
		assert.True(t, q.Playback.BassLine)
		require.Len(t, q.Playback.Bars, 5)
		assert.Equal(t, "F", q.Playback.Bars[1].Chord)
		require.NotNil(t, q.Playback.Bars[1].Numeral)
		assert.Equal(t, generated.ChordNumeral("IV"), *q.Playback.Bars[1].Numeral)
		assert.Equal(t, "Am", q.Playback.Bars[4].Chord)
		assert.Nil(t, q.Playback.Bars[4].Numeral, "the asked chord's bar shows no numeral")
		assert.Len(t, q.Voicings, 4, "every chord it sounds, though the session reads symbols")

		body, err := json.Marshal(q)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"numeral":null`)
	})

	t.Run("a heard chord seen by sight gives the chord and no playback", func(t *testing.T) {
		got := toGeneratedPracticeSessionItem(plan(domain.HeardChordItem{Numeral: "vi"}, "see_then_name", domain.DrillPlanning{Pick: first, Reading: domain.GeneratedChordSymbols}), nil, "en")

		q := got.GeneratedChoice
		require.NotNil(t, q)
		assert.Equal(t, generated.SeeThenName, q.Way)
		require.NotNil(t, q.Chord)
		assert.Equal(t, "Am", *q.Chord)
		assert.Nil(t, q.Playback)
	})
}
