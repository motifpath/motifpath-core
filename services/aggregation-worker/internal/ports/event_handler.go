package ports

import (
	"context"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// EventHandler processes one decoded tracking event. Implemented by
// application.ProcessEventService; depended on by adapters/kafka.KafkaEventConsumer
// so the consumer loop stays independent of the application layer's concrete type.
type EventHandler interface {
	Handle(ctx context.Context, event domain.TrackingEvent) error
}
