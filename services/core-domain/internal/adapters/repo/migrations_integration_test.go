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
	"time"

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

	// Same race setupPostgres documents: the postgres module reports ready
	// on a log line that can precede Postgres actually accepting
	// connections, and sql.Open is lazy, so the first Exec eats the reset.
	// Ping with backoff before executing any migration.
	for attempt := 0; ; attempt++ {
		err = db.PingContext(ctx)
		if err == nil || attempt >= 4 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err)

	applied := 0
	for _, file := range files {
		sqlBytes, err := os.ReadFile(file) //nolint:gosec // fixed test-local migration directory, not user input
		require.NoError(t, err)
		for _, stmt := range strings.Split(string(sqlBytes), ";") {
			stmt = stripSQLComments(stmt)
			if stmt == "" {
				continue
			}
			_, err := db.ExecContext(ctx, stmt)
			require.NoErrorf(t, err, "migration %s failed on statement: %s", file, stmt)
			applied++
		}
	}

	// Atlas writes a `-- description` line above every statement, and those
	// comments share a `;`-delimited chunk with the statement they describe.
	// Guard against a splitter regression that silently skips every
	// statement and leaves this test passing against an empty database.
	require.NotZero(t, applied, "expected at least one migration statement to be executed")
	assertColumnExists(t, ctx, db, "learning_path_items", "position")
	assertColumnExists(t, ctx, db, "learning_path_items", "section_label")
}

// stripSQLComments removes whole-line `--` comments from one `;`-delimited
// chunk of a migration file and trims the remainder. Returns "" when the
// chunk carries no executable SQL.
func stripSQLComments(chunk string) string {
	var lines []string
	for _, line := range strings.Split(chunk, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func assertColumnExists(t *testing.T, ctx context.Context, db *sql.DB, table, column string) {
	t.Helper()
	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
		table, column).Scan(&exists)
	require.NoError(t, err)
	assert.Truef(t, exists, "expected migrations to create %s.%s", table, column)
}
