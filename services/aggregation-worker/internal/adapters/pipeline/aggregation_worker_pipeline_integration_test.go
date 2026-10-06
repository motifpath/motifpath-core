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
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/adapters/kafka"
	"github.com/motifpath/aggregation-worker/internal/adapters/repo"
	"github.com/motifpath/aggregation-worker/internal/application"
	"github.com/motifpath/aggregation-worker/internal/domain"
)

const kafkaTopic = "motifpath.events"

func setupPipeline(t *testing.T) (broker string, db *mongo.Database) {
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

	db = mongoClient.Database("motifpath_events_test")

	redpandaContainer, err := redpanda.Run(ctx, "redpandadata/redpanda:v24.2.7", redpanda.WithAutoCreateTopics())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testcontainers.TerminateContainer(redpandaContainer))
	})
	broker, err = redpandaContainer.KafkaSeedBroker(ctx)
	require.NoError(t, err)

	return broker, db
}

// startWorker wires the worker the way cmd/main.go does and runs its consumer
// until the test ends.
func startWorker(t *testing.T, broker string, db *mongo.Database) *repo.MongoCompletionStateRepository {
	t.Helper()
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	completion := repo.NewMongoCompletionStateRepository(db)
	require.NoError(t, completion.EnsureIndexes(ctx))
	evidence := repo.NewMongoPracticeEvidenceRepository(db)
	require.NoError(t, evidence.EnsureIndexes(ctx))
	states := repo.NewMongoPracticeItemStateRepository(db)
	require.NoError(t, states.EnsureIndexes(ctx))
	history := repo.NewMongoPracticeItemHistoryRepository(db)
	require.NoError(t, history.EnsureIndexes(ctx))
	practice := application.NewPracticeEvidenceService(repo.NewMongoPracticeReferenceReader(db), evidence, states, history, logger)
	sessions := repo.NewMongoPracticeSessionRepository(db)
	require.NoError(t, sessions.EnsureIndexes(ctx))
	learning := repo.NewMongoLearningActivityRepository(db)
	require.NoError(t, learning.EnsureIndexes(ctx))
	tapChecks := repo.NewMongoTapCheckRepository(db)
	require.NoError(t, tapChecks.EnsureIndexes(ctx))

	consumer := kafka.NewKafkaEventConsumer([]string{broker},
		application.NewProcessEventService(completion, practice, application.NewActivityService(sessions, learning, tapChecks)), logger)
	t.Cleanup(func() { assert.NoError(t, consumer.Close()) })

	runCtx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	t.Cleanup(cancel)
	go func() { _ = consumer.Run(runCtx) }()
	return completion
}

func publishLessonEvent(t *testing.T, broker string, eventType domain.EventType, studentID, contentNodeID string) {
	t.Helper()
	publish(t, broker, studentID, map[string]any{
		"event_type":      string(eventType),
		"student_id":      studentID,
		"content_context": map[string]any{"content_node_id": contentNodeID},
	})
}

func publish(t *testing.T, broker, studentID string, payload map[string]any) {
	t.Helper()
	value, err := json.Marshal(payload)
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
	broker, db := setupPipeline(t)
	repository := startWorker(t, broker, db)

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

func TestAggregationWorkerPipeline_PracticeAnswer_BecomesEvidenceAndItemState(t *testing.T) {
	broker, db := setupPipeline(t)
	ctx := context.Background()
	const (
		studentID = "a11ce000-0000-4000-8000-000000000001"
		diagramID = "00000000-0000-4000-8000-0000000000f1"
		itemKey   = "play_along:" + diagramID
	)
	// The reference row core-domain keeps for the diagram.
	_, err := db.Collection("practice_reference").InsertOne(ctx, bson.D{
		{Key: "kind", Value: "diagram"}, {Key: "id", Value: diagramID}, {Key: "instrument_ids", Value: bson.A{}},
		{Key: "tempo_bpm", Value: 100}, {Key: "updated_at", Value: time.Now()}, {Key: "snapshot_version", Value: 1},
	})
	require.NoError(t, err)
	startWorker(t, broker, db)

	take := func(eventID string, rating string, bpm int) map[string]any {
		return map[string]any{
			"event_id": eventID, "event_type": "practice.item_answered", "student_id": studentID,
			"session_id": "33333333-3333-4333-8333-333333333333", "occurred_at": "2026-10-05T09:00:00Z",
			"practice_session_id": "44444444-4444-4444-8444-444444444444", "item_key": itemKey,
			"response": map[string]any{"response_type": "self_rating", "rating": rating, "tempo_bpm": bpm},
		}
	}
	first := take("e0000000-0000-4000-8000-000000000001", "clean", 80)
	publish(t, broker, studentID, first)
	publish(t, broker, studentID, first) // redelivered
	publish(t, broker, studentID, take("e0000000-0000-4000-8000-000000000002", "almost", 85))

	states := repo.NewMongoPracticeItemStateRepository(db)
	require.Eventually(t, func() bool {
		fold, _, found, err := states.Get(ctx, studentID, itemKey)
		return err == nil && found && fold.Attempts == 2
	}, 20*time.Second, 200*time.Millisecond, "both takes must fold into the item's state")

	fold, _, _, err := states.Get(ctx, studentID, itemKey)
	require.NoError(t, err)
	assert.Equal(t, 1, fold.Box)
	assert.Equal(t, 80, *fold.BestCleanBPM)
	count, err := db.Collection("practice_evidence").CountDocuments(ctx, bson.D{{Key: "student_id", Value: studentID}})
	require.NoError(t, err)
	assert.EqualValues(t, 2, count, "the redelivered take must be stored once")
}

func TestAggregationWorkerPipeline_KeepsSessionsCompletionsAndDailySnapshots(t *testing.T) {
	broker, db := setupPipeline(t)
	ctx := context.Background()
	const (
		studentID = "a11ce000-0000-4000-8000-000000000001"
		sessionID = "44444444-4444-4444-8444-444444444444"
		diagramID = "00000000-0000-4000-8000-0000000000f1"
		itemKey   = "play_along:" + diagramID
	)
	_, err := db.Collection("practice_reference").InsertOne(ctx, bson.D{
		{Key: "kind", Value: "diagram"}, {Key: "id", Value: diagramID}, {Key: "instrument_ids", Value: bson.A{}},
		{Key: "tempo_bpm", Value: 100}, {Key: "updated_at", Value: time.Now()}, {Key: "snapshot_version", Value: 1},
	})
	require.NoError(t, err)
	startWorker(t, broker, db)

	envelope := func(eventID, eventType, at string) map[string]any {
		return map[string]any{
			"event_id": eventID, "event_type": eventType, "student_id": studentID,
			"session_id": "33333333-3333-4333-8333-333333333333", "occurred_at": at,
		}
	}
	started := envelope("e0000000-0000-4000-8000-000000000001", "practice.session_started", "2026-10-05T18:00:00Z")
	started["practice_session_id"] = sessionID
	started["minutes"] = 10
	started["planned_items"] = []map[string]any{{"item_key": itemKey, "reason": "new"}}
	answered := envelope("e0000000-0000-4000-8000-000000000002", "practice.item_answered", "2026-10-05T18:03:00Z")
	answered["practice_session_id"] = sessionID
	answered["item_key"] = itemKey
	answered["response"] = map[string]any{"response_type": "self_rating", "rating": "clean", "tempo_bpm": 80}
	ended := envelope("e0000000-0000-4000-8000-000000000003", "practice.session_ended", "2026-10-05T18:11:00Z")
	ended["practice_session_id"] = sessionID
	ended["answered_count"] = 1
	ended["left_early"] = false
	ended["felt_ratings"] = []any{}
	completed := envelope("e0000000-0000-4000-8000-000000000004", "lesson.completed", "2026-10-05T19:30:00Z")
	completed["content_context"] = map[string]any{"content_node_id": "c0000000-0000-4000-8000-000000000001"}
	for _, e := range []map[string]any{started, answered, ended, completed, completed} {
		publish(t, broker, studentID, e)
	}

	activity := db.Collection("learning_activity")
	require.Eventually(t, func() bool {
		n, err := activity.CountDocuments(ctx, bson.D{{Key: "student_id", Value: studentID}})
		return err == nil && n == 1
	}, 20*time.Second, 200*time.Millisecond, "the completion must be kept, once")

	session, found, err := repo.NewMongoPracticeSessionRepository(db).Get(ctx, studentID, sessionID)
	require.NoError(t, err)
	require.True(t, found)
	status, endedAt := session.StatusAt(time.Date(2026, 10, 5, 23, 0, 0, 0, time.UTC))
	assert.Equal(t, domain.PracticeSessionFinished, status)
	assert.Equal(t, time.Date(2026, 10, 5, 18, 11, 0, 0, time.UTC), *endedAt)

	snapshots, err := db.Collection("practice_item_history").CountDocuments(ctx, bson.D{
		{Key: "student_id", Value: studentID}, {Key: "item_key", Value: itemKey},
		{Key: "day", Value: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)},
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, snapshots)
}
