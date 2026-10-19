package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/event-ingestion/internal/domain"
)

func TestToWireEvent_ReviewMarker(t *testing.T) {
	cases := []struct {
		name  string
		event domain.TrackingEvent
		want  string
	}{
		{
			name: "a lesson opened for review carries the marker",
			event: domain.LessonStartedEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypeLessonStarted),
				ContentContext:    domain.ContentContext{ContentNodeID: "55555555-5555-4555-8555-555555555555", Review: true},
			},
			want: `{` + wireBaseJSON + `,"event_type":"lesson.started","content_context":{"content_node_id":"55555555-5555-4555-8555-555555555555","review":true}}`,
		},
		{
			name: "a lesson opened from the current path leaves it out",
			event: domain.LessonStartedEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypeLessonStarted),
				ContentContext:    domain.ContentContext{ContentNodeID: "55555555-5555-4555-8555-555555555555"},
			},
			want: `{` + wireBaseJSON + `,"event_type":"lesson.started","content_context":{"content_node_id":"55555555-5555-4555-8555-555555555555"}}`,
		},
		{
			name: "an exercise started in review carries the marker",
			event: domain.ExerciseStartedEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypeExerciseStarted),
				ExerciseID:        "66666666-6666-4666-8666-666666666666",
				TriggerContext:    domain.TriggerContext{Source: domain.TriggerSourceChallengeSequence, Review: true},
			},
			want: `{` + wireBaseJSON + `,"event_type":"exercise.started","exercise_id":"66666666-6666-4666-8666-666666666666","trigger_context":{"source":"challenge_sequence","review":true}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.JSONEq(t, tc.want, wireJSON(t, tc.event))
		})
	}
}
