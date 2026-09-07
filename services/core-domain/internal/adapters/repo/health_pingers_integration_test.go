//go:build integration

package repo

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgresPinger_Ping exercises the readiness probe's learning-graph
// check against the package's shared Postgres container: a live pool pings
// clean, and a closed pool reports the failure the probe surfaces as 503.
func TestPostgresPinger_Ping(t *testing.T) {
	db, err := sql.Open("postgres", newPostgresDSN(t))
	require.NoError(t, err)

	pinger := NewPostgresPinger(db)
	assert.NoError(t, pinger.Ping(context.Background()), "ping a live pool")

	require.NoError(t, db.Close())
	assert.Error(t, pinger.Ping(context.Background()), "ping a closed pool")
}

// TestMongoCompletionStateReader_Ping exercises the readiness probe's
// completion-state check against the package's shared MongoDB container.
func TestMongoCompletionStateReader_Ping(t *testing.T) {
	reader := NewMongoCompletionStateReader(mongoDatabase(t))
	assert.NoError(t, reader.Ping(context.Background()))
}
