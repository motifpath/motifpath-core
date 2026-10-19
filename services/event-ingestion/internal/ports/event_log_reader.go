package ports

import (
	"context"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// EventLogReader reads stored tracking events as they were received.
type EventLogReader interface {
	// List returns the page q asks for, ordered by received_at then event_id,
	// both descending, so events received at the same instant keep one
	// order across pages, and the count of every event q matches.
	List(ctx context.Context, q domain.EventLogQuery) (events []domain.LoggedEvent, total int, err error)
}
