package kafka

import (
	"context"
	"errors"
	"log/slog"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/motifpath/aggregation-worker/internal/domain"
	"github.com/motifpath/aggregation-worker/internal/ports"
)

const (
	topic = "motifpath.events"

	// groupID must stay stable across deployments per ADR-006 — renaming it
	// loses committed offsets and triggers a full-topic replay.
	groupID = "aggregation-worker"
)

var errNoBrokersConfigured = errors.New("kafka: no brokers configured")

// KafkaEventConsumer subscribes to motifpath.events under the aggregation-worker
// consumer group (ADR-006) and dispatches each message to an EventHandler,
// committing its offset only after the handler succeeds. A crash or handler
// failure leaves the message uncommitted, so it is redelivered on the next
// poll or after a restart — the handler must be idempotent under this
// at-least-once contract (ADR-011 satisfies this via NextStatus).
type KafkaEventConsumer struct {
	reader  *kafkago.Reader
	handler ports.EventHandler
	logger  *slog.Logger
	brokers []string
}

func NewKafkaEventConsumer(brokers []string, handler ports.EventHandler, logger *slog.Logger) *KafkaEventConsumer {
	return &KafkaEventConsumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers: brokers,
			Topic:   topic,
			GroupID: groupID,
		}),
		handler: handler,
		logger:  logger,
		brokers: brokers,
	}
}

func (c *KafkaEventConsumer) Close() error {
	return c.reader.Close()
}

// Ping reports whether at least one configured broker is reachable.
func (c *KafkaEventConsumer) Ping(ctx context.Context) error {
	if len(c.brokers) == 0 {
		return errNoBrokersConfigured
	}
	conn, err := kafkago.DialContext(ctx, "tcp", c.brokers[0])
	if err != nil {
		return err
	}
	return conn.Close()
}

// Run consumes messages until ctx is cancelled or an unrecoverable read error
// occurs. A message that fails to decode is logged and committed rather than
// retried forever on a poison message; a message the handler fails to process
// is logged and left uncommitted so it is redelivered.
func (c *KafkaEventConsumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		wire, err := decodeWireEvent(msg.Value)
		if err != nil {
			c.logger.ErrorContext(ctx, "failed to decode tracking event, skipping", "error", err)
			c.commit(ctx, msg)
			continue
		}

		if err := c.handler.Handle(ctx, toDomainEvent(wire)); err != nil {
			c.logger.ErrorContext(ctx, "failed to process tracking event",
				"error", err,
				"event_type", wire.EventType,
				"student_id", wire.StudentID,
			)
			continue
		}

		c.commit(ctx, msg)
	}
}

func (c *KafkaEventConsumer) commit(ctx context.Context, msg kafkago.Message) {
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		c.logger.ErrorContext(ctx, "failed to commit message offset", "error", err)
	}
}

func toDomainEvent(w wireEvent) domain.TrackingEvent {
	event := domain.TrackingEvent{
		EventType: domain.EventType(w.EventType),
		StudentID: w.StudentID,
	}
	if w.ContentContext != nil {
		event.ContentNodeID = w.ContentContext.ContentNodeID
	}
	return event
}
