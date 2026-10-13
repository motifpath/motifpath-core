package http

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// reviewedContext reads the review marker off whichever context event carries.
func reviewedContext(t *testing.T, event domain.TrackingEvent) bool {
	t.Helper()
	switch e := event.(type) {
	case domain.LessonStartedEvent:
		return e.ContentContext.Review
	case domain.LessonResumedEvent:
		return e.ContentContext.Review
	case domain.LessonCompletedEvent:
		return e.ContentContext.Review
	case domain.ExerciseStartedEvent:
		return e.TriggerContext.Review
	case domain.ExerciseProgressEvent:
		return e.TriggerContext.Review
	case domain.ExerciseEndedEvent:
		return e.TriggerContext.Review
	case domain.PracticeItemAnsweredEvent:
		require.NotNil(t, e.TriggerContext)
		return e.TriggerContext.Review
	default:
		t.Fatalf("no context on %T", event)
		return false
	}
}

func TestToDomainEvent_ReviewMarker(t *testing.T) {
	content := func(review string) string {
		return fmt.Sprintf(`,"content_context":{"content_node_id":%q,"content_type":"video"%s}`, testNodeID, review)
	}
	trigger := func(review string) string {
		return fmt.Sprintf(`,"exercise_id":%q,"trigger_context":{"source":"challenge_sequence","content_node_id":%q,"challenge_id":%q%s}`,
			testDiagramID, testNodeID, testChallengeID, review)
	}
	answered := func(review string) string {
		return fmt.Sprintf(`,"item_key":%q,"response":{"response_type":"name_the_note","note_name":"C","latency_ms":1800},"trigger_context":{"source":"challenge_sequence","challenge_id":%q%s}`,
			testCellKey, testChallengeID, review)
	}

	cases := []struct {
		name       string
		eventType  string
		fields     string
		wantReview bool
	}{
		{"lesson.started marked as review", "lesson.started", content(`,"review":true`), true},
		{"lesson.resumed marked as review", "lesson.resumed", content(`,"review":true`), true},
		{"lesson.completed marked as review", "lesson.completed", content(`,"review":true`), true},
		{"lesson.started marked false", "lesson.started", content(`,"review":false`), false},
		{"lesson.started without the marker", "lesson.started", content(``), false},
		{"exercise.started marked as review", "exercise.started", trigger(`,"review":true`), true},
		{"exercise.progress marked as review", "exercise.progress", trigger(`,"review":true`) + `,"elapsed_seconds":30`, true},
		{"exercise.ended marked as review", "exercise.ended", trigger(`,"review":true`) + `,"outcome":"completed","final_score":100`, true},
		{"exercise.ended without the marker", "exercise.ended", trigger(``) + `,"outcome":"abandoned"`, false},
		{"an answer outside a session marked as review", "practice.item_answered", answered(`,"review":true`), true},
		{"an answer outside a session without the marker", "practice.item_answered", answered(``), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event, err := toDomainEvent(practiceBody(t, tc.eventType, tc.fields))

			require.NoError(t, err)
			assert.Equal(t, tc.wantReview, reviewedContext(t, event))
		})
	}
}

func TestToDomainEvent_ReviewMarkerMustBeTrueOrFalse(t *testing.T) {
	cases := []struct {
		name      string
		eventType string
		fields    string
		wantField string
	}{
		{"a trigger context marked \"yes\"", "exercise.started",
			fmt.Sprintf(`,"exercise_id":%q,"trigger_context":{"source":"challenge_sequence","review":"yes"}`, testDiagramID), "trigger_context.review"},
		{"a content context marked 1", "lesson.started",
			fmt.Sprintf(`,"content_context":{"content_node_id":%q,"review":1}`, testNodeID), "content_context.review"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := toDomainEvent(practiceBody(t, tc.eventType, tc.fields))

			require.ErrorIs(t, err, domain.ErrInvalidField)
			assert.Contains(t, err.Error(), tc.wantField)
		})
	}
}
