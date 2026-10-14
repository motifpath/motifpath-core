package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The full rule set is pinned by the shared golden cases (internal/bdd). These
// cover what the golden files can't: the registry, and a rating outside the enum.

func TestGraderFor(t *testing.T) {
	want := map[PracticeItemKind]string{
		PracticeItemKindPlayAlong:        "self_rating.v1",
		PracticeItemKindChordChange:      "self_rating.v1",
		PracticeItemKindChordVoicing:     "self_rating.v1",
		PracticeItemKindRootFifth:        "self_rating.v1",
		PracticeItemKindRootChange:       "self_rating.v1",
		PracticeItemKindExercise:         "exercise_option.v1",
		PracticeItemKindFretboardCell:    "fretboard_cell.v2",
		PracticeItemKindDiagramShape:     "diagram_shape.v1",
		PracticeItemKindDiatonicChord:    "generated_choice.v1",
		PracticeItemKindChordFunction:    "generated_choice.v1",
		PracticeItemKindChartProgression: "generated_choice.v1",
		PracticeItemKindHeardChord:       "generated_choice.v1",
		PracticeItemKindHeardProgression: "generated_choice.v1",
		PracticeItemKindGuideTone:        "timed_taps.v1",
		PracticeItemKindFretboardNote:    "multi_tap.v1",
	}
	for kind, id := range want {
		g, ok := GraderFor(kind)
		require.True(t, ok, kind)
		assert.Equal(t, id, g.ID(), kind)
	}
}

func TestSelfRatingGrader_RejectsAnUnknownRating(t *testing.T) {
	key, err := ParsePracticeItemKey("play_along:" + diagramA)
	require.NoError(t, err)
	tempo := 90
	ref := PracticeReference{Diagrams: map[string]DiagramReference{diagramA: {ID: diagramA}}}

	got := selfRatingV1{}.Grade(key, PracticeResponse{Type: PracticeResponseSelfRating, Rating: "perfect", TempoBPM: &tempo}, ref)

	assert.Equal(t, GradeRejectionResponseDoesNotFitItem, got.Rejection)
}
