package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestToGeneratedPracticeSessionItem_ChordDrills(t *testing.T) {
	am, c := "1626c216-0ff3-5500-a466-df1143624046", "c787d144-1cdc-5119-9e04-efb0a0792b89"
	bass := "7b1b7c43-6d9f-5b6d-9b57-0c1e3c9f4a11"
	best, last := 38, 40

	t.Run("a single chord names its voicing diagram and chord", func(t *testing.T) {
		got := toGeneratedPracticeSessionItem(domain.PracticeSessionItem{
			ItemKey: "chord_voicing:" + am, Kind: domain.PracticeItemKindChordVoicing, Rung: domain.PracticeRungPlayClean,
			ChordVoicing: &domain.PlannedChordVoicing{DiagramID: am, ChordSymbol: "Am"},
		}, nil, "en")

		assert.Equal(t, generated.PracticeItemKindChordVoicing, got.Kind)
		require.NotNil(t, got.ChordVoicing)
		assert.Equal(t, am, got.ChordVoicing.DiagramId.String())
		assert.Equal(t, "Am", got.ChordVoicing.ChordSymbol)
	})

	t.Run("a chord change names both chords and its minute", func(t *testing.T) {
		got := toGeneratedPracticeSessionItem(domain.PracticeSessionItem{
			ItemKey: "chord_change:" + am + ":" + c, Kind: domain.PracticeItemKindChordChange, Rung: domain.PracticeRungPlayInTime,
			ChordChange: &domain.PlannedChordChange{
				FromDiagramID: am, ToDiagramID: c, FromChordSymbol: "Am", ToChordSymbol: "C",
				Minute: domain.PlannedChangeMinute{StartTempoBPM: 45, TargetChangesPerMinute: 60, BestCleanChangesPerMinute: &best, LastChangesPerMinute: &last},
			},
		}, nil, "en")

		require.NotNil(t, got.ChordChange)
		assert.Equal(t, generated.PlannedChordChange{
			FromDiagramId: mustUUID(am), ToDiagramId: mustUUID(c), FromChordSymbol: "Am", ToChordSymbol: "C",
			Minute: generated.ChangeMinute{StartTempoBpm: 45, TargetChangesPerMinute: 60, BestCleanChangesPerMinute: &best, LastChangesPerMinute: &last},
		}, *got.ChordChange)
	})

	t.Run("a root and fifth marks both places on the bass layout", func(t *testing.T) {
		got := toGeneratedPracticeSessionItem(domain.PracticeSessionItem{
			ItemKey: "root_fifth:" + bass + ":Am", Kind: domain.PracticeItemKindRootFifth,
			RootFifth: &domain.PlannedRootFifth{LayoutInstrumentID: bass, ChordSymbol: "Am", Root: domain.Place{String: 3, Fret: 0}, Fifth: domain.Place{String: 4, Fret: 0}},
		}, nil, "en")

		require.NotNil(t, got.RootFifth)
		assert.Equal(t, generated.PlannedRootFifth{
			LayoutInstrumentId: mustUUID(bass), ChordSymbol: "Am",
			Root: generated.FretPlace{String: 3, Fret: 0}, Fifth: generated.FretPlace{String: 4, Fret: 0},
		}, *got.RootFifth)
	})

	t.Run("a root change marks both roots and its minute, a new one with no counts", func(t *testing.T) {
		got := toGeneratedPracticeSessionItem(domain.PracticeSessionItem{
			ItemKey: "root_change:" + bass + ":Am:C", Kind: domain.PracticeItemKindRootChange,
			RootChange: &domain.PlannedRootChange{
				LayoutInstrumentID: bass, FromChordSymbol: "Am", ToChordSymbol: "C",
				FromRoot: domain.Place{String: 3, Fret: 0}, ToRoot: domain.Place{String: 3, Fret: 3},
				Minute: domain.PlannedChangeMinute{StartTempoBPM: 40, TargetChangesPerMinute: 60},
			},
		}, nil, "en")

		require.NotNil(t, got.RootChange)
		assert.Equal(t, generated.FretPlace{String: 3, Fret: 3}, got.RootChange.ToRoot)
		assert.Equal(t, generated.ChangeMinute{StartTempoBpm: 40, TargetChangesPerMinute: 60}, got.RootChange.Minute)
	})
}
