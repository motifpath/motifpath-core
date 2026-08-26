package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// RetrySweepService periodically retries publish_outbox entries that a
// prior publish attempt failed to deliver, per ADR-012. It exists
// specifically for what kafka-go's own in-process retry cannot cover: an
// outage outlasting that budget, or the service process restarting
// mid-retry. The fast path (an event's first publish attempt) lives in
// IngestEventService and is unaffected by this service.
type RetrySweepService struct {
	outbox    ports.PublishOutboxRepository
	events    ports.EventRepository
	publisher ports.EventPublisher
	logger    *slog.Logger
}

func NewRetrySweepService(outbox ports.PublishOutboxRepository, events ports.EventRepository, publisher ports.EventPublisher, logger *slog.Logger) *RetrySweepService {
	return &RetrySweepService{outbox: outbox, events: events, publisher: publisher, logger: logger}
}

// Run blocks, sweeping every domain.RetryInterval until ctx is cancelled.
func (s *RetrySweepService) Run(ctx context.Context) {
	ticker := time.NewTicker(domain.RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SweepOnce(ctx)
		}
	}
}

// SweepOnce processes every entry currently due for retry. Exported
// separately from Run so tests can trigger a sweep synchronously instead of
// waiting on a real ticker.
func (s *RetrySweepService) SweepOnce(ctx context.Context) {
	entries, err := s.outbox.ListDueForRetry(ctx, time.Now().UTC())
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list publish_outbox entries due for retry", "error", err)
		return
	}

	for _, entry := range entries {
		s.retryEntry(ctx, entry.EventID)
	}
}

func (s *RetrySweepService) retryEntry(ctx context.Context, eventID string) {
	event, err := s.events.FindByEventID(ctx, eventID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load event for outbox retry", "error", err, "event_id", eventID)
		return
	}

	if err := s.publisher.Publish(ctx, event); err != nil {
		s.logger.ErrorContext(ctx, "retry sweep failed to publish tracking event", "error", err, "event_id", eventID)
		recordPublishFailure(ctx, s.outbox, s.logger, eventID, err)
		return
	}

	if err := s.outbox.MarkPublished(ctx, eventID); err != nil {
		s.logger.ErrorContext(ctx, "failed to mark outbox entry published after retry", "error", err, "event_id", eventID)
	}
}
