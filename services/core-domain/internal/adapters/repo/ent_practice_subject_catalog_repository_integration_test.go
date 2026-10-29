//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntPracticeSubjectCatalogRepository(t *testing.T) {
	ctx := context.Background()
	built := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	catalogOf := func(scope string, version int64, items ...domain.PracticeSubjectCatalogItem) domain.PracticeSubjectCatalog {
		if items == nil {
			items = []domain.PracticeSubjectCatalogItem{}
		}
		return domain.PracticeSubjectCatalog{Scope: scope, ContentVersion: version, FormatVersion: 1, BuiltAt: built, Items: items}
	}
	exercise := domain.PracticeSubjectCatalogItem{ItemKey: "exercise:e1", NodeIDs: []string{"n1", "n2"}, InHand: true, InHead: true, Rung: domain.PracticeRungKnow, Languages: []string{"pt_BR", "en"}}
	cell := domain.PracticeSubjectCatalogItem{ItemKey: "fretboard_cell:g:1:3", NodeIDs: []string{"n3"}, InHead: true, Ways: []domain.PracticeSubjectCatalogWay{
		{Way: "name_the_note", Rung: domain.PracticeRungKnow}, {Way: "find_the_note", Rung: domain.PracticeRungHear},
	}}

	t.Run("before any change the version is 0 and there are no catalogs", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t))

		got, version, err := catalogs.Catalogs(ctx, []string{"guitar-id", ""})

		require.NoError(t, err)
		assert.Empty(t, got)
		assert.Zero(t, version)
	})

	t.Run("each raise adds one to the version", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t))

		require.NoError(t, catalogs.BumpContentVersion(ctx))
		require.NoError(t, catalogs.BumpContentVersion(ctx))

		_, version, err := catalogs.Catalogs(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(2), version)
	})

	t.Run("catalogs stored for the current version are read back by scope", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t))
		require.NoError(t, catalogs.BumpContentVersion(ctx))
		guitar, none := catalogOf("guitar-id", 1, exercise, cell), catalogOf("", 1)

		stored, err := catalogs.Store(ctx, 1, []domain.PracticeSubjectCatalog{guitar, none})

		require.NoError(t, err)
		assert.True(t, stored)
		got, version, err := catalogs.Catalogs(ctx, []string{"guitar-id", "", "bass-id"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), version)
		assert.Equal(t, map[string]domain.PracticeSubjectCatalog{"guitar-id": guitar, "": none}, got, "a scope with no catalog is absent")
	})

	t.Run("storing replaces only the scopes stored", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t))
		_, err := catalogs.Store(ctx, 0, []domain.PracticeSubjectCatalog{catalogOf("guitar-id", 0, exercise), catalogOf("bass-id", 0)})
		require.NoError(t, err)
		require.NoError(t, catalogs.BumpContentVersion(ctx))
		newGuitar := catalogOf("guitar-id", 1, exercise, cell)

		_, err = catalogs.Store(ctx, 1, []domain.PracticeSubjectCatalog{newGuitar})
		require.NoError(t, err)

		got, _, err := catalogs.Catalogs(ctx, []string{"guitar-id", "bass-id"})
		require.NoError(t, err)
		assert.Equal(t, map[string]domain.PracticeSubjectCatalog{"guitar-id": newGuitar, "bass-id": catalogOf("bass-id", 0)}, got)
	})

	t.Run("a catalog built from a version raised since is not stored", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t))
		require.NoError(t, catalogs.BumpContentVersion(ctx))

		stored, err := catalogs.Store(ctx, 0, []domain.PracticeSubjectCatalog{catalogOf("guitar-id", 0, exercise)})

		require.NoError(t, err)
		assert.False(t, stored)
		got, _, err := catalogs.Catalogs(ctx, []string{"guitar-id"})
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a raise while a store holds the version waits for it, so the stored catalog ends out of date", func(t *testing.T) {
		client := setupPostgres(t)
		catalogs := NewEntPracticeSubjectCatalogRepository(client)
		require.NoError(t, catalogs.BumpContentVersion(ctx))
		tx, err := client.Tx(ctx)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, "SELECT version FROM practice_content_versions WHERE id = 1 FOR SHARE")
		require.NoError(t, err)

		raised := make(chan error, 1)
		go func() { raised <- catalogs.BumpContentVersion(ctx) }()
		select {
		case err := <-raised:
			t.Fatalf("the raise didn't wait for the share lock: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
		require.NoError(t, tx.Commit())

		require.NoError(t, <-raised)
		_, version, err := catalogs.Catalogs(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(2), version)
	})
}
