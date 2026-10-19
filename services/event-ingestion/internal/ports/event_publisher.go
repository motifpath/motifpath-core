package ports

import (
	"context"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// EventPublisher publishes a tracking event to the motifpath.events Kafka topic.
// Per ADR-006, publication is fire-and-forget from the caller's perspective: the
// application layer calls Publish without blocking the HTTP response on delivery
// confirmation, and Publish is responsible for logging its own failures.
type EventPublisher interface {
	Publish(ctx context.Context, event domain.TrackingEvent) error
}
