package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The full rule set is pinned by the shared golden cases (internal/bdd). These
// cover the item key and what the golden files leave implicit.

const exerciseA = "00000000-0000-4000-8000-0000000000e1"

func TestPracticeItemKey_ExerciseID(t *testing.T) {
	key, err := ParsePracticeItemKey("exercise:" + exerciseA)
	require.NoError(t, err)
	assert.Equal(t, exerciseA, key.ExerciseID())

	key, err = ParsePracticeItemKey("play_along:" + diagramA)
	require.NoError(t, err)
	assert.Empty(t, key.ExerciseID())
}

func TestExerciseOptionGrader_RejectsAnAnswerWithoutALatency(t *testing.T) {
	key, err := ParsePracticeItemKey("exercise:" + exerciseA)
	require.NoError(t, err)
	ref := PracticeReference{Exercises: map[string]ExerciseReference{exerciseA: {ID: exerciseA, OptionIDs: []string{"o-1"}, CorrectOptionIDs: []string{"o-1"}}}}

	got := exerciseOptionV1{}.Grade(key, PracticeResponse{Type: PracticeResponseOptionChoice, OptionIDs: []string{"o-1"}}, ref)

	assert.Equal(t, GradeRejectionResponseDoesNotFitItem, got.Rejection)
}

func TestExerciseOptionGrader_KeepsTheAudioLength(t *testing.T) {
	key, err := ParsePracticeItemKey("exercise:" + exerciseA)
	require.NoError(t, err)
	ref := PracticeReference{Exercises: map[string]ExerciseReference{exerciseA: {ID: exerciseA, OptionIDs: []string{"o-1"}, CorrectOptionIDs: []string{"o-1"}}}}
	latency, audio := 9500, 5000

	got := exerciseOptionV1{}.Grade(key, PracticeResponse{Type: PracticeResponseOptionChoice, OptionIDs: []string{"o-1"}, LatencyMs: &latency, AudioMs: &audio}, ref)

	require.Empty(t, got.Rejection)
	assert.Equal(t, &audio, got.Evidence.AudioMs)
}
