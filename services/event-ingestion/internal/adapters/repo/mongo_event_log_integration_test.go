//go:build integration

package repo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/motifpath/event-ingestion/internal/domain"
)

func TestMongoEventRepository_List(t *testing.T) {
	repo := setupMongoRepository(t)
	ctx := context.Background()
	start := time.Date(2026, 10, 14, 7, 0, 0, 0, time.UTC)

	// store keeps event as received at receivedAt, the way Save would have.
	store := func(event domain.TrackingEvent, receivedAt time.Time) domain.LoggedEvent {
		_, err := repo.collection.InsertOne(ctx, toDocument(event, receivedAt))
		require.NoError(t, err)
		return domain.LoggedEvent{ReceivedAt: receivedAt, Event: event}
	}
	lessonStarted := func(n int, student string) domain.TrackingEvent {
		return domain.LessonStartedEvent{
			TrackingEventBase: domain.TrackingEventBase{
				EventID: fmt.Sprintf("00000000-0000-4000-8000-%012d", n), EventType: domain.EventTypeLessonStarted,
				StudentID: student, SessionID: "33333333-3333-3333-3333-333333333333", OccurredAt: start,
			},
			ContentContext: domain.ContentContext{ContentNodeID: "44444444-4444-4444-4444-444444444444", ContentType: domain.ContentTypeArticle},
		}
	}
	const alice, bruno = "aaaaaaaa-0000-4000-8000-000000000001", "bbbbbbbb-0000-4000-8000-000000000002"
	sessionEnded := domain.PracticeSessionEndedEvent{
		TrackingEventBase: domain.TrackingEventBase{
			EventID: "00000000-0000-4000-8000-000000000099", EventType: domain.EventTypePracticeSessionEnded,
			StudentID: alice, SessionID: "33333333-3333-3333-3333-333333333333", OccurredAt: start,
		},
		PracticeSessionID: "55555555-5555-4555-8555-555555555555", AnsweredCount: 9, FeltRatings: []domain.FeltRating{},
	}

	first := store(lessonStarted(1, alice), start.Add(1*time.Second))
	tieLow := store(lessonStarted(2, bruno), start.Add(2*time.Second))
	tieHigh := store(lessonStarted(3, alice), start.Add(2*time.Second))
	ended := store(sessionEnded, start.Add(3*time.Second))
	// A retired event type, still in the append-only log, received last.
	_, err := repo.collection.InsertOne(ctx, bson.D{
		{Key: "event_id", Value: "00000000-0000-4000-8000-000000000100"}, {Key: "event_type", Value: "exercise.answer_sent"},
		{Key: "student_id", Value: alice}, {Key: "session_id", Value: "33333333-3333-3333-3333-333333333333"},
		{Key: "occurred_at", Value: start}, {Key: "received_at", Value: start.Add(4 * time.Second)},
	})
	require.NoError(t, err)

	cases := []struct {
		name      string
		query     domain.EventLogQuery
		want      []domain.LoggedEvent
		wantTotal int
	}{
		{"newest received first, a tie by event id descending, without retired event types", domain.EventLogQuery{Limit: 20},
			[]domain.LoggedEvent{ended, tieHigh, tieLow, first}, 4},
		{"a page further on", domain.EventLogQuery{Limit: 2, Offset: 2}, []domain.LoggedEvent{tieLow, first}, 4},
		{"past the end is empty", domain.EventLogQuery{Limit: 2, Offset: 10}, []domain.LoggedEvent{}, 4},
		{"one student's", domain.EventLogQuery{StudentID: bruno, Limit: 20}, []domain.LoggedEvent{tieLow}, 1},
		{"one group of types", domain.EventLogQuery{StudentID: alice, Limit: 20,
			EventTypes: []domain.EventType{domain.EventTypePracticeSessionStarted, domain.EventTypePracticeSessionEnded}},
			[]domain.LoggedEvent{ended}, 1},
		{"a student with none", domain.EventLogQuery{StudentID: "cccccccc-0000-4000-8000-000000000003", Limit: 20}, []domain.LoggedEvent{}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, total, err := repo.List(ctx, c.query)

			require.NoError(t, err)
			assert.Equal(t, c.want, got)
			assert.Equal(t, c.wantTotal, total)
		})
	}
}
