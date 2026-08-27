//go:build integration

package bdd

import (
	"context"
	"crypto/sha1" //nolint:gosec // used only for deterministic test UUIDs, not security
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	appHTTP "github.com/motifpath/event-ingestion/internal/adapters/http"
	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeRepository stands in for MongoEventRepository. Real MongoDB behavior (the
// append-only collection, the unique event_id index) is exercised by the Phase 3.7
// testcontainers integration tests; here we only need Save to succeed, matching
// EventRepository's documented idempotency contract.
type fakeRepository struct {
	mu    sync.Mutex
	saved map[string]domain.TrackingEvent
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{saved: make(map[string]domain.TrackingEvent)}
}

var fixedReceivedAt = time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

func (f *fakeRepository) Save(_ context.Context, event domain.TrackingEvent) (time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, alreadyExisted := f.saved[event.Base().EventID]
	f.saved[event.Base().EventID] = event
	return fixedReceivedAt, alreadyExisted, nil
}

func (f *fakeRepository) FindByEventID(_ context.Context, eventID string) (domain.TrackingEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	event, ok := f.saved[eventID]
	if !ok {
		return nil, errEventNotFound
	}
	return event, nil
}

var errEventNotFound = errors.New("event not found")

type fakePublisher struct {
	mu   sync.Mutex
	sent []domain.TrackingEvent
}

func (f *fakePublisher) Publish(_ context.Context, event domain.TrackingEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, event)
	return nil
}

type fakePinger struct {
	err error
}

func (f *fakePinger) Ping(_ context.Context) error {
	return f.err
}

// fakeOutboxRepository stands in for MongoPublishOutboxRepository. No BDD
// scenario exercises the admin outbox endpoints yet (ADR-012 Part 3) -- this
// exists only so IngestEventService and Handler can be constructed.
type fakeOutboxRepository struct {
	mu      sync.Mutex
	entries map[string]ports.OutboxEntry
}

func newFakeOutboxRepository() *fakeOutboxRepository {
	return &fakeOutboxRepository{entries: make(map[string]ports.OutboxEntry)}
}

func (f *fakeOutboxRepository) Create(_ context.Context, eventID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries[eventID] = ports.OutboxEntry{EventID: eventID, Status: domain.OutboxStatusPending}
	return nil
}

func (f *fakeOutboxRepository) Get(_ context.Context, eventID string) (ports.OutboxEntry, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, found := f.entries[eventID]
	return entry, found, nil
}

func (f *fakeOutboxRepository) MarkPublished(_ context.Context, eventID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.entries[eventID]
	entry.Status = domain.OutboxStatusPublished
	f.entries[eventID] = entry
	return nil
}

func (f *fakeOutboxRepository) Update(_ context.Context, entry ports.OutboxEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries[entry.EventID] = entry
	return nil
}

func (f *fakeOutboxRepository) MarkResolvedManually(_ context.Context, eventID string, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.entries[eventID]
	entry.Status = domain.OutboxStatusResolvedManually
	f.entries[eventID] = entry
	return nil
}

func (f *fakeOutboxRepository) ListDueForRetry(_ context.Context, now time.Time) ([]ports.OutboxEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var due []ports.OutboxEntry
	for _, entry := range f.entries {
		if entry.Status == domain.OutboxStatusPending && !entry.NextAttemptAt.After(now) {
			due = append(due, entry)
		}
	}
	return due, nil
}

// fakeProfileResolver stands in for the Core Domain Service call the outbox
// admin endpoints make to establish a caller's role (ADR-013). Scenarios set
// the role or err via the Given steps in steps_admin_test.go.
type fakeProfileResolver struct {
	profile ports.Profile
	err     error
}

func (f *fakeProfileResolver) ResolveProfile(_ context.Context, _ string) (ports.Profile, error) {
	if f.err != nil {
		return ports.Profile{}, f.err
	}
	return f.profile, nil
}

// fakeIdentityResolver stands in for the cached sub -> user_id resolution
// POST /events performs (ADR-014). The ingest "authenticated" steps set userID
// to the token student's id; the identity-failure steps set err.
type fakeIdentityResolver struct {
	userID string
	err    error
}

func (f *fakeIdentityResolver) ResolveUserID(_ context.Context, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.userID, nil
}

var (
	_ ports.EventRepository         = (*fakeRepository)(nil)
	_ ports.EventPublisher          = (*fakePublisher)(nil)
	_ ports.Pinger                  = (*fakePinger)(nil)
	_ ports.PublishOutboxRepository = (*fakeOutboxRepository)(nil)
	_ ports.ProfileResolver         = (*fakeProfileResolver)(nil)
	_ ports.IdentityResolver        = (*fakeIdentityResolver)(nil)
)

// exerciseAttempt records the trigger context established by a "has an active
// exercise attempt" Given step, so a later When step can build a consistent
// exercise event against it.
type exerciseAttempt struct {
	exerciseID  uuid.UUID
	challengeID *uuid.UUID // nil when the attempt has no challenge
}

func (a exerciseAttempt) triggerContext() generated.TriggerContext {
	if a.challengeID == nil {
		return generated.TriggerContext{Source: "free_practice"}
	}
	return generated.TriggerContext{Source: "challenge_sequence", ChallengeId: a.challengeID}
}

// world holds all state for a single scenario. A fresh instance is created by
// InitializeScenario for every scenario godog runs, giving each scenario full
// isolation without an explicit teardown step.
type world struct {
	repo             *fakeRepository
	outbox           *fakeOutboxRepository
	publisher        *fakePublisher
	mongoPinger      *fakePinger
	kafkaPinger      *fakePinger
	profileResolver  *fakeProfileResolver
	identityResolver *fakeIdentityResolver
	handler          *appHTTP.Handler

	hasToken       bool
	tokenStudentID string

	// admin (outbox remediation) scenario state
	hasBearerToken          bool
	adminStatusCode         int
	adminBody               []byte
	adminErr                error
	publishCountBeforeAdmin int

	exerciseAttempts map[string]exerciseAttempt

	// lastSubmittedBody lets the "submits the same event again" step resend an
	// identical payload rather than reconstructing an approximation of it.
	lastSubmittedBody *generated.TrackingEvent

	ingestResp generated.IngestTrackingEventResponseObject
	ingestErr  error

	readinessResp generated.ReadinessCheckResponseObject
	livenessResp  generated.LivenessCheckResponseObject
}

func newWorld() *world {
	w := &world{
		repo:             newFakeRepository(),
		outbox:           newFakeOutboxRepository(),
		publisher:        &fakePublisher{},
		mongoPinger:      &fakePinger{},
		kafkaPinger:      &fakePinger{},
		profileResolver:  &fakeProfileResolver{},
		identityResolver: &fakeIdentityResolver{},
		exerciseAttempts: make(map[string]exerciseAttempt),
	}
	service := application.NewIngestEventService(w.repo, w.outbox, w.publisher, discardLogger())
	authorizer := application.NewAdminAuthorizer(w.profileResolver)
	adminOutbox := application.NewAdminOutboxService(w.outbox, w.repo, w.publisher, authorizer)
	w.handler = appHTTP.NewHandler(service, adminOutbox, w.identityResolver, w.mongoPinger, w.kafkaPinger)
	return w
}

// submit runs body through the handler exactly as the generated router would,
// using the world's current auth state to build the context.
func (w *world) submit(body *generated.TrackingEvent) {
	ctx := context.Background()
	if w.hasToken {
		ctx = appHTTP.WithStudentID(ctx, w.tokenStudentID)
		ctx = appHTTP.WithBearerToken(ctx, "bdd-token")
	}
	w.lastSubmittedBody = body
	w.ingestResp, w.ingestErr = w.handler.IngestTrackingEvent(ctx, generated.IngestTrackingEventRequestObject{Body: body})
}

// deterministicUUID maps a human-readable test identifier (a student name, a
// content node slug, an event identifier from the feature file, ...) to a stable
// UUID, so the same name always produces the same ID within and across steps
// without every step needing to thread real UUIDs through Gherkin text.
func deterministicUUID(parts ...string) uuid.UUID {
	h := sha1.New() //nolint:gosec // deterministic ID derivation, not a security use
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	var id uuid.UUID
	copy(id[:], sum[:16])
	id[6] = (id[6] & 0x0f) | 0x50 // version 5
	id[8] = (id[8] & 0x3f) | 0x80 // RFC 4122 variant
	return id
}

func studentUUID(name string) uuid.UUID {
	return deterministicUUID("student", name)
}

func studentID(name string) string {
	return studentUUID(name).String()
}

var (
	fixedSessionID  = deterministicUUID("session", "bdd-fixed-session")
	fixedOccurredAt = time.Date(2026, 8, 25, 11, 55, 0, 0, time.UTC)
)
