//go:build integration

package bdd

import (
	"context"
	"crypto/sha1" //nolint:gosec // used only for deterministic test UUIDs, not security
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

func (f *fakeRepository) Save(_ context.Context, event domain.TrackingEvent) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved[event.Base().EventID] = event
	return fixedReceivedAt, nil
}

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

var (
	_ ports.EventRepository = (*fakeRepository)(nil)
	_ ports.EventPublisher  = (*fakePublisher)(nil)
	_ ports.Pinger          = (*fakePinger)(nil)
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
	repo        *fakeRepository
	publisher   *fakePublisher
	mongoPinger *fakePinger
	kafkaPinger *fakePinger
	handler     *appHTTP.Handler

	hasToken       bool
	tokenStudentID string

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
		publisher:        &fakePublisher{},
		mongoPinger:      &fakePinger{},
		kafkaPinger:      &fakePinger{},
		exerciseAttempts: make(map[string]exerciseAttempt),
	}
	service := application.NewIngestEventService(w.repo, w.publisher, discardLogger())
	w.handler = appHTTP.NewHandler(service, w.mongoPinger, w.kafkaPinger)
	return w
}

// submit runs body through the handler exactly as the generated router would,
// using the world's current auth state to build the context.
func (w *world) submit(body *generated.TrackingEvent) {
	ctx := context.Background()
	if w.hasToken {
		ctx = appHTTP.WithStudentID(ctx, w.tokenStudentID)
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
