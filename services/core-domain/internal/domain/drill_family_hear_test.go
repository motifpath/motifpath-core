package domain_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func playbackChords(p *domain.GeneratedPlayback) []string {
	chords := make([]string, len(p.Bars))
	for i, b := range p.Bars {
		chords[i] = b.Chord
	}
	return chords
}

func playbackNumerals(p *domain.GeneratedPlayback) []string {
	numerals := make([]string, len(p.Bars))
	for i, b := range p.Bars {
		numerals[i] = b.Numeral
	}
	return numerals
}

func TestHeardChordItem_AsADrillItem(t *testing.T) {
	vi := domain.HeardChordItem{Numeral: "vi"}
	inG := func() func(int) int { return picks(7) } // MusicKeys[7] is G

	t.Run("is a numeral heard in any key, in the head, by ear then by sight", func(t *testing.T) {
		assert.Equal(t, "heard_chord:vi", vi.ItemKey())
		assert.Equal(t, domain.PracticeItemKindHeardChord, vi.Kind())
		assert.Equal(t, []string{"cadence_then_chord", "see_then_name"}, vi.Ways())
		assert.True(t, vi.SuitsSession(false))
		assert.False(t, vi.SuitsSession(true))
	})

	t.Run("while learning it, the cadence I IV V I sets the key, then the chord plays with no numeral", func(t *testing.T) {
		q, s := plannedChoice(t, vi, "cadence_then_chord", domain.DrillPlanning{Pick: inG(), Level: domain.KnowledgeLevelLearning, Voicing: boxOf})

		assert.Equal(t, "G", q.MusicKey)
		require.NotNil(t, q.Playback)
		assert.Equal(t, []string{"G", "C", "D", "G", "Em"}, playbackChords(q.Playback))
		assert.Equal(t, []string{"I", "IV", "V", "I", ""}, playbackNumerals(q.Playback))
		assert.Equal(t, "vi", q.Answer)
		require.NoError(t, domain.CheckChallenge(s))
	})

	t.Run("a new item gets the cadence too", func(t *testing.T) {
		q, _ := plannedChoice(t, vi, "cadence_then_chord", domain.DrillPlanning{Pick: inG()})
		assert.Equal(t, []string{"I", "IV", "V", "I", ""}, playbackNumerals(q.Playback))
	})

	t.Run("once accurate, only the tonic plays, for one bar, before the chord", func(t *testing.T) {
		q, _ := plannedChoice(t, vi, "cadence_then_chord", domain.DrillPlanning{Pick: inG(), Level: domain.KnowledgeLevelAccurate})

		assert.Equal(t, []string{"G", "Em"}, playbackChords(q.Playback))
		assert.Equal(t, []string{"I", ""}, playbackNumerals(q.Playback))
	})

	t.Run("until fluent, it offers the numeral and the three nearest it", func(t *testing.T) {
		q, _ := plannedChoice(t, vi, "cadence_then_chord", domain.DrillPlanning{Pick: firstPick, Level: domain.KnowledgeLevelAccurate})
		assert.ElementsMatch(t, []string{"vi", "V", "viio", "IV"}, optionValues(q))
	})

	t.Run("once fluent, it offers all seven numerals in scale order", func(t *testing.T) {
		for _, level := range []domain.KnowledgeLevel{domain.KnowledgeLevelFluent, domain.KnowledgeLevelRetained} {
			q, s := plannedChoice(t, vi, "cadence_then_chord", domain.DrillPlanning{Pick: rand.IntN, Level: level})
			assert.Equal(t, domain.ChordNumerals, optionValues(q), level)
			require.NoError(t, domain.CheckChallenge(s))
		}
	})

	t.Run("its playback starts at the tempo of the student's level", func(t *testing.T) {
		for level, bpm := range map[domain.KnowledgeLevel]int{
			"": 70, domain.KnowledgeLevelNew: 70, domain.KnowledgeLevelLearning: 70,
			domain.KnowledgeLevelAccurate: 90, domain.KnowledgeLevelFluent: 110, domain.KnowledgeLevelRetained: 110,
		} {
			q, _ := plannedChoice(t, vi, "cadence_then_chord", domain.DrillPlanning{Pick: firstPick, Level: level})
			assert.Equal(t, bpm, q.Playback.TempoBPM, level)
		}
	})

	t.Run("it lists the voicing of every chord it sounds, whatever the reading", func(t *testing.T) {
		q, _ := plannedChoice(t, vi, "cadence_then_chord", domain.DrillPlanning{Pick: inG(), Reading: domain.GeneratedChordSymbols, Voicing: boxOf})

		assert.Equal(t, []domain.ChordBox{
			{Chord: "G", DiagramID: "box-G"}, {Chord: "C", DiagramID: "box-C"}, {Chord: "D", DiagramID: "box-D"}, {Chord: "Em", DiagramID: "box-Em"},
		}, q.Voicings)
	})

	t.Run("on bass, a root line plays under the chords on the bass the session reads", func(t *testing.T) {
		q, _ := plannedChoice(t, vi, "cadence_then_chord", domain.DrillPlanning{Pick: inG(), Reading: domain.GeneratedChordSymbols, BassInstrumentID: "the-bass"})
		assert.Equal(t, "the-bass", q.Playback.BassInstrumentID)
	})

	t.Run("on guitar, no root line plays", func(t *testing.T) {
		q, _ := plannedChoice(t, vi, "cadence_then_chord", domain.DrillPlanning{Pick: inG(), Reading: domain.GeneratedChordBoxes})
		assert.Empty(t, q.Playback.BassInstrumentID)
	})

	t.Run("by sight, it names the key, draws the chord and plays nothing", func(t *testing.T) {
		q, s := plannedChoice(t, vi, "see_then_name", domain.DrillPlanning{Pick: inG(), Reading: domain.GeneratedChordBoxes, Voicing: boxOf, Level: domain.KnowledgeLevelLearning})

		assert.Equal(t, "G", q.MusicKey)
		assert.Equal(t, "Em", q.Chord)
		assert.Nil(t, q.Playback)
		assert.Equal(t, domain.GeneratedChordBoxes, q.DrawnAs)
		assert.Equal(t, []domain.ChordBox{{Chord: "Em", DiagramID: "box-Em"}}, q.Voicings)
		assert.Contains(t, optionValues(q), "vi")
		assert.Len(t, q.Options, 4)
		assert.Equal(t, "vi", q.Answer)
		require.NoError(t, domain.CheckChallenge(s))
	})
}

func TestHeardProgressionItem_AsADrillItem(t *testing.T) {
	pop := domain.HeardProgressionItem{Progression: progression("I-V-vi-IV"), Catalog: catalogProgressions}

	t.Run("is a progression heard in any key, in the head, confused with its neighbours", func(t *testing.T) {
		assert.Equal(t, "heard_progression:I-V-vi-IV", pop.ItemKey())
		assert.Equal(t, domain.PracticeItemKindHeardProgression, pop.Kind())
		assert.Equal(t, []string{"progression_by_number"}, pop.Ways())
		assert.Equal(t, []string{"heard_progression:I-vi-IV-V", "heard_progression:vi-IV-I-V"}, pop.ConfusableWith())
		assert.True(t, pop.SuitsSession(false))
		assert.False(t, pop.SuitsSession(true))
	})

	t.Run("plays its four bars with no numeral and offers it among its confusables", func(t *testing.T) {
		q, s := plannedChoice(t, pop, "progression_by_number", domain.DrillPlanning{Pick: picks(7), Voicing: boxOf, Level: domain.KnowledgeLevelFluent})

		assert.Equal(t, "G", q.MusicKey)
		require.NotNil(t, q.Playback)
		assert.Equal(t, []string{"G", "D", "Em", "C"}, playbackChords(q.Playback))
		assert.Equal(t, []string{"", "", "", ""}, playbackNumerals(q.Playback))
		assert.Equal(t, 110, q.Playback.TempoBPM)
		assert.ElementsMatch(t, []string{"I-V-vi-IV", "I-vi-IV-V", "vi-IV-I-V", "I-vi-ii-V"}, optionValues(q))
		assert.Equal(t, "I-V-vi-IV", q.Answer)
		assert.Empty(t, q.Chart)
		assert.Len(t, q.Voicings, 4)
		require.NoError(t, domain.CheckChallenge(s))
	})
}

// Every heard item, asked every way, at every level and in every reading,
// challenges the student and is a question the grader accepts.
func TestHearDrills_EveryPlanChallenges(t *testing.T) {
	var items []domain.DrillItem
	for _, numeral := range domain.HarmonyItemNumerals {
		items = append(items, domain.HeardChordItem{Numeral: numeral})
	}
	for _, p := range catalogProgressions {
		if len(p.Bars) == 4 {
			items = append(items, domain.HeardProgressionItem{Progression: p, Catalog: catalogProgressions})
		}
	}
	levels := []domain.KnowledgeLevel{"", domain.KnowledgeLevelNew, domain.KnowledgeLevelLearning, domain.KnowledgeLevelAccurate, domain.KnowledgeLevelFluent, domain.KnowledgeLevelRetained}
	readings := []domain.GeneratedChordDrawing{domain.GeneratedChordBoxes, domain.GeneratedChordSymbols}

	for _, item := range items {
		for _, way := range item.Ways() {
			for _, level := range levels {
				for _, reading := range readings {
					for range 5 {
						planning := domain.DrillPlanning{Pick: rand.IntN, Reading: reading, Voicing: boxOf, Level: level}
						q, s := plannedChoice(t, item, way, planning)
						name := item.ItemKey() + " " + way + " " + string(level)

						require.NoError(t, domain.CheckChallenge(s), name)
						values := optionValues(q)
						assert.Len(t, slices.Compact(slices.Sorted(slices.Values(values))), len(values), name+": distinct options")
						assert.Contains(t, values, q.Answer, name)
						assert.LessOrEqual(t, len(values), 7, name)
						assert.GreaterOrEqual(t, len(values), 2, name)
						assert.Contains(t, domain.MusicKeys, q.MusicKey, name)
						if way == "see_then_name" {
							assert.Nil(t, q.Playback, name)
							continue
						}
						require.NotNil(t, q.Playback, name)
						for _, bar := range q.Playback.Bars {
							assert.Contains(t, chordsOfKey(q.MusicKey), bar.Chord, name+": chords of the key")
							assert.True(t, slices.ContainsFunc(q.Voicings, func(b domain.ChordBox) bool { return b.Chord == bar.Chord }), name+": every chord sounded has a voicing")
						}
						assert.Empty(t, q.Playback.Bars[len(q.Playback.Bars)-1].Numeral, name+": the asked bar shows no numeral")
					}
				}
			}
		}
	}
}
