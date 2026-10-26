package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	diagramA = "00000000-0000-4000-8000-0000000000f1"
	diagramB = "00000000-0000-4000-8000-0000000000f2"
)

func TestParsePracticeItemKey(t *testing.T) {
	cases := []struct {
		raw        string
		kind       PracticeItemKind
		diagramIDs []string
	}{
		{"play_along:" + diagramA, PracticeItemKindPlayAlong, []string{diagramA}},
		{"chord_change:" + diagramA + ":" + diagramB, PracticeItemKindChordChange, []string{diagramA, diagramB}},
		{"exercise:" + diagramA, PracticeItemKindExercise, nil},
		{"fretboard_cell:" + diagramA + ":5:3", PracticeItemKindFretboardCell, nil},
		{"diagram_shape:" + diagramA, PracticeItemKindDiagramShape, []string{diagramA}},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			key, err := ParsePracticeItemKey(c.raw)
			require.NoError(t, err)
			assert.Equal(t, c.raw, key.String())
			assert.Equal(t, c.kind, key.Kind)
			assert.Equal(t, c.diagramIDs, key.DiagramIDs())
		})
	}
}

func TestParsePracticeItemKey_ProgressionKeyTour(t *testing.T) {
	key, err := ParsePracticeItemKey("progression_keys:" + diagramA + ":I-V-vi-IV")
	require.NoError(t, err)

	assert.Equal(t, PracticeItemKindProgressionKeys, key.Kind)
	assert.Equal(t, diagramA, key.LayoutInstrumentID())
	assert.Equal(t, "I-V-vi-IV", key.ProgressionKey())
	assert.Nil(t, key.DiagramIDs())
}

func TestParsePracticeItemKey_Song(t *testing.T) {
	key, err := ParsePracticeItemKey("song:" + diagramA + ":" + diagramB)
	require.NoError(t, err)

	assert.Equal(t, PracticeItemKindSong, key.Kind)
	assert.Equal(t, diagramA, key.LayoutInstrumentID())
	assert.Equal(t, diagramB, key.SongChartID())
	assert.Empty(t, key.ProgressionKey())
	assert.Nil(t, key.DiagramIDs())
}

func TestParsePracticeItemKey_RejectsMalformedKeys(t *testing.T) {
	for _, raw := range []string{
		"",
		"play_along",
		"play_along:not-a-uuid",
		"rhythm:" + diagramA,
		"chord_change:" + diagramA,
		"fretboard_cell:" + diagramA + ":0:3",
		"diagram_shape:" + diagramA + ":" + diagramB,
		"progression_keys:" + diagramA,
		"progression_keys:I-V-vi-IV",
		// A song is played on a layout, so its key names one.
		"song:" + diagramA,
		"song:" + diagramA + ":I-IV-V-I",
		// Finding every instance is a way of asking a fretboard cell, never an item of its own.
		"fretboard_note:" + diagramA + ":G",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := ParsePracticeItemKey(raw)
			assert.ErrorIs(t, err, ErrInvalidPracticeItemKey)
		})
	}
}
