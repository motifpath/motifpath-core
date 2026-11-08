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

func setRequiredEnv(t *testing.T) {
	t.Helper()
	for name, value := range map[string]string{
		"DATABASE_URL":             "postgres://localhost/motifpath",
		"MONGO_URI":                "mongodb://localhost",
		"CLERK_SECRET_KEY":         "sk_test_x",
		"MEDIA_S3_BUCKET":          "bucket",
		"MEDIA_PUBLIC_BASE_URL":    "http://localhost:9000/bucket",
		"EMAIL_UNSUBSCRIBE_SECRET": "secret",
		"MOTIFPATH_ENV":            "local",
	} {
		t.Setenv(name, value)
	}
}

// Commercial evidence and operation keys are namespaced by the deployment,
// so the service must know which one it is before it serves anything.
func TestLoadConfig_Environment(t *testing.T) {
	t.Run("a known deployment is loaded", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("MOTIFPATH_ENV", "staging")

		cfg, err := loadConfig()

		require.NoError(t, err)
		assert.Equal(t, "staging", cfg.environment.String())
	})

	for name, value := range map[string]string{
		"missing": "",
		"unknown": "qa",
	} {
		t.Run("refused when "+name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("MOTIFPATH_ENV", value)

			_, err := loadConfig()

			require.ErrorContains(t, err, "MOTIFPATH_ENV")
		})
	}
}
