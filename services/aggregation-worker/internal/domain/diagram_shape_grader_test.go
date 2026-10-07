package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The full rule set is pinned by the shared golden cases (internal/bdd). These
// cover what the golden files leave implicit.

const cagedA = "928330d5-903e-572c-9d41-5fde99d51ed1"

// cagedAReference is C major's CAGED A grip at shift 3, on the guitar layout.
func cagedAReference() PracticeReference {
	ref := guitarReference()
	ref.Diagrams = map[string]DiagramReference{cagedA: {
		ID: cagedA, LayoutInstrumentID: guitarLayout,
		ShapeFamily: "caged-grip", Shape: "A", FamilyMembers: []string{"C", "A", "G", "E", "D"},
		Positions: []DiagramPosition{
			{String: 5, Fret: 3, Interval: "R"}, {String: 4, Fret: 5, Interval: "5"}, {String: 3, Fret: 5, Interval: "R"},
			{String: 2, Fret: 5, Interval: "3"}, {String: 1, Fret: 3, Interval: "5"},
		},
	}}
	return ref
}

func shapeKey(t *testing.T) PracticeItemKey {
	t.Helper()
	key, err := ParsePracticeItemKey("diagram_shape:" + cagedA)
	require.NoError(t, err)
	return key
}

func TestDiagramShapeGrader(t *testing.T) {
	latency := 2000
	two, five := 2, 5
	tests := []struct {
		name     string
		response PracticeResponse
		ref      func() PracticeReference
		want     GradeRejection
		correct  bool
	}{
		{name: "a shape named without a time is rejected", response: PracticeResponse{Type: PracticeResponseNameTheShape, Shape: "A"}, want: GradeRejectionResponseDoesNotFitItem},
		{name: "a shape named without a shape is rejected", response: PracticeResponse{Type: PracticeResponseNameTheShape, LatencyMs: &latency}, want: GradeRejectionResponseDoesNotFitItem},
		{name: "a tap without a cell is rejected", response: PracticeResponse{Type: PracticeResponseFindTheDegree, Interval: "3", String: &two, LatencyMs: &latency}, want: GradeRejectionResponseDoesNotFitItem},
		{name: "a tap without the asked degree is rejected", response: PracticeResponse{Type: PracticeResponseFindTheDegree, String: &two, Fret: &five, LatencyMs: &latency}, want: GradeRejectionResponseDoesNotFitItem},
		{name: "a tap below the open string is rejected", response: PracticeResponse{Type: PracticeResponseFindTheDegree, Interval: "3", String: &two, Fret: intPtr(-1), LatencyMs: &latency}, want: GradeRejectionInvalidCell},
		{
			name:     "a shape whose layout instrument is unknown is rejected",
			response: PracticeResponse{Type: PracticeResponseFindTheDegree, Interval: "3", String: &two, Fret: &five, LatencyMs: &latency},
			ref: func() PracticeReference {
				ref := cagedAReference()
				ref.Instruments = nil
				return ref
			},
			want: GradeRejectionUnknownReference,
		},
		{name: "the right tap is right", response: PracticeResponse{Type: PracticeResponseFindTheDegree, Interval: "3", String: &two, Fret: &five, LatencyMs: &latency}, correct: true},
		{name: "an octave of the right tap, off the shape, is wrong", response: PracticeResponse{Type: PracticeResponseFindTheDegree, Interval: "3", String: &two, Fret: intPtr(17), LatencyMs: &latency}, correct: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := cagedAReference()
			if tt.ref != nil {
				ref = tt.ref()
			}

			got := diagramShapeV1{}.Grade(shapeKey(t), tt.response, ref)

			assert.Equal(t, tt.want, got.Rejection)
			if tt.want == "" {
				require.NotNil(t, got.Evidence.Correct)
				assert.Equal(t, tt.correct, *got.Evidence.Correct)
			}
		})
	}
}

func TestDiagramShapeGrader_IsTheGraderOfDiagramShapes(t *testing.T) {
	g, ok := GraderFor(PracticeItemKindDiagramShape)

	require.True(t, ok)
	assert.Equal(t, "diagram_shape.v1", g.ID())
	assert.Contains(t, Graders(), g)
}
