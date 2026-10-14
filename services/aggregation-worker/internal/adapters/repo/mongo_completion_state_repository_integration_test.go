//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

func setupMongoRepository(t *testing.T) *MongoCompletionStateRepository {
	t.Helper()
	repository := NewMongoCompletionStateRepository(mongoDatabase(t))
	require.NoError(t, repository.EnsureIndexes(context.Background()))
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

func TestMongoCompletionStateRepository_MarkStarted_KeepsTheEarliestStart(t *testing.T) {
	repository := setupMongoRepository(t)
	ctx := context.Background()
	first := time.Date(2026, 10, 5, 19, 0, 0, 0, time.UTC)

	require.NoError(t, repository.Upsert(ctx, "student-1", "node-1", domain.CompletionStatusInProgress))
	require.NoError(t, repository.MarkStarted(ctx, "student-1", "node-1", first.Add(time.Hour)))
	require.NoError(t, repository.MarkStarted(ctx, "student-1", "node-1", first))
	require.NoError(t, repository.MarkStarted(ctx, "student-1", "node-1", first.Add(2*time.Hour)))

	var doc bson.M
	require.NoError(t, repository.collection.FindOne(ctx, bson.D{
		{Key: "student_id", Value: "student-1"},
		{Key: "content_node_id", Value: "node-1"},
	}).Decode(&doc))
	assert.Equal(t, bson.NewDateTimeFromTime(first), doc["started_at"])
	assert.Equal(t, string(domain.CompletionStatusInProgress), doc["status"], "marking the start leaves the status as it is")
}
