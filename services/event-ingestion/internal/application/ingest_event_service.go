package application

import (
	"context"
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
	logger    *slog.Logger
}

func NewIngestEventService(repo ports.EventRepository, outbox ports.PublishOutboxRepository, publisher ports.EventPublisher, logger *slog.Logger) *IngestEventService {
	return &IngestEventService{repo: repo, outbox: outbox, publisher: publisher, logger: logger}
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
func (s *IngestEventService) Ingest(ctx context.Context, event domain.TrackingEvent) (time.Time, error) {
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
