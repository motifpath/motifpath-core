//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/event-ingestion/internal/domain"
)

func setupMongoRepository(t *testing.T) *MongoEventRepository {
	t.Helper()
	ctx := context.Background()

	container, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testcontainers.TerminateContainer(container))
	})

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	client, err := mongo.Connect(options.Client().ApplyURI(connStr))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, client.Disconnect(context.Background()))
	})

	repo := NewMongoEventRepository(client.Database("motifpath_events_test"))
	require.NoError(t, repo.EnsureIndexes(ctx))
	return repo
}

func TestMongoEventRepository_Save_WritesCorrectFields(t *testing.T) {
	repo := setupMongoRepository(t)
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
			TeacherID:     "55555555-5555-5555-5555-555555555555",
		},
	}

	receivedAt, err := repo.Save(ctx, event)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().UTC(), receivedAt, 5*time.Second)

	var doc eventDocument
	err = repo.collection.FindOne(ctx, bson.D{{Key: "event_id", Value: event.EventID}}).Decode(&doc)
	require.NoError(t, err)

	assert.Equal(t, event.EventID, doc.EventID)
	assert.Equal(t, string(domain.EventTypeLessonStarted), doc.EventType)
	assert.Equal(t, event.StudentID, doc.StudentID)
	assert.Equal(t, event.SessionID, doc.SessionID)
	assert.True(t, event.OccurredAt.Equal(doc.OccurredAt), "occurred_at should round-trip exactly")
	assert.Equal(t, receivedAt.Truncate(time.Millisecond), doc.ReceivedAt.Truncate(time.Millisecond))

	require.NotNil(t, doc.ContentContext)
	assert.Equal(t, event.ContentContext.ContentNodeID, doc.ContentContext.ContentNodeID)
	assert.Equal(t, string(domain.ContentTypeVideo), doc.ContentContext.ContentType)
	assert.Equal(t, event.ContentContext.TeacherID, doc.ContentContext.TeacherID)
}

func TestMongoEventRepository_Save_IsIdempotentOnEventID(t *testing.T) {
	repo := setupMongoRepository(t)
	ctx := context.Background()

	event := domain.LessonStartedEvent{
		TrackingEventBase: domain.TrackingEventBase{
			EventID:    "66666666-6666-6666-6666-666666666666",
			EventType:  domain.EventTypeLessonStarted,
			StudentID:  "22222222-2222-2222-2222-222222222222",
			SessionID:  "33333333-3333-3333-3333-333333333333",
			OccurredAt: time.Now().UTC(),
		},
		ContentContext: domain.ContentContext{ContentNodeID: "44444444-4444-4444-4444-444444444444"},
	}

	_, err := repo.Save(ctx, event)
	require.NoError(t, err)

	_, err = repo.Save(ctx, event)
	require.NoError(t, err, "resubmitting the same event_id must not error")

	count, err := repo.collection.CountDocuments(ctx, bson.D{{Key: "event_id", Value: event.EventID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "exactly one document must exist for a duplicate event_id — Save must not overwrite")
}

func TestMongoEventRepository_EnsureIndexes_CreatesExpectedIndexes(t *testing.T) {
	repo := setupMongoRepository(t)
	ctx := context.Background()

	cursor, err := repo.collection.Indexes().List(ctx)
	require.NoError(t, err)
	var indexes []bson.M
	require.NoError(t, cursor.All(ctx, &indexes))

	names := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		names = append(names, idx["name"].(string)) //nolint:forcetypeassert // index name is always a string
	}

	assert.Contains(t, names, "event_id_1")
	assert.Contains(t, names, "student_id_1_occurred_at_-1")
	assert.Contains(t, names, "event_type_1_occurred_at_-1")
}

func TestMongoEventRepository_Ping(t *testing.T) {
	repo := setupMongoRepository(t)
	assert.NoError(t, repo.Ping(context.Background()))
}
