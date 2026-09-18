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
	seedLanguages(t, client)
	return client
}

// seedLanguages inserts the en/pt_BR/any system rows the Atlas migration
// seeds in production (see ADR-024) — auto-migrate creates only the DDL, not
// this data, so every test needing a resolvable locale needs it seeded here
// once per scratch database.
func seedLanguages(t *testing.T, client *ent.Client) {
	t.Helper()
	ctx := context.Background()
	seeds := []struct{ code, name string }{
		{"en", "English"},
		{"pt_BR", "Portuguese (Brazil)"},
		{"any", "Language-agnostic"},
	}
	for _, s := range seeds {
		require.NoError(t, client.Language.Create().SetCode(s.code).SetName(s.name).Exec(ctx))
	}
}
