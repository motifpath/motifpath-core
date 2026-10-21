package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The full rule set is pinned by the shared golden cases (internal/bdd). These
// cover what the golden files can't: the registry, and a rating outside the enum.

func TestGraderFor(t *testing.T) {
	tests := []struct {
		kind     PracticeItemKind
		response PracticeResponseType
		want     string
	}{
		{PracticeItemKindPlayAlong, PracticeResponseSelfRating, "self_rating.v1"},
		{PracticeItemKindChordChange, PracticeResponseSelfRating, "self_rating.v1"},
		{PracticeItemKindChordVoicing, PracticeResponseSelfRating, "self_rating.v1"},
		{PracticeItemKindRootFifth, PracticeResponseSelfRating, "self_rating.v1"},
		{PracticeItemKindRootChange, PracticeResponseSelfRating, "self_rating.v1"},
		{PracticeItemKindProgressionKeys, PracticeResponseSelfRating, "self_rating.v1"},
		{PracticeItemKindExercise, PracticeResponseOptionChoice, "exercise_option.v1"},
		{PracticeItemKindFretboardCell, PracticeResponseNameTheNote, "fretboard_cell.v2"},
		{PracticeItemKindFretboardCell, PracticeResponseFindTheNote, "fretboard_cell.v2"},
		{PracticeItemKindFretboardCell, PracticeResponseMultiTap, "multi_tap.v2"},
		{PracticeItemKindDiagramShape, PracticeResponseNameTheShape, "diagram_shape.v1"},
		{PracticeItemKindDiatonicChord, PracticeResponseGeneratedChoice, "generated_choice.v1"},
		{PracticeItemKindChordFunction, PracticeResponseGeneratedChoice, "generated_choice.v1"},
		{PracticeItemKindChartProgression, PracticeResponseGeneratedChoice, "generated_choice.v1"},
		{PracticeItemKindHeardChord, PracticeResponseGeneratedChoice, "generated_choice.v1"},
		{PracticeItemKindHeardProgression, PracticeResponseGeneratedChoice, "generated_choice.v1"},
		{PracticeItemKindGuideTone, PracticeResponseTimedTaps, "timed_taps.v1"},
		{PracticeItemKindFretboardNote, PracticeResponseMultiTap, "multi_tap.v1"},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind)+"/"+string(tt.response), func(t *testing.T) {
			g, ok := GraderFor(tt.kind, tt.response)
			require.True(t, ok)
			assert.Equal(t, tt.want, g.ID())
		})
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

func TestSelfRatingGrader_RejectsAnUnknownNoteValue(t *testing.T) {
	key, err := ParsePracticeItemKey("play_along:" + diagramA)
	require.NoError(t, err)
	tempo, fifths := 90, NoteValue("fifths")
	ref := PracticeReference{Diagrams: map[string]DiagramReference{diagramA: {ID: diagramA}}}

	got := selfRatingV1{}.Grade(key, PracticeResponse{Type: PracticeResponseSelfRating, Rating: SelfRatingClean, TempoBPM: &tempo, NoteValue: &fifths}, ref)

	assert.Equal(t, GradeRejectionResponseDoesNotFitItem, got.Rejection)
}
