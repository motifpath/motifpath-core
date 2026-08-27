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

	"github.com/motifpath/core-domain/internal/domain"
)

// TestMongoCompletionStateReader_ReadsAggregationWorkerShape verifies
// GetStatuses reflects a document written in exactly the shape ADR-011's
// Aggregation Worker writes — simulating that worker's output directly
// (rather than running the real worker) is sufficient here because this
// service treats the `aggregates` collection as read-only; the worker's own
// write-path correctness is covered by its own integration tests.
func TestMongoCompletionStateReader_ReadsAggregationWorkerShape(t *testing.T) {
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

	db := client.Database("motifpath_events_test")
	_, err = db.Collection("aggregates").InsertMany(ctx, []any{
		bson.D{{Key: "student_id", Value: "alice"}, {Key: "content_node_id", Value: "node-01"}, {Key: "status", Value: "completed"}, {Key: "updated_at", Value: time.Now().UTC()}},
		bson.D{{Key: "student_id", Value: "alice"}, {Key: "content_node_id", Value: "node-02"}, {Key: "status", Value: "in_progress"}, {Key: "updated_at", Value: time.Now().UTC()}},
		// Different student — must never leak into alice's result.
		bson.D{{Key: "student_id", Value: "bob"}, {Key: "content_node_id", Value: "node-01"}, {Key: "status", Value: "completed"}, {Key: "updated_at", Value: time.Now().UTC()}},
	})
	require.NoError(t, err)

	reader := NewMongoCompletionStateReader(db)

	statuses, err := reader.GetStatuses(ctx, "alice", []string{"node-01", "node-02", "node-03"})
	require.NoError(t, err)
	assert.Equal(t, map[string]domain.CompletionStatus{
		"node-01": domain.CompletionStatusCompleted,
		"node-02": domain.CompletionStatusInProgress,
	}, statuses)
}
