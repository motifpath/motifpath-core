package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// recordPublishFailure loads the current outbox entry for eventID, applies
// domain.NextOutboxState, and persists the result. Shared by the initial
// publish attempt (IngestEventService) and the retry sweep
// (RetrySweepService) so the attempts/backoff/dead-lettering rule is defined
// in exactly one place.
func recordPublishFailure(ctx context.Context, outbox ports.PublishOutboxRepository, logger *slog.Logger, eventID string, publishErr error) {
	entry, found, err := outbox.Get(ctx, eventID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to load outbox entry after publish failure", "error", err, "event_id", eventID)
		return
	}
	if !found {
		// The outbox entry itself was never created (see
		// IngestEventService.Ingest) -- nothing to update. Recoverable only
		// via a future POST /events retry with the same event_id, or manual
		// intervention.
		return
	}

	entry.Attempts++
	entry.LastError = publishErr.Error()
	entry.Status, entry.NextAttemptAt = domain.NextOutboxState(entry.Attempts, time.Now().UTC())

	if err := outbox.Update(ctx, entry); err != nil {
		logger.ErrorContext(ctx, "failed to update outbox entry after publish failure", "error", err, "event_id", eventID)
		return
	}

	if entry.Status == domain.OutboxStatusDead {
		logger.ErrorContext(ctx, "event permanently failed to publish",
			"event_id", eventID,
			"attempts", entry.Attempts,
			"last_error", publishErr.Error(),
		)
	}
}
