//go:build integration

package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntPracticeSubjectCatalogRepository(t *testing.T) {
	ctx := context.Background()
	built := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	guitar := domain.PracticeSubjectCatalog{Scope: "guitar-id", FormatVersion: 1, BuiltAt: built, Items: []domain.PracticeSubjectCatalogItem{
		{ItemKey: "exercise:e1", NodeIDs: []string{"n1", "n2"}, InHand: true, InHead: true, Rung: domain.PracticeRungKnow},
		{ItemKey: "fretboard_cell:g:1:3", NodeIDs: []string{"n3"}, InHead: true, Ways: []domain.PracticeSubjectCatalogWay{
			{Way: "name_the_note", Rung: domain.PracticeRungKnow}, {Way: "find_the_note", Rung: domain.PracticeRungHear},
		}},
	}}
	none := domain.PracticeSubjectCatalog{Scope: "", FormatVersion: 1, BuiltAt: built, Items: []domain.PracticeSubjectCatalogItem{}}
	buildOf := func(catalogs ...domain.PracticeSubjectCatalog) func(context.Context) ([]domain.PracticeSubjectCatalog, error) {
		return func(context.Context) ([]domain.PracticeSubjectCatalog, error) { return catalogs, nil }
	}

	t.Run("with nothing pending, a rebuild builds nothing", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t), func() time.Time { return built })
		called := false

		rebuilt, err := catalogs.Rebuild(ctx, func(context.Context) ([]domain.PracticeSubjectCatalog, error) {
			called = true
			return nil, nil
		})

		require.NoError(t, err)
		assert.False(t, rebuilt)
		assert.False(t, called)
	})

	t.Run("a rebuild stores what it builds, by scope, and clears the signals", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t), func() time.Time { return built })
		require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalStart))
		require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalDiagram))

		rebuilt, err := catalogs.Rebuild(ctx, buildOf(guitar, none))

		require.NoError(t, err)
		assert.True(t, rebuilt)
		got, err := catalogs.Catalogs(ctx, []string{"guitar-id", "", "bass-id"})
		require.NoError(t, err)
		assert.Equal(t, map[string]domain.PracticeSubjectCatalog{"guitar-id": guitar, "": none}, got, "a scope with no catalog is absent")
		rebuilt, err = catalogs.Rebuild(ctx, buildOf(guitar))
		require.NoError(t, err)
		assert.False(t, rebuilt, "nothing is pending any more")
	})

	t.Run("a rebuild replaces every catalog", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t), func() time.Time { return built })
		require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalStart))
		_, err := catalogs.Rebuild(ctx, buildOf(guitar, none))
		require.NoError(t, err)
		require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalInstrument))
		later := none
		later.BuiltAt = built.Add(time.Hour)

		_, err = catalogs.Rebuild(ctx, buildOf(later))

		require.NoError(t, err)
		got, err := catalogs.Catalogs(ctx, []string{"guitar-id", ""})
		require.NoError(t, err)
		assert.Equal(t, map[string]domain.PracticeSubjectCatalog{"": later}, got)
	})

	t.Run("a signal queued while a rebuild builds stays for the next", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t), func() time.Time { return built })
		require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalStart))

		_, err := catalogs.Rebuild(ctx, func(ctx context.Context) ([]domain.PracticeSubjectCatalog, error) {
			require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalExercise))
			return []domain.PracticeSubjectCatalog{none}, nil
		})
		require.NoError(t, err)

		rebuilt, err := catalogs.Rebuild(ctx, buildOf(none))
		require.NoError(t, err)
		assert.True(t, rebuilt)
	})

	t.Run("while one rebuild builds, another builds nothing", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t), func() time.Time { return built })
		require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalStart))
		var other bool

		_, err := catalogs.Rebuild(ctx, func(ctx context.Context) ([]domain.PracticeSubjectCatalog, error) {
			var err error
			other, err = catalogs.Rebuild(ctx, buildOf(guitar))
			require.NoError(t, err)
			return []domain.PracticeSubjectCatalog{none}, nil
		})

		require.NoError(t, err)
		assert.False(t, other)
	})

	t.Run("a failed build keeps the catalogs and the signals", func(t *testing.T) {
		catalogs := NewEntPracticeSubjectCatalogRepository(setupPostgres(t), func() time.Time { return built })
		require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalStart))
		_, err := catalogs.Rebuild(ctx, buildOf(guitar))
		require.NoError(t, err)
		require.NoError(t, catalogs.Signal(ctx, domain.PracticeCatalogSignalDiagram))

		_, err = catalogs.Rebuild(ctx, func(context.Context) ([]domain.PracticeSubjectCatalog, error) {
			return nil, errors.New("build failed")
		})

		require.Error(t, err)
		got, err := catalogs.Catalogs(ctx, []string{"guitar-id"})
		require.NoError(t, err)
		assert.Equal(t, map[string]domain.PracticeSubjectCatalog{"guitar-id": guitar}, got)
		rebuilt, err := catalogs.Rebuild(ctx, buildOf(guitar))
		require.NoError(t, err)
		assert.True(t, rebuilt, "the signal is still pending")
	})
}
