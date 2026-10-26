package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestToGeneratedPracticeSessionItem_GuideTone(t *testing.T) {
	guitar, boxAm := "6ea2d087-ab9c-59dc-9657-8546025414d2", "c787d144-1cdc-5119-9e04-efb0a0792b89"

	got := toGeneratedPracticeSessionItem(domain.PracticeSessionItem{
		ItemKey: "guide_tone:" + guitar + ":ii-V-I", Kind: domain.PracticeItemKindGuideTone, Rung: domain.PracticeRungTarget,
		GuideTone: &domain.PlannedGuideTone{
			LayoutInstrumentID: guitar, ProgressionKey: "ii-V-I",
			ProgressionNames: domain.LocalizedText{"en": "ii–V–I", "pt-BR": "ii–V–I"},
			MusicKey:         "G", Tone: "3", TempoBPM: 60, TargetTempoBPM: 90, Takes: 3, FretWindow: domain.FretWindow{FromFret: 0},
			Bars:     []domain.GuideToneBar{{Numeral: "ii", Chord: "Am"}, {Numeral: "V", Chord: "D"}},
			Voicings: []domain.ChordBox{{Chord: "Am", DiagramID: boxAm}},
		},
	}, nil, "en")

	assert.Equal(t, generated.PracticeItemKindGuideTone, got.Kind)
	require.NotNil(t, got.GuideTone)
	g := got.GuideTone
	assert.Equal(t, mustUUID(guitar), g.LayoutInstrumentId)
	assert.Equal(t, "ii-V-I", g.Progression.Key)
	assert.Equal(t, "ii–V–I", g.Progression.Name)
	assert.Equal(t, generated.MusicKey("G"), g.MusicKey)
	assert.Equal(t, generated.PlannedGuideToneTone("3"), g.Tone)
	assert.Equal(t, 60, g.TempoBpm)
	assert.Equal(t, 90, g.TargetTempoBpm)
	assert.Equal(t, 3, g.Takes)
	assert.Equal(t, generated.FretWindow{FromFret: generated.FretWindowFromFretN0}, g.FretWindow)
	require.Len(t, g.Bars, 2)
	assert.Equal(t, "ii", g.Bars[0].Numeral)
	assert.Equal(t, "Am", g.Bars[0].Chord)
	require.Len(t, g.Voicings, 1)
	assert.Equal(t, mustUUID(boxAm), g.Voicings[0].DiagramId)
}

func TestToGeneratedPracticeSessionItem_GuideToneListsAreNeverNull(t *testing.T) {
	got := toGeneratedPracticeSessionItem(domain.PracticeSessionItem{
		ItemKey: "guide_tone:x:ii-V-I", Kind: domain.PracticeItemKindGuideTone,
		GuideTone: &domain.PlannedGuideTone{LayoutInstrumentID: "6ea2d087-ab9c-59dc-9657-8546025414d2", ProgressionKey: "ii-V-I", MusicKey: "C", Tone: "7"},
	}, nil, "en")

	require.NotNil(t, got.GuideTone)
	assert.NotNil(t, got.GuideTone.Bars)
	assert.NotNil(t, got.GuideTone.Voicings)
}
