package kafka

import (
	"context"
	"encoding/json"
	"errors"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/motifpath/event-ingestion/internal/domain"
)

const topic = "motifpath.events"

var errNoBrokersConfigured = errors.New("kafka: no brokers configured")

// KafkaEventPublisher publishes tracking events to the motifpath.events topic,
// keyed by student_id so kafka-go's hash balancer routes all of a student's events
// to the same partition — required for the ordering guarantee in ADR-006.
type KafkaEventPublisher struct {
	writer  *kafkago.Writer
	brokers []string
}

func NewKafkaEventPublisher(brokers []string) *KafkaEventPublisher {
	return &KafkaEventPublisher{
		writer: &kafkago.Writer{
			Addr:     kafkago.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafkago.Hash{},
		},
		brokers: brokers,
	}
}

func (p *KafkaEventPublisher) Close() error {
	return p.writer.Close()
}

// Ping reports whether at least one configured broker is reachable, for the
// readiness probe. Dials directly rather than through the Writer, which has no
// built-in connectivity check.
func (p *KafkaEventPublisher) Ping(ctx context.Context) error {
	if len(p.brokers) == 0 {
		return errNoBrokersConfigured
	}
	conn, err := kafkago.DialContext(ctx, "tcp", p.brokers[0])
	if err != nil {
		return err
	}
	return conn.Close()
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
