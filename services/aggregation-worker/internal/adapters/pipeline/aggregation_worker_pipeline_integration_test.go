//go:build integration

// Package pipeline exercises the Aggregation Worker end to end — real Kafka,
// real MongoDB, real application service — matching ADR-011's Phase 4.0
// validation criteria: a lesson.completed event must reach the aggregates
// collection as status: completed, and redelivery of the same event must
// neither error nor change that outcome.
package pipeline

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/adapters/kafka"
	"github.com/motifpath/aggregation-worker/internal/adapters/repo"
	"github.com/motifpath/aggregation-worker/internal/application"
	"github.com/motifpath/aggregation-worker/internal/domain"
)

const kafkaTopic = "motifpath.events"

func setupPipeline(t *testing.T) (broker string, repository *repo.MongoCompletionStateRepository) {
	t.Helper()
	ctx := context.Background()

	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testcontainers.TerminateContainer(mongoContainer))
	})
	connStr, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(connStr))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mongoClient.Disconnect(context.Background()))
	})

	repository = repo.NewMongoCompletionStateRepository(mongoClient.Database("motifpath_events_test"))
	require.NoError(t, repository.EnsureIndexes(ctx))

	redpandaContainer, err := redpanda.Run(ctx, "redpandadata/redpanda:v24.2.7", redpanda.WithAutoCreateTopics())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testcontainers.TerminateContainer(redpandaContainer))
	})
	broker, err = redpandaContainer.KafkaSeedBroker(ctx)
	require.NoError(t, err)

	return broker, repository
}

func publishLessonEvent(t *testing.T, broker string, eventType domain.EventType, studentID, contentNodeID string) {
	t.Helper()
	value, err := json.Marshal(map[string]any{
		"event_type":      string(eventType),
		"student_id":      studentID,
		"content_context": map[string]any{"content_node_id": contentNodeID},
	})
	require.NoError(t, err)

	writer := &kafkago.Writer{
		Addr:                   kafkago.TCP(broker),
		Topic:                  kafkaTopic,
		AllowAutoTopicCreation: true,
	}
	defer func() { assert.NoError(t, writer.Close()) }()

	require.NoError(t, writer.WriteMessages(context.Background(), kafkago.Message{
		Key:   []byte(studentID),
		Value: value,
	}))
}

func TestAggregationWorkerPipeline_LessonCompleted_ReachesCompletedStatus(t *testing.T) {
	broker, repository := setupPipeline(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := application.NewProcessEventService(repository)
	consumer := kafka.NewKafkaEventConsumer([]string{broker}, service, logger)
	t.Cleanup(func() { assert.NoError(t, consumer.Close()) })

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	go func() { _ = consumer.Run(ctx) }()

	const studentID = "student-pipeline-1"
	const contentNodeID = "node-pipeline-1"

	publishLessonEvent(t, broker, domain.EventTypeLessonStarted, studentID, contentNodeID)
	require.Eventually(t, func() bool {
		status, found, err := repository.GetStatus(context.Background(), studentID, contentNodeID)
		return err == nil && found && status == domain.CompletionStatusInProgress
	}, 20*time.Second, 200*time.Millisecond, "lesson.started must reach in_progress")

	publishLessonEvent(t, broker, domain.EventTypeLessonCompleted, studentID, contentNodeID)
	require.Eventually(t, func() bool {
		status, found, err := repository.GetStatus(context.Background(), studentID, contentNodeID)
		return err == nil && found && status == domain.CompletionStatusCompleted
	}, 20*time.Second, 200*time.Millisecond, "lesson.completed must reach completed")

	// Duplicate delivery of the same event must not error and must not change
	// the outcome — ADR-011's idempotency guarantee.
	publishLessonEvent(t, broker, domain.EventTypeLessonCompleted, studentID, contentNodeID)

	// Prove the consumer loop kept running past the duplicate (rather than
	// having wedged on an unexpected error) by processing one more, distinct
	// event for a different node on the same student.
	const secondContentNodeID = "node-pipeline-2"
	publishLessonEvent(t, broker, domain.EventTypeLessonStarted, studentID, secondContentNodeID)
	require.Eventually(t, func() bool {
		status, found, err := repository.GetStatus(context.Background(), studentID, secondContentNodeID)
		return err == nil && found && status == domain.CompletionStatusInProgress
	}, 20*time.Second, 200*time.Millisecond, "consumer must keep processing after a duplicate delivery")

	status, found, err := repository.GetStatus(context.Background(), studentID, contentNodeID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, domain.CompletionStatusCompleted, status, "the duplicate lesson.completed must leave status unchanged")
}
