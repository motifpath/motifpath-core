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
	"github.com/motifpath/event-ingestion/internal/ports"
)

func TestRetrySweepService_SweepOnce_RepublishesDueEntry(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	event := newEvent(domain.EventTypeLessonStarted)
	eventID := event.Base().EventID

	_, _, err := repo.Save(context.Background(), event)
	require.NoError(t, err)
	require.NoError(t, outbox.Create(context.Background(), eventID))
	// Simulate a prior failed attempt that's now due for retry.
	entry, _, _ := outbox.Get(context.Background(), eventID)
	entry.Attempts = 1
	entry.LastError = "kafka unavailable"
	entry.NextAttemptAt = time.Now().Add(-time.Minute)
	require.NoError(t, outbox.Update(context.Background(), entry))

	sweep := application.NewRetrySweepService(outbox, repo, publisher, testLogger())
	sweep.SweepOnce(context.Background())

	republished := waitForPublish(t, publisher.calls)
	assert.Equal(t, event, republished)

	updated, found := outbox.snapshot(eventID)
	require.True(t, found)
	assert.Equal(t, domain.OutboxStatusPublished, updated.Status)
}

func TestRetrySweepService_SweepOnce_IgnoresEntriesNotYetDue(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	event := newEvent(domain.EventTypeLessonStarted)
	eventID := event.Base().EventID

	_, _, err := repo.Save(context.Background(), event)
	require.NoError(t, err)
	require.NoError(t, outbox.Create(context.Background(), eventID))
	entry, _, _ := outbox.Get(context.Background(), eventID)
	entry.NextAttemptAt = time.Now().Add(time.Hour) // far in the future
	require.NoError(t, outbox.Update(context.Background(), entry))

	sweep := application.NewRetrySweepService(outbox, repo, publisher, testLogger())
	sweep.SweepOnce(context.Background())

	select {
	case <-publisher.calls:
		t.Fatal("an entry not yet due must not be retried")
	default:
	}
}

func TestRetrySweepService_SweepOnce_MarksDeadAfterMaxAttempts(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(errors.New("kafka unavailable"))
	event := newEvent(domain.EventTypeLessonStarted)
	eventID := event.Base().EventID

	_, _, err := repo.Save(context.Background(), event)
	require.NoError(t, err)
	require.NoError(t, outbox.Create(context.Background(), eventID))
	entry, _, _ := outbox.Get(context.Background(), eventID)
	entry.Attempts = domain.MaxPublishAttempts - 1
	entry.NextAttemptAt = time.Now().Add(-time.Minute)
	require.NoError(t, outbox.Update(context.Background(), entry))

	sweep := application.NewRetrySweepService(outbox, repo, publisher, testLogger())
	sweep.SweepOnce(context.Background())

	waitForPublish(t, publisher.calls)

	updated, found := outbox.snapshot(eventID)
	require.True(t, found)
	assert.Equal(t, domain.OutboxStatusDead, updated.Status)
	assert.Equal(t, domain.MaxPublishAttempts, updated.Attempts)
}

func TestRetrySweepService_SweepOnce_SkipsEventItCannotFind(t *testing.T) {
	// An outbox entry with no corresponding events document is an
	// inconsistent state that should never happen in practice, but the
	// sweep must not panic or crash on it.
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)

	require.NoError(t, outbox.Create(context.Background(), "missing-event-id"))
	entry, _, _ := outbox.Get(context.Background(), "missing-event-id")
	entry.NextAttemptAt = time.Now().Add(-time.Minute)
	require.NoError(t, outbox.Update(context.Background(), entry))

	sweep := application.NewRetrySweepService(outbox, repo, publisher, testLogger())
	require.NotPanics(t, func() {
		sweep.SweepOnce(context.Background())
	})

	select {
	case <-publisher.calls:
		t.Fatal("Publish must not be called when the underlying event can't be found")
	default:
	}
}

func TestRetrySweepService_SweepOnce_ListErrorDoesNotPanic(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	outbox.listErr = errors.New("mongo unavailable")
	publisher := newFakePublisher(nil)

	sweep := application.NewRetrySweepService(outbox, repo, publisher, testLogger())
	require.NotPanics(t, func() {
		sweep.SweepOnce(context.Background())
	})
}

func TestRetrySweepService_SweepOnce_MarkPublishedFailureDoesNotPanic(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	event := newEvent(domain.EventTypeLessonStarted)
	eventID := event.Base().EventID

	_, _, err := repo.Save(context.Background(), event)
	require.NoError(t, err)
	require.NoError(t, outbox.Create(context.Background(), eventID))
	entry, _, _ := outbox.Get(context.Background(), eventID)
	entry.NextAttemptAt = time.Now().Add(-time.Minute)
	require.NoError(t, outbox.Update(context.Background(), entry))
	outbox.markPublishedErr = errors.New("mongo unavailable")

	sweep := application.NewRetrySweepService(outbox, repo, publisher, testLogger())
	require.NotPanics(t, func() {
		sweep.SweepOnce(context.Background())
	})
	waitForPublish(t, publisher.calls)
}

func TestRetrySweepService_Run_ReturnsWhenContextAlreadyCancelled(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	sweep := application.NewRetrySweepService(outbox, repo, publisher, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		sweep.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run must return promptly once ctx is cancelled")
	}
}

var _ ports.PublishOutboxRepository = (*fakeOutboxRepository)(nil)
