package kafka

import (
	"context"
	"encoding/json"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/motifpath/event-ingestion/internal/domain"
)

const topic = "motifpath.events"

// KafkaEventPublisher publishes tracking events to the motifpath.events topic,
// keyed by student_id so kafka-go's hash balancer routes all of a student's events
// to the same partition — required for the ordering guarantee in ADR-006.
type KafkaEventPublisher struct {
	writer *kafkago.Writer
}

func NewKafkaEventPublisher(brokers []string) *KafkaEventPublisher {
	return &KafkaEventPublisher{
		writer: &kafkago.Writer{
			Addr:     kafkago.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafkago.Hash{},
		},
	}
}

func (p *KafkaEventPublisher) Close() error {
	return p.writer.Close()
}

func (p *KafkaEventPublisher) Publish(ctx context.Context, event domain.TrackingEvent) error {
	payload, err := json.Marshal(toWireEvent(event))
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(event.Base().StudentID),
		Value: payload,
	})
}
