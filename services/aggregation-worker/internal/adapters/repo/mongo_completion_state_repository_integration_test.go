//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

func setupMongoRepository(t *testing.T) *MongoCompletionStateRepository {
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

	repository := NewMongoCompletionStateRepository(client.Database("motifpath_events_test"))
	require.NoError(t, repository.EnsureIndexes(ctx))
	return repository
}

func TestMongoCompletionStateRepository_GetStatus_NotFoundWhenNoDocument(t *testing.T) {
	repository := setupMongoRepository(t)

	_, found, err := repository.GetStatus(context.Background(), "student-1", "node-1")

	require.NoError(t, err)
	assert.False(t, found)
}

func TestMongoCompletionStateRepository_Upsert_ThenGetStatus_RoundTrips(t *testing.T) {
	repository := setupMongoRepository(t)
	ctx := context.Background()

	require.NoError(t, repository.Upsert(ctx, "student-1", "node-1", domain.CompletionStatusInProgress))

	status, found, err := repository.GetStatus(ctx, "student-1", "node-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, domain.CompletionStatusInProgress, status)
}

func TestMongoCompletionStateRepository_Upsert_OverwritesExistingStatus(t *testing.T) {
	repository := setupMongoRepository(t)
	ctx := context.Background()

	require.NoError(t, repository.Upsert(ctx, "student-1", "node-1", domain.CompletionStatusInProgress))
	require.NoError(t, repository.Upsert(ctx, "student-1", "node-1", domain.CompletionStatusCompleted))

	status, found, err := repository.GetStatus(ctx, "student-1", "node-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, domain.CompletionStatusCompleted, status)

	count, err := repository.collection.CountDocuments(ctx, bson.D{
		{Key: "student_id", Value: "student-1"},
		{Key: "content_node_id", Value: "node-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "exactly one document must exist per (student_id, content_node_id) pair")
}

func TestMongoCompletionStateRepository_EnsureIndexes_CreatesUniqueCompoundIndex(t *testing.T) {
	repository := setupMongoRepository(t)
	ctx := context.Background()

	cursor, err := repository.collection.Indexes().List(ctx)
	require.NoError(t, err)
	var indexes []bson.M
	require.NoError(t, cursor.All(ctx, &indexes))

	names := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		names = append(names, idx["name"].(string)) //nolint:forcetypeassert // index name is always a string
	}

	assert.Contains(t, names, "student_id_1_content_node_id_1")
}

func TestMongoCompletionStateRepository_Ping(t *testing.T) {
	repository := setupMongoRepository(t)
	assert.NoError(t, repository.Ping(context.Background()))
}
