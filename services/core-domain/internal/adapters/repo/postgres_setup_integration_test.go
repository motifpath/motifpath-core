//go:build integration

package repo

import (
	"context"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
)

// setupPostgres returns an ent client bound to a fresh database on the
// package's shared Postgres container, with the schema applied via
// auto-migrate.
//
// Per ADR-010, auto-migrate (client.Schema.Create) is reserved for exactly
// this: a scratch database per test, where there's no migration history to
// preserve and no reviewable-SQL requirement — that requirement applies to
// the real versioned migrations in internal/adapters/repo/ent/migrate/
// migrations/, generated separately via `make migrate:diff` (cmd/entmigrate)
// and verified by TestMigrationsApplyCleanly.
//
// The container-readiness retry loop that used to live here is now done once
// in TestMain, before any test runs.
func setupPostgres(t *testing.T) *ent.Client {
	t.Helper()

	client, err := ent.Open("postgres", newPostgresDSN(t))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, client.Close())
	})

	require.NoError(t, client.Schema.Create(context.Background()))
	return client
}
