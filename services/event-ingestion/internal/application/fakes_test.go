package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

var fakeReceivedAt = time.Date(2026, 8, 25, 12, 0, 5, 0, time.UTC)

var errNotFound = errors.New("event not found")

// fakeRepository is a minimal in-memory ports.EventRepository. Save reports
// alreadyExisted based on whether eventID has been seen before, mirroring
// MongoEventRepository's real duplicate-key behavior closely enough for
// application-layer tests.
type fakeRepository struct {
	mu      sync.Mutex
	saveErr error
	findErr error
	saved   []domain.TrackingEvent
	byID    map[string]domain.TrackingEvent
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{byID: map[string]domain.TrackingEvent{}}
}

func (f *fakeRepository) Save(_ context.Context, event domain.TrackingEvent) (time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return time.Time{}, false, f.saveErr
	}

	eventID := event.Base().EventID
	_, alreadyExisted := f.byID[eventID]
	if !alreadyExisted {
		f.byID[eventID] = event
	}
	f.saved = append(f.saved, event)
	return fakeReceivedAt, alreadyExisted, nil
}

func (f *fakeRepository) FindByEventID(_ context.Context, eventID string) (domain.TrackingEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.findErr != nil {
		return nil, f.findErr
	}
	event, ok := f.byID[eventID]
	if !ok {
		return nil, errNotFound
	}
	return event, nil
}

// fakeOutboxRepository is a minimal in-memory ports.PublishOutboxRepository.
type fakeOutboxRepository struct {
	mu               sync.Mutex
	entries          map[string]ports.OutboxEntry
	createCalls      int
	createErr        error
	getErr           error
	updateErr        error
	markPublishedErr error
	markResolvedErr  error
	listErr          error
}

func newFakeOutboxRepository() *fakeOutboxRepository {
	return &fakeOutboxRepository{entries: map[string]ports.OutboxEntry{}}
}

func (f *fakeOutboxRepository) Create(_ context.Context, eventID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.createCalls++
	f.entries[eventID] = ports.OutboxEntry{EventID: eventID, Status: domain.OutboxStatusPending}
	return nil
}

func (f *fakeOutboxRepository) Get(_ context.Context, eventID string) (ports.OutboxEntry, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return ports.OutboxEntry{}, false, f.getErr
	}
	entry, found := f.entries[eventID]
	return entry, found, nil
}

func (f *fakeOutboxRepository) MarkPublished(_ context.Context, eventID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markPublishedErr != nil {
		return f.markPublishedErr
	}
	entry := f.entries[eventID]
	entry.Status = domain.OutboxStatusPublished
	f.entries[eventID] = entry
	return nil
}

func (f *fakeOutboxRepository) Update(_ context.Context, entry ports.OutboxEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updateErr != nil {
		return f.updateErr
	}
	f.entries[entry.EventID] = entry
	return nil
}

func (f *fakeOutboxRepository) MarkResolvedManually(_ context.Context, eventID string, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markResolvedErr != nil {
		return f.markResolvedErr
	}
	entry := f.entries[eventID]
	entry.Status = domain.OutboxStatusResolvedManually
	entry.LastError = reason // reused field for test visibility only
	f.entries[eventID] = entry
	return nil
}

func (f *fakeOutboxRepository) ListDueForRetry(_ context.Context, now time.Time) ([]ports.OutboxEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	var due []ports.OutboxEntry
	for _, entry := range f.entries {
		if entry.Status == domain.OutboxStatusPending && !entry.NextAttemptAt.After(now) {
			due = append(due, entry)
		}
	}
	return due, nil
}

func (f *fakeOutboxRepository) snapshot(eventID string) (ports.OutboxEntry, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, ok := f.entries[eventID]
	return entry, ok
}

type fakePublisher struct {
	publishErr error
	calls      chan domain.TrackingEvent
}

func newFakePublisher(publishErr error) *fakePublisher {
	return &fakePublisher{publishErr: publishErr, calls: make(chan domain.TrackingEvent, 8)}
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

// fakeProfileResolver is a minimal in-memory ports.ProfileResolver. It returns
// err when set, otherwise profile, and records the last bearer token given.
type fakeProfileResolver struct {
	profile   ports.Profile
	err       error
	lastToken string
}

func (f *fakeProfileResolver) ResolveProfile(_ context.Context, bearerToken string) (ports.Profile, error) {
	f.lastToken = bearerToken
	if f.err != nil {
		return ports.Profile{}, f.err
	}
	return f.profile, nil
}

// adminAuthorizer builds an AdminAuthorizer that always resolves the caller as
// an admin -- the default for outbox tests that are not about authorization.
func adminAuthorizer() *application.AdminAuthorizer {
	return application.NewAdminAuthorizer(&fakeProfileResolver{profile: ports.Profile{UserID: "admin-user", Role: "admin"}})
}

// callerUserID is the StudentID every test event helper stamps -- pass it as
// IngestEventService.Ingest's callerUserID so the identity check passes.
const callerUserID = "22222222-2222-2222-2222-222222222222"

func newEvent(eventType domain.EventType) domain.TrackingEvent {
	return newEventWithID(eventType, "11111111-1111-1111-1111-111111111111")
}

func newEventWithID(eventType domain.EventType, eventID string) domain.TrackingEvent {
	base := domain.TrackingEventBase{
		EventID:    eventID,
		EventType:  eventType,
		StudentID:  callerUserID,
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
