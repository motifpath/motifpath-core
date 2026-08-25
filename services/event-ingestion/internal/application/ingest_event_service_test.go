package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
)

var fakeReceivedAt = time.Date(2026, 8, 25, 12, 0, 5, 0, time.UTC)

type fakeRepository struct {
	saveErr error
	saved   []domain.TrackingEvent
}

func (f *fakeRepository) Save(_ context.Context, event domain.TrackingEvent) (time.Time, error) {
	if f.saveErr != nil {
		return time.Time{}, f.saveErr
	}
	f.saved = append(f.saved, event)
	return fakeReceivedAt, nil
}

type fakePublisher struct {
	publishErr error
	calls      chan domain.TrackingEvent
}

func newFakePublisher(publishErr error) *fakePublisher {
	return &fakePublisher{publishErr: publishErr, calls: make(chan domain.TrackingEvent, 1)}
}

func (f *fakePublisher) Publish(_ context.Context, event domain.TrackingEvent) error {
	f.calls <- event
	return f.publishErr
}

func waitForPublish(t *testing.T, calls <-chan domain.TrackingEvent) domain.TrackingEvent {
	t.Helper()
	select {
	case event := <-calls:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Publish to be called")
		return nil
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newEvent(eventType domain.EventType) domain.TrackingEvent {
	base := domain.TrackingEventBase{
		EventID:    "11111111-1111-1111-1111-111111111111",
		EventType:  eventType,
		StudentID:  "22222222-2222-2222-2222-222222222222",
		SessionID:  "33333333-3333-3333-3333-333333333333",
		OccurredAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
	}
	trigger := domain.TriggerContext{Source: domain.TriggerSourceFreePractice}

	switch eventType {
	case domain.EventTypeLessonStarted:
		return domain.LessonStartedEvent{TrackingEventBase: base, ContentContext: domain.ContentContext{ContentNodeID: "node-1"}}
	case domain.EventTypeLessonResumed:
		return domain.LessonResumedEvent{TrackingEventBase: base, ContentContext: domain.ContentContext{ContentNodeID: "node-1"}}
	case domain.EventTypeLessonCompleted:
		return domain.LessonCompletedEvent{TrackingEventBase: base, ContentContext: domain.ContentContext{ContentNodeID: "node-1"}}
	case domain.EventTypeExerciseStarted:
		return domain.ExerciseStartedEvent{TrackingEventBase: base, ExerciseID: "ex-1", TriggerContext: trigger}
	case domain.EventTypeExerciseProgress:
		return domain.ExerciseProgressEvent{TrackingEventBase: base, ExerciseID: "ex-1", TriggerContext: trigger}
	case domain.EventTypeExerciseAnswerSent:
		return domain.ExerciseAnswerSentEvent{TrackingEventBase: base, ExerciseID: "ex-1", TriggerContext: trigger, AttemptNumber: 1}
	case domain.EventTypeExerciseEnded:
		return domain.ExerciseEndedEvent{TrackingEventBase: base, ExerciseID: "ex-1", TriggerContext: trigger, Outcome: domain.ExerciseOutcomeCompleted}
	default:
		panic("unhandled event type in test helper: " + string(eventType))
	}
}

func TestIngestEventService_Ingest_HappyPath(t *testing.T) {
	eventTypes := []domain.EventType{
		domain.EventTypeLessonStarted,
		domain.EventTypeLessonResumed,
		domain.EventTypeLessonCompleted,
		domain.EventTypeExerciseStarted,
		domain.EventTypeExerciseProgress,
		domain.EventTypeExerciseAnswerSent,
		domain.EventTypeExerciseEnded,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			repo := &fakeRepository{}
			publisher := newFakePublisher(nil)
			svc := application.NewIngestEventService(repo, publisher, testLogger())
			event := newEvent(eventType)

			receivedAt, err := svc.Ingest(context.Background(), event)

			require.NoError(t, err)
			assert.Equal(t, fakeReceivedAt, receivedAt)
			require.Len(t, repo.saved, 1)
			assert.Equal(t, event, repo.saved[0])
			assert.Equal(t, event, waitForPublish(t, publisher.calls))
		})
	}
}

func TestIngestEventService_Ingest_Idempotency(t *testing.T) {
	// EventRepository.Save is documented as idempotent on EventID; Ingest must not
	// layer its own duplicate-rejection logic on top of that contract.
	repo := &fakeRepository{}
	publisher := newFakePublisher(nil)
	svc := application.NewIngestEventService(repo, publisher, testLogger())
	event := newEvent(domain.EventTypeLessonStarted)

	_, err1 := svc.Ingest(context.Background(), event)
	waitForPublish(t, publisher.calls)
	_, err2 := svc.Ingest(context.Background(), event)
	waitForPublish(t, publisher.calls)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Len(t, repo.saved, 2)
}

func TestIngestEventService_Ingest_RepositoryFailure(t *testing.T) {
	repo := &fakeRepository{saveErr: errors.New("connection refused")}
	publisher := newFakePublisher(nil)
	svc := application.NewIngestEventService(repo, publisher, testLogger())

	_, err := svc.Ingest(context.Background(), newEvent(domain.EventTypeLessonStarted))

	require.Error(t, err)
	select {
	case <-publisher.calls:
		t.Fatal("Publish must not be called when Save fails")
	default:
	}
}

func TestIngestEventService_Ingest_PublishFailureDoesNotFailRequest(t *testing.T) {
	// The 202 response confirms durable receipt only — Kafka delivery is asynchronous
	// and its failure must not be reflected in Ingest's return value.
	repo := &fakeRepository{}
	publisher := newFakePublisher(errors.New("kafka unavailable"))
	svc := application.NewIngestEventService(repo, publisher, testLogger())

	_, err := svc.Ingest(context.Background(), newEvent(domain.EventTypeLessonStarted))

	require.NoError(t, err)
	waitForPublish(t, publisher.calls)
}
