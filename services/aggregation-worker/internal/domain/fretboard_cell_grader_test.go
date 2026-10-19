package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The full rule set is pinned by the shared golden cases (internal/bdd). These
// cover what the golden files leave implicit.

const guitarLayout = "6ea2d087-ab9c-59dc-9657-8546025414d2"

func guitarReference() PracticeReference {
	return PracticeReference{Instruments: map[string]InstrumentReference{
		guitarLayout: {ID: guitarLayout, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}},
	}}
}

func cellKey(t *testing.T, str, fret string) PracticeItemKey {
	t.Helper()
	key, err := ParsePracticeItemKey("fretboard_cell:" + guitarLayout + ":" + str + ":" + fret)
	require.NoError(t, err)
	return key
}

func TestFretboardCellGrader(t *testing.T) {
	latency := 1800
	five, three := 5, 3
	tests := []struct {
		name     string
		response PracticeResponse
		want     GradeRejection
		correct  bool
	}{
		{name: "a double flat names its pitch", response: PracticeResponse{Type: PracticeResponseNameTheNote, NoteName: "Dbb", LatencyMs: &latency}, correct: true},
		{name: "a note that isn't one is rejected", response: PracticeResponse{Type: PracticeResponseNameTheNote, NoteName: "H", LatencyMs: &latency}, want: GradeRejectionResponseDoesNotFitItem},
		{name: "an answer without a time is rejected", response: PracticeResponse{Type: PracticeResponseNameTheNote, NoteName: "C"}, want: GradeRejectionResponseDoesNotFitItem},
		{name: "a tap without a cell is rejected", response: PracticeResponse{Type: PracticeResponseFindTheNote, String: &five, LatencyMs: &latency}, want: GradeRejectionResponseDoesNotFitItem},
		{name: "a tap below the open string is rejected", response: PracticeResponse{Type: PracticeResponseFindTheNote, String: &five, Fret: intPtr(-1), LatencyMs: &latency}, want: GradeRejectionInvalidCell},
		{name: "two octaves up on the asked string is right", response: PracticeResponse{Type: PracticeResponseFindTheNote, String: &five, Fret: intPtr(three + 24), LatencyMs: &latency}, correct: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fretboardCellV1{}.Grade(cellKey(t, "5", "3"), tt.response, guitarReference())

			assert.Equal(t, tt.want, got.Rejection)
			if tt.want == "" {
				require.NotNil(t, got.Evidence.Correct)
				assert.Equal(t, tt.correct, *got.Evidence.Correct)
			}
		})
	}
}

func TestFretboardCellGrader_KeepsTheAskedCellAndItsNoteAsTheAnswerKey(t *testing.T) {
	latency := 2500

	got := fretboardCellV1{}.Grade(cellKey(t, "6", "1"), PracticeResponse{Type: PracticeResponseNameTheNote, NoteName: "G", LatencyMs: &latency}, guitarReference())

	require.NotNil(t, got.Evidence.AnswerKey)
	assert.Equal(t, AnswerKey{String: intPtr(6), Fret: intPtr(1), NoteName: "F"}, *got.Evidence.AnswerKey)
}

func TestExerciseOptionGrader_KeepsEveryOptionShownAsTheAnswerKey(t *testing.T) {
	key, err := ParsePracticeItemKey("exercise:" + exerciseA)
	require.NoError(t, err)
	shown := []AnswerOption{{OptionID: "o-1", IsCorrect: true, Shown: []byte("c")}, {OptionID: "o-2", Shown: []byte("d")}}
	ref := PracticeReference{Exercises: map[string]ExerciseReference{exerciseA: {ID: exerciseA, OptionIDs: []string{"o-1", "o-2"}, CorrectOptionIDs: []string{"o-1"}, Options: shown}}}
	latency := 4000

	got := exerciseOptionV1{}.Grade(key, PracticeResponse{Type: PracticeResponseOptionChoice, OptionIDs: []string{"o-2"}, LatencyMs: &latency}, ref)

	require.NotNil(t, got.Evidence.AnswerKey)
	assert.Equal(t, shown, got.Evidence.AnswerKey.Options)
}

func TestExerciseOptionGrader_AnExerciseKeptWithoutItsOptionsHasNoAnswerKey(t *testing.T) {
	key, err := ParsePracticeItemKey("exercise:" + exerciseA)
	require.NoError(t, err)
	ref := PracticeReference{Exercises: map[string]ExerciseReference{exerciseA: {ID: exerciseA, OptionIDs: []string{"o-1"}, CorrectOptionIDs: []string{"o-1"}}}}
	latency := 4000

	got := exerciseOptionV1{}.Grade(key, PracticeResponse{Type: PracticeResponseOptionChoice, OptionIDs: []string{"o-1"}, LatencyMs: &latency}, ref)

	assert.Nil(t, got.Evidence.AnswerKey)
}

func intPtr(n int) *int { return &n }
