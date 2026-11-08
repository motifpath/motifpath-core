//go:build integration

package repo

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
)

// The commercial store depends on what only the real migrations create — the
// append-only trigger, the activation row and the 17 flags seeded OFF — so
// its tests run against migrated databases, not Schema.Create. Migrating once
// into a template and cloning it per test keeps that fast.
var (
	migratedTemplateOnce sync.Once
	migratedTemplateErr  error
)

const migratedTemplateName = "migrated_template"

func setupMigratedPostgres(t *testing.T) (*ent.Client, *sql.DB) {
	t.Helper()
	require.NotEmpty(t, sharedPostgresDSN, "TestMain did not start the shared postgres container")
	migratedTemplateOnce.Do(func() { migratedTemplateErr = buildMigratedTemplate(t) })
	require.NoError(t, migratedTemplateErr)

	name := fmt.Sprintf("test_%d", postgresDatabaseNo.Add(1))
	admin, err := sql.Open("postgres", sharedPostgresDSN)
	require.NoError(t, err)
	defer func() { require.NoError(t, admin.Close()) }()
	_, err = admin.ExecContext(context.Background(), "CREATE DATABASE "+name+" TEMPLATE "+migratedTemplateName)
	require.NoError(t, err)

	dsn := databaseDSN(t, name)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	client, err := ent.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, client.Close())
		assert.NoError(t, db.Close())
	})
	return client, db
}

func buildMigratedTemplate(t *testing.T) error {
	ctx := context.Background()
	admin, err := sql.Open("postgres", sharedPostgresDSN)
	if err != nil {
		return err
	}
	defer func() {
		if err := admin.Close(); err != nil {
			t.Log(err)
		}
	}()
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+migratedTemplateName); err != nil {
		return err
	}

	db, err := sql.Open("postgres", databaseDSN(t, migratedTemplateName))
	if err != nil {
		return err
	}
	for _, file := range migrationFiles(t) {
		if _, err := execMigrationFile(ctx, db, file); err != nil {
			if closeErr := db.Close(); closeErr != nil {
				t.Log(closeErr)
			}
			return err
		}
	}
	// A template cannot be cloned while anyone is connected to it.
	return db.Close()
}

func databaseDSN(t *testing.T, name string) string {
	t.Helper()
	parsed, err := url.Parse(sharedPostgresDSN)
	require.NoError(t, err)
	parsed.Path = "/" + name
	return parsed.String()
}
