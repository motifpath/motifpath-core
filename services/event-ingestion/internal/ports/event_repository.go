package ports

import (
	"context"
	"time"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// EventRepository persists tracking events durably. Save must be idempotent on the
// event's EventID: a duplicate delivery (at-least-once Kafka semantics upstream of
// the client, or a client retry) must not error or create a second record.
//
// Save returns the server-side timestamp at which the event was durably written —
// per the OpenAPI spec, this is echoed to the caller in the 202 response body, so
// it reflects the actual write moment rather than a value computed earlier by the
// application layer. It also reports alreadyExisted, so the application layer can
// tell a fresh write apart from a duplicate without a second query -- per ADR-012,
// this determines whether a new publish_outbox entry should be created.
type EventRepository interface {
	Save(ctx context.Context, event domain.TrackingEvent) (receivedAt time.Time, alreadyExisted bool, err error)

	// FindByEventID reconstructs the stored event for eventID, for the retry
	// sweep and admin endpoints to republish -- publish_outbox entries carry
	// only bookkeeping fields, not the event payload itself.
	FindByEventID(ctx context.Context, eventID string) (domain.TrackingEvent, error)
}
