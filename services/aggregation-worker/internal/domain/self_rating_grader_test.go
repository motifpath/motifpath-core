package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The full rule set is pinned by the shared golden cases (internal/bdd). These
// cover what the golden files can't: the registry, and a rating outside the enum.

func TestGraderFor(t *testing.T) {
	for _, kind := range []PracticeItemKind{PracticeItemKindPlayAlong, PracticeItemKindChordChange} {
		g, ok := GraderFor(kind)
		require.True(t, ok, kind)
		assert.Equal(t, "self_rating.v1", g.ID())
	}
	g, ok := GraderFor(PracticeItemKindExercise)
	require.True(t, ok)
	assert.Equal(t, "exercise_option.v1", g.ID())
	// Its grader comes with the fretboard drill.
	_, ok = GraderFor(PracticeItemKindFretboardCell)
	assert.False(t, ok)
}

func TestSelfRatingGrader_RejectsAnUnknownRating(t *testing.T) {
	key, err := ParsePracticeItemKey("play_along:" + diagramA)
	require.NoError(t, err)
	tempo := 90
	ref := PracticeReference{Diagrams: map[string]DiagramReference{diagramA: {ID: diagramA}}}

	got := selfRatingV1{}.Grade(key, PracticeResponse{Type: PracticeResponseSelfRating, Rating: "perfect", TempoBPM: &tempo}, ref)

	assert.Equal(t, GradeRejectionResponseDoesNotFitItem, got.Rejection)
}
