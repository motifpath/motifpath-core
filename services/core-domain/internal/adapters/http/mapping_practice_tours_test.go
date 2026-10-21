package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestToGeneratedPracticeSessionItem_ProgressionKeyTour(t *testing.T) {
	bass, boxC := "7b1b7c43-6d9f-5b6d-9b57-0c1e3c9f4a11", "c787d144-1cdc-5119-9e04-efb0a0792b89"

	got := toGeneratedPracticeSessionItem(domain.PracticeSessionItem{
		ItemKey: "progression_keys:" + bass + ":I-V-vi-IV", Kind: domain.PracticeItemKindProgressionKeys, Rung: domain.PracticeRungMove,
		ProgressionKeys: &domain.PlannedProgressionKeys{
			LayoutInstrumentID: bass, ProgressionKey: "I-V-vi-IV",
			ProgressionNames: domain.LocalizedText{"en": "I–V–vi–IV (pop)", "pt-BR": "I–V–vi–IV (pop)"},
			TimesThrough:     4, Takes: 3, StartMusicKey: "F", TourTempoBPM: 35, StartNoteValue: domain.TempoNoteValueEighths,
			TargetTempoBPM: 100, MinTempoBPM: 60,
			Keys: []domain.PlannedTourKey{
				{
					MusicKey: "C", Clean: true, Chart: []string{"C", "G", "Am", "F"},
					Voicings: []domain.ChordBox{{Chord: "C", DiagramID: boxC}},
					ChordTones: []domain.ChordTonePlaces{{Chord: "C", Root: domain.Place{String: 3, Fret: 3}, Third: domain.Place{String: 2, Fret: 2}, Fifth: domain.Place{String: 1, Fret: 0}}},
				},
				{MusicKey: "F", Chart: []string{"F", "C", "Dm", "Bb"}},
			},
		},
	}, nil, "en")

	assert.Equal(t, generated.PracticeItemKindProgressionKeys, got.Kind)
	require.NotNil(t, got.ProgressionKeys)
	tour := got.ProgressionKeys
	assert.Equal(t, mustUUID(bass), tour.LayoutInstrumentId)
	assert.Equal(t, "I-V-vi-IV", tour.Progression.Key)
	assert.Equal(t, "I–V–vi–IV (pop)", tour.Progression.Name)
	assert.Equal(t, 4, tour.TimesThrough)
	assert.Equal(t, 3, tour.Takes)
	assert.Equal(t, generated.MusicKey("F"), tour.StartMusicKey)
	assert.Equal(t, 35, tour.TourTempoBpm)
	assert.Equal(t, generated.NoteValue2("eighths"), tour.StartNoteValue)
	assert.Equal(t, 100, tour.TargetTempoBpm)
	assert.Equal(t, 60, tour.MinTempoBpm)
	require.Len(t, tour.Keys, 2)
	c := tour.Keys[0]
	assert.Equal(t, generated.MusicKey("C"), c.MusicKey)
	assert.True(t, c.Clean)
	assert.Equal(t, []string{"C", "G", "Am", "F"}, c.Chart)
	require.Len(t, c.Voicings, 1)
	assert.Equal(t, "C", c.Voicings[0].Chord)
	assert.Equal(t, mustUUID(boxC), c.Voicings[0].DiagramId)
	require.Len(t, c.ChordTones, 1)
	assert.Equal(t, generated.FretPlace{String: 2, Fret: 2}, c.ChordTones[0].Third)
	f := tour.Keys[1]
	assert.False(t, f.Clean)
	assert.NotNil(t, f.Voicings, "an empty list, never null")
	assert.NotNil(t, f.ChordTones, "an empty list on guitar, never null")
}
