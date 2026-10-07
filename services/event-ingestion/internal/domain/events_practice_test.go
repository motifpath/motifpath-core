package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/event-ingestion/internal/domain"
)

func TestValidPracticeItemKey(t *testing.T) {
	const id = "6ea2d087-ab9c-59dc-9657-8546025414d2"
	const other = "1f0c0d1e-5555-4555-8555-555555555555"

	cases := []struct {
		key  string
		want bool
	}{
		{"fretboard_cell:" + id + ":5:3", true},
		{"fretboard_cell:" + id + ":1:0", true},
		{"fretboard_cell:" + id + ":12:21", true},
		{"exercise:" + id, true},
		{"play_along:" + id, true},
		{"chord_change:" + id + ":" + other, true},
		{"diagram_shape:" + id, true},

		{"", false},
		{"fretboard:" + id + ":5:3", false},
		{"fretboard_cell:" + id + ":0:3", false},
		{"fretboard_cell:" + id + ":05:3", false},
		{"fretboard_cell:" + id + ":5:03", false},
		{"fretboard_cell:" + id + ":5", false},
		{"exercise:" + "6EA2D087-AB9C-59DC-9657-8546025414D2", false},
		{"exercise:not-a-uuid", false},
		{"play_along:" + id + ":90", false},
		{"chord_change:" + id, false},
		{"diagram_shape:" + id + ":" + other, false},
		{" exercise:" + id, false},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			assert.Equal(t, tc.want, domain.ValidPracticeItemKey(tc.key))
		})
	}
}

func TestPracticeResponse_IsTimed(t *testing.T) {
	latency := 1800
	tempo := 90

	assert.True(t, domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: &latency}.IsTimed())
	assert.True(t, domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice, LatencyMs: &latency}.IsTimed())
	assert.False(t, domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: domain.SelfRatingClean, TempoBPM: &tempo}.IsTimed())
}
