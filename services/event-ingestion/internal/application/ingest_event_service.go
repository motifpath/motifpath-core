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
	publisher ports.EventPublisher
	logger    *slog.Logger
}

func NewIngestEventService(repo ports.EventRepository, publisher ports.EventPublisher, logger *slog.Logger) *IngestEventService {
	return &IngestEventService{repo: repo, publisher: publisher, logger: logger}
}

// Ingest writes event durably via the repository, then publishes it to Kafka without
// blocking on delivery. A publish failure is logged but never fails the request: per
// the OpenAPI spec, the 202 response confirms durable receipt only — Kafka delivery
// is asynchronous and is not reflected in the response status. The returned time is
// the repository's own write timestamp, echoed to the caller in the 202 body.
func (s *IngestEventService) Ingest(ctx context.Context, event domain.TrackingEvent) (time.Time, error) {
	receivedAt, err := s.repo.Save(ctx, event)
	if err != nil {
		return time.Time{}, err
	}

	// Detached from ctx: the request context may be cancelled the moment the HTTP
	// handler returns, before this publish would otherwise get a chance to run.
	go s.publishAsync(context.WithoutCancel(ctx), event)

	return receivedAt, nil
}

func (s *IngestEventService) publishAsync(ctx context.Context, event domain.TrackingEvent) {
	if err := s.publisher.Publish(ctx, event); err != nil {
		base := event.Base()
		s.logger.ErrorContext(ctx, "failed to publish tracking event",
			"error", err,
			"event_id", base.EventID,
			"event_type", base.EventType,
		)
	}
}
