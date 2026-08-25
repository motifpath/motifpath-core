//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"

	"github.com/motifpath/event-ingestion/internal/domain"
)

func setupKafkaPublisher(t *testing.T) (*KafkaEventPublisher, string) {
	t.Helper()
	ctx := context.Background()

	container, err := redpanda.Run(ctx, "redpandadata/redpanda:v24.2.7", redpanda.WithAutoCreateTopics())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testcontainers.TerminateContainer(container))
	})

	broker, err := container.KafkaSeedBroker(ctx)
	require.NoError(t, err)

	publisher := NewKafkaEventPublisher([]string{broker})
	t.Cleanup(func() {
		assert.NoError(t, publisher.Close())
	})

	return publisher, broker
}

// newTestReader reads from the beginning of the topic under a fresh, uniquely-named
// consumer group, so it reliably sees messages regardless of which partition the
// Hash balancer routed them to and regardless of publish/subscribe ordering.
func newTestReader(t *testing.T, broker, groupID string) *kafkago.Reader {
	t.Helper()
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     []string{broker},
		Topic:       topic,
		GroupID:     groupID,
		StartOffset: kafkago.FirstOffset,
		MaxWait:     2 * time.Second,
	})
	t.Cleanup(func() {
		assert.NoError(t, reader.Close())
	})
	return reader
}

func TestKafkaEventPublisher_Publish_ProducesToCorrectTopicAndPartitionKey(t *testing.T) {
	publisher, broker := setupKafkaPublisher(t)
	ctx := context.Background()

	event := domain.LessonStartedEvent{
		TrackingEventBase: domain.TrackingEventBase{
			EventID:    "11111111-1111-1111-1111-111111111111",
			EventType:  domain.EventTypeLessonStarted,
			StudentID:  "22222222-2222-2222-2222-222222222222",
			SessionID:  "33333333-3333-3333-3333-333333333333",
			OccurredAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
		},
		ContentContext: domain.ContentContext{
			ContentNodeID: "44444444-4444-4444-4444-444444444444",
			ContentType:   domain.ContentTypeVideo,
		},
	}

	require.NoError(t, publisher.Publish(ctx, event))

	reader := newTestReader(t, broker, "test-single-message")
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	msg, err := reader.ReadMessage(readCtx)
	require.NoError(t, err)

	assert.Equal(t, topic, msg.Topic)
	assert.Equal(t, event.StudentID, string(msg.Key), "partition key must be student_id")

	var wire wireEvent
	require.NoError(t, json.Unmarshal(msg.Value, &wire))
	assert.Equal(t, event.EventID, wire.EventID)
	assert.Equal(t, string(domain.EventTypeLessonStarted), wire.EventType)
	assert.Equal(t, event.StudentID, wire.StudentID)
	require.NotNil(t, wire.ContentContext)
	assert.Equal(t, event.ContentContext.ContentNodeID, wire.ContentContext.ContentNodeID)
}

func TestKafkaEventPublisher_Publish_SameStudentGoesToSamePartition(t *testing.T) {
	publisher, broker := setupKafkaPublisher(t)
	ctx := context.Background()

	studentID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	makeEvent := func(eventID string) domain.LessonStartedEvent {
		return domain.LessonStartedEvent{
			TrackingEventBase: domain.TrackingEventBase{
				EventID:    eventID,
				EventType:  domain.EventTypeLessonStarted,
				StudentID:  studentID,
				SessionID:  "33333333-3333-3333-3333-333333333333",
				OccurredAt: time.Now().UTC(),
			},
			ContentContext: domain.ContentContext{ContentNodeID: "44444444-4444-4444-4444-444444444444"},
		}
	}

	require.NoError(t, publisher.Publish(ctx, makeEvent("event-1")))
	require.NoError(t, publisher.Publish(ctx, makeEvent("event-2")))

	reader := newTestReader(t, broker, "test-same-partition")
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	msg1, err := reader.ReadMessage(readCtx)
	require.NoError(t, err)
	msg2, err := reader.ReadMessage(readCtx)
	require.NoError(t, err)

	assert.Equal(t, msg1.Partition, msg2.Partition, "same student_id key must route to the same partition")
}

func TestKafkaEventPublisher_Ping(t *testing.T) {
	publisher, _ := setupKafkaPublisher(t)
	assert.NoError(t, publisher.Ping(context.Background()))
}
