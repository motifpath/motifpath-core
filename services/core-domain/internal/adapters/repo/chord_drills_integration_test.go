//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

// TestChordDrills reads the chord drills, pairs and change targets the
// migrations install, so it migrates an empty database rather than creating
// the bare schema.
func TestChordDrills(t *testing.T) {
	ctx := context.Background()
	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))

	catalog, err := NewEntChordDrillRepository(client).Catalog(ctx)
	require.NoError(t, err)

	t.Run("installs the eight open chords and ten pairs", func(t *testing.T) {
		chords := make([]string, len(catalog.Drills))
		for i, d := range catalog.Drills {
			chords[i] = d.Chord
		}
		assert.Equal(t, []string{"C", "A", "G", "E", "D", "Am", "Em", "Dm"}, chords)
		assert.Len(t, catalog.Changes, 10)
		assert.Equal(t, catalogID("chord-drill/a-minor-open"), catalog.Changes[0].FromChordDrillID)
		assert.Equal(t, catalogID("chord-drill/c-open"), catalog.Changes[0].ToChordDrillID)
	})

	t.Run("each chord drill plays a chord voicing diagram of the chord catalog", func(t *testing.T) {
		diagrams := NewEntDiagramRepository(client)
		for _, d := range catalog.Drills {
			diagram, err := diagrams.GetByID(ctx, d.DiagramID)
			require.NoError(t, err, d.Chord)
			assert.Equal(t, domain.DiagramPurposeChordVoicing, diagram.Purpose, d.Chord)
		}
	})

	t.Run("installs a first change target of 60 clean changes a minute", func(t *testing.T) {
		assert.Equal(t, []domain.ChangeTarget{{
			ID: catalogID("change-target/v1"), Version: 1, EffectiveFrom: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC),
			ChangesPerMinute: 60, Source: domain.DrillThresholdSourceDefault,
		}}, catalog.Targets)
	})
}
