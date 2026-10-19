package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// IngestEventService orchestrates durable storage and asynchronous publication of a
// single tracking event.
type IngestEventService struct {
	repo      ports.EventRepository
	outbox    ports.PublishOutboxRepository
	publisher ports.EventPublisher
	taps      ports.TapBaselineReader
	logger    *slog.Logger
}

func NewIngestEventService(repo ports.EventRepository, outbox ports.PublishOutboxRepository, publisher ports.EventPublisher, taps ports.TapBaselineReader, logger *slog.Logger) *IngestEventService {
	return &IngestEventService{repo: repo, outbox: outbox, publisher: publisher, taps: taps, logger: logger}
}

// Ingest writes event durably via the repository, then publishes it to Kafka without
// blocking on delivery. A publish failure is logged but never fails the request: per
// the OpenAPI spec, the 202 response confirms durable receipt only — Kafka delivery
// is asynchronous and is not reflected in the response status. The returned time is
// the repository's own write timestamp, echoed to the caller in the 202 body.
//
// Publishing is always attempted, whether or not this call turned out to be a
// duplicate (repo.Save's alreadyExisted) -- per ADR-012, every consumer of
// motifpath.events already has to tolerate duplicate delivery regardless, so
// suppressing a republish on retry has no correctness benefit and would only
// reintroduce the silent-loss risk this design closes. A fresh (non-duplicate)
// write additionally creates a publish_outbox entry, which is what makes a
// failed publish retryable later instead of being lost.
//
// callerUserID is the caller's resolved MotifPath user id (ADR-014). An event
// whose student_id is not that value is rejected with domain.ErrIdentityMismatch
// before any write -- the student_id carried into motifpath.events is always the
// caller's own identity, which is what every downstream consumer keys on.
//
// A practice.item_answered event is stamped with the student's tap time before it
// is stored, so the stored document and the published message carry the same value.
func (s *IngestEventService) Ingest(ctx context.Context, callerUserID string, event domain.TrackingEvent) (time.Time, error) {
	if event.Base().StudentID != callerUserID {
		return time.Time{}, domain.ErrIdentityMismatch
	}

	if answer, ok := event.(domain.PracticeItemAnsweredEvent); ok {
		stamped, err := s.stampTap(ctx, answer)
		if err != nil {
			return time.Time{}, err
		}
		event = stamped
	}

	receivedAt, alreadyExisted, err := s.repo.Save(ctx, event)
	if err != nil {
		return time.Time{}, err
	}

	eventID := event.Base().EventID

	if !alreadyExisted {
		if err := s.outbox.Create(ctx, eventID); err != nil {
			s.logger.ErrorContext(ctx, "failed to create publish outbox entry", "error", err, "event_id", eventID)
		}
	}

	// Detached from ctx: the request context may be cancelled the moment the HTTP
	// handler returns, before this publish would otherwise get a chance to run.
	go s.publishAsync(context.WithoutCancel(ctx), event)

	return receivedAt, nil
}

func (s *IngestEventService) publishAsync(ctx context.Context, event domain.TrackingEvent) {
	base := event.Base()

	if err := s.publisher.Publish(ctx, event); err != nil {
		s.logger.ErrorContext(ctx, "failed to publish tracking event",
			"error", err,
			"event_id", base.EventID,
			"event_type", base.EventType,
		)
		recordPublishFailure(ctx, s.outbox, s.logger, base.EventID, err)
		return
	}

	if err := s.outbox.MarkPublished(ctx, base.EventID); err != nil {
		s.logger.ErrorContext(ctx, "failed to mark outbox entry published", "error", err, "event_id", base.EventID)
	}
}

// stampTap sets the answer's TapMs from the student's latest tap check before it, on
// timed answers only, and clears any value already there: tap_ms is server-set. A
// failed lookup fails the request rather than storing the answer without its tap --
// the client retries with the same event_id. On such a retry the lookup runs again;
// it is keyed on occurred_at, so it finds the same tap check unless an older one
// arrived late in between.
func (s *IngestEventService) stampTap(ctx context.Context, answer domain.PracticeItemAnsweredEvent) (domain.PracticeItemAnsweredEvent, error) {
	answer.TapMs = nil
	if !answer.Response.IsTimed() {
		return answer, nil
	}

	tapMs, found, err := s.taps.LatestTap(ctx, answer.StudentID, answer.OccurredAt)
	if err != nil {
		return answer, fmt.Errorf("reading the student's latest tap check: %w", err)
	}
	if found {
		answer.TapMs = &tapMs
	}
	return answer, nil
}
