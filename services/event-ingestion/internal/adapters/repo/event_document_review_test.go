package repo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/domain"
)

func TestEventDocument_KeepsTheReviewMarker(t *testing.T) {
	base := func(eventType domain.EventType) domain.TrackingEventBase {
		return domain.TrackingEventBase{
			EventID: "11111111-1111-4111-8111-111111111111", EventType: eventType,
			StudentID: "22222222-2222-4222-8222-222222222222", SessionID: "33333333-3333-4333-8333-333333333333",
			OccurredAt: time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC),
		}
	}
	trigger := &domain.TriggerContext{Source: domain.TriggerSourceChallengeSequence, Review: true}

	cases := []struct {
		name  string
		event domain.TrackingEvent
	}{
		{"lesson.completed in review", domain.LessonCompletedEvent{
			TrackingEventBase: base(domain.EventTypeLessonCompleted),
			ContentContext:    domain.ContentContext{ContentNodeID: "55555555-5555-4555-8555-555555555555", Review: true},
		}},
		{"lesson.started not in review", domain.LessonStartedEvent{
			TrackingEventBase: base(domain.EventTypeLessonStarted),
			ContentContext:    domain.ContentContext{ContentNodeID: "55555555-5555-4555-8555-555555555555"},
		}},
		{"exercise.ended in review", domain.ExerciseEndedEvent{
			TrackingEventBase: base(domain.EventTypeExerciseEnded),
			ExerciseID:        "66666666-6666-4666-8666-666666666666",
			TriggerContext:    *trigger,
			Outcome:           domain.ExerciseOutcomeAbandoned,
		}},
		{"an answer outside a session in review", domain.PracticeItemAnsweredEvent{
			TrackingEventBase: base(domain.EventTypePracticeItemAnswered),
			TriggerContext:    trigger,
			ItemKey:           "exercise:66666666-6666-4666-8666-666666666666",
			Response:          domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C"},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fromDocument(toDocument(tc.event, time.Date(2026, 10, 13, 10, 0, 1, 0, time.UTC)))

			require.NoError(t, err)
			assert.Equal(t, tc.event, got)
		})
	}
}
