package ports

import (
	"context"
	"time"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// OutboxEntry is the retry-tracking record for a single tracking event's
// Kafka delivery, per ADR-012. EventID is the collection's document ID --
// there is at most one entry per event.
type OutboxEntry struct {
	EventID       string
	Status        domain.OutboxStatus
	Attempts      int
	LastError     string
	NextAttemptAt time.Time
	UpdatedAt     time.Time
}

// PublishOutboxRepository persists publish_outbox entries. It holds no
// business logic of its own -- callers decide the next status and retry
// time (see domain.NextOutboxState) and this repository just writes what
// it's given.
type PublishOutboxRepository interface {
	// Create inserts a new pending entry with zero attempts. Called only
	// alongside a fresh events insert, never for a duplicate.
	Create(ctx context.Context, eventID string) error

	// Get returns the entry for eventID, or found=false if none exists.
	Get(ctx context.Context, eventID string) (entry OutboxEntry, found bool, err error)

	// MarkPublished sets status to published.
	MarkPublished(ctx context.Context, eventID string) error

	// Update persists entry's Attempts, Status, LastError, and NextAttemptAt
	// verbatim -- the caller (application layer) is responsible for having
	// already computed the correct next state.
	Update(ctx context.Context, entry OutboxEntry) error

	// MarkResolvedManually sets status to resolved_manually and stores an
	// optional operator-supplied reason as part of LastError-adjacent audit
	// context.
	MarkResolvedManually(ctx context.Context, eventID string, reason string) error

	// ListDueForRetry returns pending entries whose NextAttemptAt has
	// passed, for the retry sweep to process.
	ListDueForRetry(ctx context.Context, now time.Time) ([]OutboxEntry, error)
}
