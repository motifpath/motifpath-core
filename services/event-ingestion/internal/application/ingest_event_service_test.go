package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
)

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
			repo := newFakeRepository()
			outbox := newFakeOutboxRepository()
			publisher := newFakePublisher(nil)
			svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())
			event := newEvent(eventType)

			receivedAt, err := svc.Ingest(context.Background(), callerUserID, event)

			require.NoError(t, err)
			assert.Equal(t, fakeReceivedAt, receivedAt)
			require.Len(t, repo.saved, 1)
			assert.Equal(t, event, repo.saved[0])
			assert.Equal(t, event, waitForPublish(t, publisher.calls))

			assert.Equal(t, 1, outbox.createCalls)
			require.Eventually(t, func() bool {
				entry, found := outbox.snapshot(event.Base().EventID)
				return found && entry.Status == domain.OutboxStatusPublished
			}, time.Second, 10*time.Millisecond, "outbox entry must be marked published after a successful publish")
		})
	}
}

func TestIngestEventService_Ingest_DuplicateEventIDRepublishesButDoesNotRecreateOutboxEntry(t *testing.T) {
	// Per ADR-012, a retry with the same event_id is not suppressed --
	// every consumer already has to tolerate duplicate delivery -- but a
	// duplicate must not create a second outbox entry.
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())
	event := newEvent(domain.EventTypeLessonStarted)

	_, err1 := svc.Ingest(context.Background(), callerUserID, event)
	waitForPublish(t, publisher.calls)
	_, err2 := svc.Ingest(context.Background(), callerUserID, event)
	waitForPublish(t, publisher.calls)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Len(t, repo.saved, 2, "Save is called on every attempt, duplicate or not")
	assert.Equal(t, 1, outbox.createCalls, "a duplicate event_id must not create a second outbox entry")
}

func TestIngestEventService_Ingest_RepositoryFailure(t *testing.T) {
	repo := newFakeRepository()
	repo.saveErr = errors.New("connection refused")
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())

	_, err := svc.Ingest(context.Background(), callerUserID, newEvent(domain.EventTypeLessonStarted))

	require.Error(t, err)
	assert.Zero(t, outbox.createCalls, "no outbox entry should be created when the durable write itself fails")
	select {
	case <-publisher.calls:
		t.Fatal("Publish must not be called when Save fails")
	default:
	}
}

func TestIngestEventService_Ingest_PublishFailureDoesNotFailRequest(t *testing.T) {
	// The 202 response confirms durable receipt only — Kafka delivery is asynchronous
	// and its failure must not be reflected in Ingest's return value.
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(errors.New("kafka unavailable"))
	svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())
	event := newEvent(domain.EventTypeLessonStarted)

	_, err := svc.Ingest(context.Background(), callerUserID, event)

	require.NoError(t, err)
	waitForPublish(t, publisher.calls)

	require.Eventually(t, func() bool {
		entry, found := outbox.snapshot(event.Base().EventID)
		return found && entry.Attempts == 1
	}, time.Second, 10*time.Millisecond, "a failed publish must be recorded on the outbox entry")

	entry, _ := outbox.snapshot(event.Base().EventID)
	assert.Equal(t, domain.OutboxStatusPending, entry.Status, "one failure must not exhaust the retry cap")
	assert.Equal(t, "kafka unavailable", entry.LastError)
	assert.False(t, entry.NextAttemptAt.IsZero())
}

func TestIngestEventService_Ingest_OutboxCreateFailureDoesNotFailRequest(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	outbox.createErr = errors.New("mongo unavailable")
	publisher := newFakePublisher(nil)
	svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())

	receivedAt, err := svc.Ingest(context.Background(), callerUserID, newEvent(domain.EventTypeLessonStarted))

	require.NoError(t, err)
	assert.Equal(t, fakeReceivedAt, receivedAt)
	waitForPublish(t, publisher.calls)
}

func TestIngestEventService_Ingest_PublishFailureWithNoOutboxEntryIsANoOp(t *testing.T) {
	// If the outbox entry was never created (e.g. the Create call above
	// failed), a subsequent publish failure has nothing to record against
	// -- recordPublishFailure must handle that missing-entry case without
	// erroring or panicking.
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	outbox.createErr = errors.New("mongo unavailable")
	publisher := newFakePublisher(errors.New("kafka unavailable"))
	svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())
	event := newEvent(domain.EventTypeLessonStarted)

	_, err := svc.Ingest(context.Background(), callerUserID, event)

	require.NoError(t, err)
	waitForPublish(t, publisher.calls)

	// Give the async goroutine a moment to reach recordPublishFailure.
	time.Sleep(20 * time.Millisecond)
	_, found := outbox.snapshot(event.Base().EventID)
	assert.False(t, found, "no entry should exist to update")
}

func TestIngestEventService_Ingest_RecordPublishFailure_GetErrorDoesNotPanic(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(errors.New("kafka unavailable"))
	svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())
	event := newEvent(domain.EventTypeLessonStarted)

	_, err := svc.Ingest(context.Background(), callerUserID, event)
	require.NoError(t, err)
	waitForPublish(t, publisher.calls)

	// Set the Get error only after Create has already succeeded, so the
	// failure path (which runs in a separate goroutine) is the one that
	// observes it.
	outbox.getErr = errors.New("mongo unavailable")
	time.Sleep(20 * time.Millisecond)
}

func TestIngestEventService_Ingest_RecordPublishFailure_UpdateErrorDoesNotPanic(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(errors.New("kafka unavailable"))
	svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())
	event := newEvent(domain.EventTypeLessonStarted)

	outbox.updateErr = errors.New("mongo unavailable")
	_, err := svc.Ingest(context.Background(), callerUserID, event)
	require.NoError(t, err)
	waitForPublish(t, publisher.calls)
	time.Sleep(20 * time.Millisecond)
}

func TestIngestEventService_Ingest_MarkPublishedFailureDoesNotPanic(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	outbox.markPublishedErr = errors.New("mongo unavailable")
	publisher := newFakePublisher(nil)
	svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())

	_, err := svc.Ingest(context.Background(), callerUserID, newEvent(domain.EventTypeLessonStarted))

	require.NoError(t, err)
	waitForPublish(t, publisher.calls)
	time.Sleep(20 * time.Millisecond)
}

func TestIngestEventService_Ingest_RejectsEventForAnotherStudent(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	svc := application.NewIngestEventService(repo, outbox, publisher, testLogger())
	event := newEvent(domain.EventTypeLessonStarted) // StudentID == callerUserID

	_, err := svc.Ingest(context.Background(), "a-different-user-id", event)

	require.ErrorIs(t, err, domain.ErrIdentityMismatch)
	assert.Empty(t, repo.saved, "a mismatched event must not be stored")
	select {
	case <-publisher.calls:
		t.Fatal("a mismatched event must not be published")
	default:
	}
}
