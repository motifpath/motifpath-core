package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

func TestAdminOutboxService_RetryEntry_ForwardsCallerTokenToAuthorization(t *testing.T) {
	resolver := &fakeRoleResolver{role: "admin"}
	svc := application.NewAdminOutboxService(
		newFakeOutboxRepository(), newFakeRepository(), newFakePublisher(nil),
		application.NewAdminAuthorizer(resolver),
	)

	_, _ = svc.RetryEntry(context.Background(), "caller-token-xyz", "any-event")

	assert.Equal(t, "caller-token-xyz", resolver.lastToken)
}

func TestAdminOutboxService_RetryEntry_RefusedForNonAdminBeforeTouchingState(t *testing.T) {
	outbox := newFakeOutboxRepository()
	outbox.getErr = errors.New("outbox must not be read when authorization fails")
	publisher := newFakePublisher(nil)
	svc := application.NewAdminOutboxService(
		outbox, newFakeRepository(), publisher,
		application.NewAdminAuthorizer(&fakeRoleResolver{role: "student"}),
	)

	_, err := svc.RetryEntry(context.Background(), "caller-token", "any-event")

	require.ErrorIs(t, err, domain.ErrForbidden)
	select {
	case <-publisher.calls:
		t.Fatal("Publish must not be called when authorization fails")
	default:
	}
}

func TestAdminOutboxService_RetryEntry_FailsClosedWhenRoleUnavailable(t *testing.T) {
	svc := application.NewAdminOutboxService(
		newFakeOutboxRepository(), newFakeRepository(), newFakePublisher(nil),
		application.NewAdminAuthorizer(&fakeRoleResolver{err: ports.ErrRoleUnavailable}),
	)

	_, err := svc.RetryEntry(context.Background(), "caller-token", "any-event")

	require.ErrorIs(t, err, domain.ErrAuthorizationUnavailable)
}

func TestAdminOutboxService_ResolveEntry_RefusedForNonAdminBeforeTouchingState(t *testing.T) {
	outbox := newFakeOutboxRepository()
	outbox.getErr = errors.New("outbox must not be read when authorization fails")
	svc := application.NewAdminOutboxService(
		outbox, newFakeRepository(), newFakePublisher(nil),
		application.NewAdminAuthorizer(&fakeRoleResolver{err: ports.ErrIdentityNotRegistered}),
	)

	_, err := svc.ResolveEntry(context.Background(), "caller-token", "any-event", "cleanup")

	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestAdminOutboxService_ResolveEntry_FailsClosedWhenRoleUnavailable(t *testing.T) {
	svc := application.NewAdminOutboxService(
		newFakeOutboxRepository(), newFakeRepository(), newFakePublisher(nil),
		application.NewAdminAuthorizer(&fakeRoleResolver{err: ports.ErrRoleUnavailable}),
	)

	_, err := svc.ResolveEntry(context.Background(), "caller-token", "any-event", "")

	require.ErrorIs(t, err, domain.ErrAuthorizationUnavailable)
}

func TestAdminOutboxService_RetryEntry_SucceedsAndMarksPublished(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	event := newEvent(domain.EventTypeLessonStarted)
	eventID := event.Base().EventID
	_, _, err := repo.Save(context.Background(), event)
	require.NoError(t, err)
	require.NoError(t, outbox.Create(context.Background(), eventID))
	entryBeforeRetry, _, _ := outbox.Get(context.Background(), eventID)
	entryBeforeRetry.Status = domain.OutboxStatusDead
	require.NoError(t, outbox.Update(context.Background(), entryBeforeRetry))

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	entry, err := svc.RetryEntry(context.Background(), "caller-token", eventID)

	require.NoError(t, err)
	assert.Equal(t, domain.OutboxStatusPublished, entry.Status)
	waitForPublish(t, publisher.calls)

	stored, found := outbox.snapshot(eventID)
	require.True(t, found)
	assert.Equal(t, domain.OutboxStatusPublished, stored.Status)
}

func TestAdminOutboxService_RetryEntry_FailureLeavesEntryDead(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(errors.New("kafka still unavailable"))
	event := newEvent(domain.EventTypeLessonStarted)
	eventID := event.Base().EventID
	_, _, err := repo.Save(context.Background(), event)
	require.NoError(t, err)
	require.NoError(t, outbox.Create(context.Background(), eventID))

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	entry, err := svc.RetryEntry(context.Background(), "caller-token", eventID)

	require.NoError(t, err, "a failed manual retry is a handled outcome, not an application error")
	assert.Equal(t, domain.OutboxStatusDead, entry.Status)

	stored, found := outbox.snapshot(eventID)
	require.True(t, found)
	assert.Equal(t, domain.OutboxStatusDead, stored.Status)
	assert.Equal(t, "kafka still unavailable", stored.LastError)
}

func TestAdminOutboxService_RetryEntry_NoOpWhenAlreadyPublished(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	eventID := "already-published-event"
	require.NoError(t, outbox.Create(context.Background(), eventID))
	require.NoError(t, outbox.MarkPublished(context.Background(), eventID))

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	entry, err := svc.RetryEntry(context.Background(), "caller-token", eventID)

	require.NoError(t, err)
	assert.Equal(t, domain.OutboxStatusPublished, entry.Status)
	select {
	case <-publisher.calls:
		t.Fatal("Publish must not be called for an already-published entry")
	default:
	}
}

func TestAdminOutboxService_RetryEntry_NotFound(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	_, err := svc.RetryEntry(context.Background(), "caller-token", "does-not-exist")

	require.ErrorIs(t, err, domain.ErrOutboxEntryNotFound)
}

func TestAdminOutboxService_ResolveEntry_MarksResolvedWithoutPublishing(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	eventID := "dead-event"
	require.NoError(t, outbox.Create(context.Background(), eventID))
	entry, _, _ := outbox.Get(context.Background(), eventID)
	entry.Status = domain.OutboxStatusDead
	require.NoError(t, outbox.Update(context.Background(), entry))

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	result, err := svc.ResolveEntry(context.Background(), "caller-token", eventID, "verified delivered out of band")

	require.NoError(t, err)
	assert.Equal(t, domain.OutboxStatusResolvedManually, result.Status)
	select {
	case <-publisher.calls:
		t.Fatal("resolve must never attempt to publish")
	default:
	}

	stored, found := outbox.snapshot(eventID)
	require.True(t, found)
	assert.Equal(t, domain.OutboxStatusResolvedManually, stored.Status)
}

func TestAdminOutboxService_ResolveEntry_NoOpWhenAlreadyResolved(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	eventID := "resolved-event"
	require.NoError(t, outbox.Create(context.Background(), eventID))
	require.NoError(t, outbox.MarkResolvedManually(context.Background(), eventID, "first reason"))

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	entry, err := svc.ResolveEntry(context.Background(), "caller-token", eventID, "second reason")

	require.NoError(t, err)
	assert.Equal(t, domain.OutboxStatusResolvedManually, entry.Status)
}

func TestAdminOutboxService_ResolveEntry_NotFound(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	_, err := svc.ResolveEntry(context.Background(), "caller-token", "does-not-exist", "")

	require.ErrorIs(t, err, domain.ErrOutboxEntryNotFound)
}

func TestAdminOutboxService_RetryEntry_PropagatesGetError(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	outbox.getErr = errors.New("mongo unavailable")
	publisher := newFakePublisher(nil)

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	_, err := svc.RetryEntry(context.Background(), "caller-token", "some-event")

	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrOutboxEntryNotFound)
}

func TestAdminOutboxService_RetryEntry_PropagatesFindEventError(t *testing.T) {
	repo := newFakeRepository()
	repo.findErr = errors.New("mongo unavailable")
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	eventID := "some-event"
	require.NoError(t, outbox.Create(context.Background(), eventID))

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	_, err := svc.RetryEntry(context.Background(), "caller-token", eventID)

	require.Error(t, err)
}

func TestAdminOutboxService_RetryEntry_PropagatesUpdateErrorOnFailedPublish(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(errors.New("kafka unavailable"))
	event := newEvent(domain.EventTypeLessonStarted)
	eventID := event.Base().EventID
	_, _, err := repo.Save(context.Background(), event)
	require.NoError(t, err)
	require.NoError(t, outbox.Create(context.Background(), eventID))
	outbox.updateErr = errors.New("mongo unavailable")

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	_, err = svc.RetryEntry(context.Background(), "caller-token", eventID)

	require.Error(t, err)
}

func TestAdminOutboxService_RetryEntry_PropagatesMarkPublishedError(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	event := newEvent(domain.EventTypeLessonStarted)
	eventID := event.Base().EventID
	_, _, err := repo.Save(context.Background(), event)
	require.NoError(t, err)
	require.NoError(t, outbox.Create(context.Background(), eventID))
	outbox.markPublishedErr = errors.New("mongo unavailable")

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	_, err = svc.RetryEntry(context.Background(), "caller-token", eventID)

	require.Error(t, err)
}

func TestAdminOutboxService_ResolveEntry_PropagatesGetError(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	outbox.getErr = errors.New("mongo unavailable")
	publisher := newFakePublisher(nil)

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	_, err := svc.ResolveEntry(context.Background(), "caller-token", "some-event", "")

	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrOutboxEntryNotFound)
}

func TestAdminOutboxService_ResolveEntry_PropagatesMarkResolvedError(t *testing.T) {
	repo := newFakeRepository()
	outbox := newFakeOutboxRepository()
	publisher := newFakePublisher(nil)
	eventID := "some-event"
	require.NoError(t, outbox.Create(context.Background(), eventID))
	outbox.markResolvedErr = errors.New("mongo unavailable")

	svc := application.NewAdminOutboxService(outbox, repo, publisher, adminAuthorizer())
	_, err := svc.ResolveEntry(context.Background(), "caller-token", eventID, "")

	require.Error(t, err)
}
