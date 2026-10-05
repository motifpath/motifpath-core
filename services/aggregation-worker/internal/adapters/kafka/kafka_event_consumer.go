package kafka

import (
	"context"
	"errors"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/motifpath/aggregation-worker/internal/domain"
	"github.com/motifpath/aggregation-worker/internal/ports"
)

const (
	topic = "motifpath.events"

	// firstRetryPause and maxRetryPause bound the wait between attempts at a
	// message the handler failed on.
	firstRetryPause = 500 * time.Millisecond
	maxRetryPause   = 30 * time.Second

	// groupID must stay stable across deployments per ADR-006 — renaming it
	// loses committed offsets and triggers a full-topic replay.
	groupID = "aggregation-worker"
)

var errNoBrokersConfigured = errors.New("kafka: no brokers configured")

// KafkaEventConsumer subscribes to motifpath.events under the aggregation-worker
// consumer group (ADR-006) and dispatches each message to an EventHandler,
// committing its offset only after the handler succeeds. A handler failure is
// retried in place, and a crash leaves the message uncommitted for the next
// consumer — the handler must be idempotent under this at-least-once contract
// (ADR-011 satisfies this via NextStatus).
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
// retried forever on a poison message. A message the handler fails to process
// is retried, with a growing pause, before the next one is fetched: the reader
// never fetches a message twice, and committing a later one would move the
// group's offset past it, losing it for good. Retrying in place also keeps each
// student's events in order. The cost is that a handler failing for good stalls
// its partition, which the error log shows on every attempt.
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

		if !c.handleUntilDone(ctx, wire) {
			return nil
		}

		c.commit(ctx, msg)
	}
}

// handleUntilDone retries the handler until it succeeds, reporting false when
// ctx is cancelled first.
func (c *KafkaEventConsumer) handleUntilDone(ctx context.Context, wire wireEvent) bool {
	pause := firstRetryPause
	for attempt := 1; ; attempt++ {
		err := c.handler.Handle(ctx, toDomainEvent(wire))
		if err == nil {
			return true
		}
		c.logger.ErrorContext(ctx, "failed to process tracking event, retrying",
			"error", err,
			"attempt", attempt,
			"event_type", wire.EventType,
			"student_id", wire.StudentID,
		)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(pause):
		}
		pause = min(2*pause, maxRetryPause)
	}
}

func (c *KafkaEventConsumer) commit(ctx context.Context, msg kafkago.Message) {
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		c.logger.ErrorContext(ctx, "failed to commit message offset", "error", err)
	}
}

func toDomainEvent(w wireEvent) domain.TrackingEvent {
	event := domain.TrackingEvent{
		EventType:         domain.EventType(w.EventType),
		EventID:           w.EventID,
		StudentID:         w.StudentID,
		OccurredAt:        w.OccurredAt,
		PracticeSessionID: w.PracticeSessionID,
	}
	if w.ContentContext != nil {
		event.ContentNodeID = w.ContentContext.ContentNodeID
	}
	switch event.EventType {
	case domain.EventTypePracticeItemAnswered:
		if w.Response != nil {
			event.PracticeAnswer = toPracticeAnswer(w)
		}
	case domain.EventTypePracticeSessionStarted:
		event.SessionStart = toSessionStart(w)
	case domain.EventTypePracticeSessionEnded:
		event.SessionEnd = &domain.PracticeSessionEnd{
			EventID:           w.EventID,
			StudentID:         w.StudentID,
			PracticeSessionID: w.PracticeSessionID,
			OccurredAt:        w.OccurredAt,
			LeftEarly:         w.LeftEarly,
			AnsweredCount:     w.AnsweredCount,
		}
	case domain.EventTypeLessonStarted, domain.EventTypeLessonResumed, domain.EventTypeLessonCompleted:
		// A lesson event carries only its content node, set above.
	}
	return event
}

func toSessionStart(w wireEvent) *domain.PracticeSessionStart {
	items := make([]domain.PlannedPracticeItem, len(w.PlannedItems))
	for i, p := range w.PlannedItems {
		items[i] = domain.PlannedPracticeItem{ItemKey: p.ItemKey, Reason: p.Reason}
	}
	return &domain.PracticeSessionStart{
		EventID:           w.EventID,
		StudentID:         w.StudentID,
		PracticeSessionID: w.PracticeSessionID,
		OccurredAt:        w.OccurredAt,
		InstrumentID:      w.InstrumentID,
		Minutes:           w.Minutes,
		PlannedItems:      items,
	}
}

func toPracticeAnswer(w wireEvent) *domain.PracticeAnswer {
	r := w.Response
	var trigger *domain.TriggerContext
	if tc := w.TriggerContext; tc != nil {
		trigger = &domain.TriggerContext{Source: tc.Source, ContentNodeID: tc.ContentNodeID, ChallengeID: tc.ChallengeID}
	}
	return &domain.PracticeAnswer{
		EventID:           w.EventID,
		StudentID:         w.StudentID,
		OccurredAt:        w.OccurredAt,
		PracticeSessionID: w.PracticeSessionID,
		TriggerContext:    trigger,
		ItemKey:           w.ItemKey,
		Response: domain.PracticeResponse{
			Type:             domain.PracticeResponseType(r.ResponseType),
			NoteName:         r.NoteName,
			String:           r.String,
			Fret:             r.Fret,
			OptionIDs:        r.OptionIDs,
			LatencyMs:        r.LatencyMs,
			AudioMs:          r.AudioMs,
			Rating:           domain.SelfRating(r.Rating),
			TempoBPM:         r.TempoBPM,
			ChangesPerMinute: r.ChangesPerMinute,
		},
		TapMs: w.TapMs,
	}
}
