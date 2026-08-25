package ports

import (
	"context"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// EventRepository persists tracking events durably. Save must be idempotent on the
// event's EventID: a duplicate delivery (at-least-once Kafka semantics upstream of
// the client, or a client retry) must not error or create a second record.
type EventRepository interface {
	Save(ctx context.Context, event domain.TrackingEvent) error
}
