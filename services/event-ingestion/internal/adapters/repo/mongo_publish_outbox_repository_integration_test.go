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
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

func setupOutboxRepository(t *testing.T) *MongoPublishOutboxRepository {
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

	repo := NewMongoPublishOutboxRepository(client.Database("motifpath_events_test"))
	require.NoError(t, repo.EnsureIndexes(ctx))
	return repo
}

func TestMongoPublishOutboxRepository_Create_ThenGet_RoundTrips(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, "event-1"))

	entry, found, err := repo.Get(ctx, "event-1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "event-1", entry.EventID)
	assert.Equal(t, domain.OutboxStatusPending, entry.Status)
	assert.Equal(t, 0, entry.Attempts)
}

func TestMongoPublishOutboxRepository_Get_NotFound(t *testing.T) {
	repo := setupOutboxRepository(t)

	_, found, err := repo.Get(context.Background(), "does-not-exist")

	require.NoError(t, err)
	assert.False(t, found)
}

func TestMongoPublishOutboxRepository_Create_IsUniquePerEventID(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, "event-1"))
	err := repo.Create(ctx, "event-1")

	require.Error(t, err, "creating a second entry for the same event_id must fail on the _id uniqueness constraint")
}

func TestMongoPublishOutboxRepository_MarkPublished(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, "event-1"))
	require.NoError(t, repo.MarkPublished(ctx, "event-1"))

	entry, found, err := repo.Get(ctx, "event-1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, domain.OutboxStatusPublished, entry.Status)
}

func TestMongoPublishOutboxRepository_Update_PersistsAttemptsStatusAndBackoff(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, "event-1"))

	nextAttemptAt := time.Now().Add(30 * time.Second).UTC().Truncate(time.Millisecond)
	err := repo.Update(ctx, ports.OutboxEntry{
		EventID:       "event-1",
		Status:        domain.OutboxStatusPending,
		Attempts:      1,
		LastError:     "kafka unavailable",
		NextAttemptAt: nextAttemptAt,
	})
	require.NoError(t, err)

	entry, found, err := repo.Get(ctx, "event-1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 1, entry.Attempts)
	assert.Equal(t, "kafka unavailable", entry.LastError)
	assert.True(t, nextAttemptAt.Equal(entry.NextAttemptAt))
}

func TestMongoPublishOutboxRepository_MarkResolvedManually(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, "event-1"))
	require.NoError(t, repo.MarkResolvedManually(ctx, "event-1", "verified delivered out of band"))

	entry, found, err := repo.Get(ctx, "event-1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, domain.OutboxStatusResolvedManually, entry.Status)
}

func TestMongoPublishOutboxRepository_ListDueForRetry_OnlyReturnsPendingAndDue(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Due: pending, next_attempt_at in the past.
	require.NoError(t, repo.Create(ctx, "due-event"))
	require.NoError(t, repo.Update(ctx, ports.OutboxEntry{EventID: "due-event", Status: domain.OutboxStatusPending, NextAttemptAt: now.Add(-time.Minute)}))

	// Not due yet: pending, next_attempt_at in the future.
	require.NoError(t, repo.Create(ctx, "future-event"))
	require.NoError(t, repo.Update(ctx, ports.OutboxEntry{EventID: "future-event", Status: domain.OutboxStatusPending, NextAttemptAt: now.Add(time.Hour)}))

	// Not pending: already published, must be excluded regardless of timestamp.
	require.NoError(t, repo.Create(ctx, "published-event"))
	require.NoError(t, repo.MarkPublished(ctx, "published-event"))

	// Not pending: dead, must be excluded.
	require.NoError(t, repo.Create(ctx, "dead-event"))
	require.NoError(t, repo.Update(ctx, ports.OutboxEntry{EventID: "dead-event", Status: domain.OutboxStatusDead}))

	due, err := repo.ListDueForRetry(ctx, now)
	require.NoError(t, err)

	ids := make([]string, 0, len(due))
	for _, entry := range due {
		ids = append(ids, entry.EventID)
	}
	assert.ElementsMatch(t, []string{"due-event"}, ids)
}

func TestMongoPublishOutboxRepository_EnsureIndexes_CreatesExpectedIndex(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()

	cursor, err := repo.collection.Indexes().List(ctx)
	require.NoError(t, err)
	var indexes []map[string]any
	require.NoError(t, cursor.All(ctx, &indexes))

	names := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		names = append(names, idx["name"].(string)) //nolint:forcetypeassert // index name is always a string
	}
	assert.Contains(t, names, "status_1_next_attempt_at_1")
}
