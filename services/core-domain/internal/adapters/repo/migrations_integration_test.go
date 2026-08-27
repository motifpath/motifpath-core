//go:build integration

package repo

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestMigrationsApplyCleanly runs every versioned migration file in
// internal/adapters/repo/ent/migrate/migrations/ against a fresh Postgres
// container, in filename (timestamp) order — the same mechanism
// `atlas migrate apply` uses at service startup (ADR-005), minus the
// advisory lock, which only matters under concurrent replica startup, not
// a single-connection test.
func TestMigrationsApplyCleanly(t *testing.T) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("core_domain_migrate_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testcontainers.TerminateContainer(container))
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	files, err := filepath.Glob("ent/migrate/migrations/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files, "expected at least one migration file")
	sort.Strings(files)

	for _, file := range files {
		sqlBytes, err := os.ReadFile(file) //nolint:gosec // fixed test-local migration directory, not user input
		require.NoError(t, err)
		statements := strings.Split(string(sqlBytes), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}
			_, err := db.ExecContext(ctx, stmt)
			require.NoErrorf(t, err, "migration %s failed on statement: %s", file, stmt)
		}
	}
}
