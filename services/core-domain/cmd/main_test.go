package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// An unreachable MongoDB must not hold the listener back: the start-up sync
// gives up at its own deadline instead of the driver's 30s server selection.
func TestSyncPracticeReference_GivesUpAtItsTimeout(t *testing.T) {
	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1"))
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Log(err)
		}
	})

	start := time.Now()
	syncPracticeReference(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), nil, client.Database("unreachable"), 200*time.Millisecond)

	assert.Less(t, time.Since(start), 5*time.Second)
}
