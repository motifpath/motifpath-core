//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

type fakeHandler struct {
	mu       sync.Mutex
	handled  []domain.TrackingEvent
	attempts int
	fail     bool
}

func (f *fakeHandler) Handle(_ context.Context, event domain.TrackingEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	if f.fail {
		return errors.New("simulated handler failure")
	}
	f.handled = append(f.handled, event)
	return nil
}

func (f *fakeHandler) snapshot() (handled int, attempts int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.handled), f.attempts
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func setupBroker(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := redpanda.Run(ctx, "redpandadata/redpanda:v24.2.7", redpanda.WithAutoCreateTopics())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testcontainers.TerminateContainer(container))
	})

	broker, err := container.KafkaSeedBroker(ctx)
	require.NoError(t, err)
	return broker
}

func publishRaw(t *testing.T, broker, key string, payload map[string]any) {
	t.Helper()
	value, err := json.Marshal(payload)
	require.NoError(t, err)

	writer := &kafkago.Writer{
		Addr:                   kafkago.TCP(broker),
		Topic:                  topic,
		AllowAutoTopicCreation: true,
	}
	defer func() { assert.NoError(t, writer.Close()) }()

	require.NoError(t, writer.WriteMessages(context.Background(), kafkago.Message{
		Key:   []byte(key),
		Value: value,
	}))
}

func TestKafkaEventConsumer_Run_DecodesAndDispatchesToHandler(t *testing.T) {
	broker := setupBroker(t)
	handler := &fakeHandler{}
	consumer := NewKafkaEventConsumer([]string{broker}, handler, testLogger())
	t.Cleanup(func() { assert.NoError(t, consumer.Close()) })

	publishRaw(t, broker, "student-1", map[string]any{
		"event_type":      "lesson.started",
		"student_id":      "student-1",
		"content_context": map[string]any{"content_node_id": "node-1"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go func() { _ = consumer.Run(ctx) }()

	require.Eventually(t, func() bool {
		handled, _ := handler.snapshot()
		return handled == 1
	}, 15*time.Second, 200*time.Millisecond)

	handler.mu.Lock()
	defer handler.mu.Unlock()
	assert.Equal(t, domain.EventTypeLessonStarted, handler.handled[0].EventType)
	assert.Equal(t, "student-1", handler.handled[0].StudentID)
	assert.Equal(t, "node-1", handler.handled[0].ContentNodeID)
}

func TestKafkaEventConsumer_Run_DoesNotCommitOnHandlerFailure(t *testing.T) {
	broker := setupBroker(t)
	publishRaw(t, broker, "student-2", map[string]any{
		"event_type":      "lesson.completed",
		"student_id":      "student-2",
		"content_context": map[string]any{"content_node_id": "node-2"},
	})

	failingHandler := &fakeHandler{fail: true}
	firstConsumer := NewKafkaEventConsumer([]string{broker}, failingHandler, testLogger())

	firstCtx, firstCancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() { _ = firstConsumer.Run(firstCtx) }()

	require.Eventually(t, func() bool {
		_, attempts := failingHandler.snapshot()
		return attempts >= 1
	}, 8*time.Second, 200*time.Millisecond, "handler must be invoked at least once before it is torn down")

	firstCancel()
	require.NoError(t, firstConsumer.Close())

	// A fresh consumer joining the same, stable aggregation-worker group ID
	// must see the message again: the first consumer's handler failure left
	// the offset uncommitted.
	succeedingHandler := &fakeHandler{}
	secondConsumer := NewKafkaEventConsumer([]string{broker}, succeedingHandler, testLogger())
	t.Cleanup(func() { assert.NoError(t, secondConsumer.Close()) })

	secondCtx, secondCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer secondCancel()
	go func() { _ = secondConsumer.Run(secondCtx) }()

	require.Eventually(t, func() bool {
		handled, _ := succeedingHandler.snapshot()
		return handled == 1
	}, 15*time.Second, 200*time.Millisecond, "the uncommitted message must be redelivered to the next consumer in the group")

	assert.Equal(t, "student-2", succeedingHandler.handled[0].StudentID)
}
