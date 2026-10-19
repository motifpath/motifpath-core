//go:build integration

package repo

import (
	"context"
	"strings"
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

	t.Run("an instrument's chord items are classified under their skills", func(t *testing.T) {
		source := NewEntNodeItemSource(client)
		// byKind keeps instrumentID's chord items by kind, each with its nodes.
		byKind := func(instrumentID string) map[domain.PracticeItemKind]map[string][]string {
			items, err := source.ClassifiedItems(ctx, instrumentID)
			require.NoError(t, err)
			found := map[domain.PracticeItemKind]map[string][]string{}
			for _, item := range items {
				kind, _, _ := strings.Cut(item.ItemKey, ":")
				switch k := domain.PracticeItemKind(kind); k {
				case domain.PracticeItemKindChordVoicing, domain.PracticeItemKindChordChange, domain.PracticeItemKindRootFifth, domain.PracticeItemKindRootChange:
					if found[k] == nil {
						found[k] = map[string][]string{}
					}
					found[k][item.ItemKey] = item.NodeIDs
				}
			}
			return found
		}

		guitar := byKind(acousticGuitarID)
		require.Len(t, guitar[domain.PracticeItemKindChordVoicing], 8)
		require.Len(t, guitar[domain.PracticeItemKindChordChange], 10)
		assert.Empty(t, guitar[domain.PracticeItemKindRootFifth])
		amC := "chord_change:" + catalogID("chord-diagram/chord-voicing/a-minor-open") + ":" + catalogID("chord-diagram/chord-voicing/c-open")
		assert.Equal(t, []string{catalogID("knowledge-node/change-chords")}, guitar[domain.PracticeItemKindChordChange][amC])
		assert.Equal(t, []string{catalogID("knowledge-node/play-open-chords")},
			guitar[domain.PracticeItemKindChordVoicing]["chord_voicing:"+catalogID("chord-diagram/chord-voicing/a-minor-open")])

		assert.Equal(t, guitar, byKind(electricGuitarID), "the electric guitar plays the same chords, with the same keys")

		bass := byKind(electricBassID)
		assert.Empty(t, bass[domain.PracticeItemKindChordVoicing])
		require.Len(t, bass[domain.PracticeItemKindRootFifth], 8)
		require.Len(t, bass[domain.PracticeItemKindRootChange], 10)
		assert.Equal(t, []string{catalogID("knowledge-node/play-root-fifth-lines")}, bass[domain.PracticeItemKindRootFifth]["root_fifth:"+electricBassID+":Am"])
		assert.Equal(t, []string{catalogID("knowledge-node/play-root-notes")}, bass[domain.PracticeItemKindRootChange]["root_change:"+electricBassID+":Am:C"])
	})
}
