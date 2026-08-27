//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
)

// setupPostgres starts a scratch Postgres container and returns an ent
// client with the schema applied via auto-migrate. Per ADR-010, auto-migrate
// (client.Schema.Create) is reserved for exactly this: a fresh testcontainers
// database per test run, where there's no migration history to preserve and
// no reviewable-SQL requirement — that requirement applies to the real
// versioned migrations in internal/adapters/repo/ent/migrate/migrations/,
// generated separately via `make migrate:diff` (cmd/entmigrate) and verified
// by TestMigrationsApplyCleanly below.
func setupPostgres(t *testing.T) *ent.Client {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("core_domain_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testcontainers.TerminateContainer(container))
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	client, err := ent.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, client.Close())
	})

	// The postgres testcontainers module reports "ready" once its wait
	// strategy's log line matches, but Schema.Create's first act is `SHOW
	// server_version_num` — occasionally racing a connection reset in the
	// brief window right after that log line, before Postgres is actually
	// accepting connections. A few retries with backoff absorbs that race;
	// ent.Open itself is lazy (just wraps sql.Open) and never hits it.
	for attempt := 0; ; attempt++ {
		err = client.Schema.Create(ctx)
		if err == nil || attempt >= 4 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err)
	return client
}
