//go:build integration

package repo

import (
	"context"
	"slices"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

// TestCreatedProgressionsInTheDrills reads what the migrations install, so it
// migrates an empty database rather than creating the bare schema.
func TestCreatedProgressionsInTheDrills(t *testing.T) {
	ctx := context.Background()
	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))

	t.Run("every chord a progression can spell has a voicing in every root", func(t *testing.T) {
		chords := NewEntChordCatalogRepository(client)
		var missing []string
		for _, suffix := range []string{"", "m", "7", "maj7", "m7", "m7b5", "dim", "dim7"} {
			for pitchClass, root := range domain.MusicKeys {
				reading := domain.ParseChordSymbol(root + suffix)
				require.NotNil(t, reading.Parsed, root+suffix)
				chord, err := chords.FindChord(ctx, pitchClass, reading.Parsed.Quality, nil)
				if err == nil {
					if _, ok := domain.PreferredChordBox(chord.Voicings); ok {
						continue
					}
				}
				missing = append(missing, root+suffix)
			}
		}
		assert.Empty(t, missing)
	})

	progressions := NewEntProgressionRepository(client)
	admin := "00000000-0000-4000-8000-0000000000a1"
	node := catalogID("knowledge-node/common-progressions")
	inDrills := domain.Progression{
		ID: "00000000-0000-4000-8000-0000000000d1", Key: "major/I-bVII-IV-V", Mode: "major",
		Bars: []string{"I", "♭VII", "IV", "V"}, NodeIDs: []string{node}, Confusable: []string{"I-V-vi-IV"},
		Names: domain.LocalizedText{"en": "I–♭VII–IV–V"}, InDrills: true, CreatedBy: &admin,
	}
	chartOnly := domain.Progression{
		ID: "00000000-0000-4000-8000-0000000000d2", Key: "major/I-bVII-IV-I", Mode: "major",
		Bars: []string{"I", "♭VII", "IV", "I"}, NodeIDs: []string{}, Confusable: []string{},
		Names: domain.LocalizedText{"en": "I–♭VII–IV–I"}, CreatedBy: &admin,
	}
	require.NoError(t, progressions.Create(ctx, inDrills))
	require.NoError(t, progressions.Create(ctx, chartOnly))

	items, err := NewEntNodeItemSource(client).ClassifiedItems(ctx, acousticGuitarID)
	require.NoError(t, err)
	nodesOf := map[string][]string{}
	for _, item := range items {
		nodesOf[item.ItemKey] = item.NodeIDs
	}

	t.Run("a progression in the drills is practised in every drill, under its nodes", func(t *testing.T) {
		for _, key := range []string{
			domain.ChartProgressionItemKey(inDrills.Key),
			domain.HeardProgressionItemKey(inDrills.Key),
			domain.GuideToneItemKey(acousticGuitarID, inDrills.Key),
			domain.ProgressionKeysItemKey(acousticGuitarID, inDrills.Key),
		} {
			assert.Contains(t, nodesOf, key)
		}
		assert.Contains(t, nodesOf[domain.ChartProgressionItemKey(inDrills.Key)], node)
	})

	t.Run("a progression for charts only is never practised", func(t *testing.T) {
		for key := range nodesOf {
			assert.False(t, strings.HasSuffix(key, chartOnly.Key), key)
		}
	})

	t.Run("the drills read it back with its wrong answers", func(t *testing.T) {
		drills, err := progressions.ListInDrills(ctx)
		require.NoError(t, err)
		var keys []string
		for _, p := range drills {
			keys = append(keys, p.Key)
			if p.Key == inDrills.Key {
				assert.Equal(t, []string{"I-V-vi-IV"}, p.Confusable)
			}
		}
		assert.Contains(t, keys, inDrills.Key)
		assert.NotContains(t, keys, chartOnly.Key)
	})

	t.Run("a chart-only progression added later is practised under its nodes", func(t *testing.T) {
		added := chartOnly
		added.InDrills, added.NodeIDs, added.Confusable = true, []string{node}, []string{inDrills.Key}
		require.NoError(t, progressions.SaveInDrills(ctx, added))

		items, err := NewEntNodeItemSource(client).ClassifiedItems(ctx, acousticGuitarID)
		require.NoError(t, err)
		i := slices.IndexFunc(items, func(item domain.ClassifiedItem) bool {
			return item.ItemKey == domain.ChartProgressionItemKey(chartOnly.Key)
		})
		require.GreaterOrEqual(t, i, 0)
		assert.Contains(t, items[i].NodeIDs, node)
	})
}
