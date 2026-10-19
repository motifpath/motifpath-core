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

func TestParsePracticeItemKey_RejectsMalformedKeys(t *testing.T) {
	for _, raw := range []string{
		"",
		"play_along",
		"play_along:not-a-uuid",
		"rhythm:" + diagramA,
		"chord_change:" + diagramA,
		"fretboard_cell:" + diagramA + ":0:3",
		"diagram_shape:" + diagramA + ":" + diagramB,
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := ParsePracticeItemKey(raw)
			assert.ErrorIs(t, err, ErrInvalidPracticeItemKey)
		})
	}
}
