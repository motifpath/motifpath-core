//go:build integration

package repo

import (
	"context"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

// TestProgressionKeyTourItems classifies the tours the progression catalog and
// the knowledge map install, so it migrates an empty database rather than
// creating the bare schema.
func TestProgressionKeyTourItems(t *testing.T) {
	ctx := context.Background()
	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	source := NewEntNodeItemSource(ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db))))

	// tours keeps instrumentID's tour items, each with its nodes.
	tours := func(instrumentID string) map[string][]string {
		items, err := source.ClassifiedItems(ctx, instrumentID)
		require.NoError(t, err)
		found := map[string][]string{}
		for _, item := range items {
			if kind, _, _ := strings.Cut(item.ItemKey, ":"); domain.PracticeItemKind(kind) == domain.PracticeItemKindProgressionKeys {
				found[item.ItemKey] = item.NodeIDs
			}
		}
		return found
	}
	node := func(key string) string { return catalogID("knowledge-node/" + key) }

	t.Run("on guitar, every progression is a tour on the guitar layout", func(t *testing.T) {
		guitar := tours(acousticGuitarID)
		require.Len(t, guitar, 10)
		assert.Equal(t, []string{node("common-progressions"), node("play-shapes-in-any-key")},
			guitar["progression_keys:"+acousticGuitarID+":I-V-vi-IV"])
		assert.Equal(t, []string{node("common-progressions"), node("play-shapes-in-any-key"), node("cadences")},
			guitar["progression_keys:"+acousticGuitarID+":ii-V-I"], "a tour also rolls up to its progression's nodes")
	})

	t.Run("an electric guitar tours the same items, keyed by the layout it shares", func(t *testing.T) {
		assert.Equal(t, tours(acousticGuitarID), tours(electricGuitarID))
	})

	t.Run("on bass, every progression is a tour under the chord-tone lines skill", func(t *testing.T) {
		bass := tours(electricBassID)
		require.Len(t, bass, 10)
		assert.Equal(t, []string{node("common-progressions"), node("play-chord-tone-lines")},
			bass["progression_keys:"+electricBassID+":I-V-vi-IV"])
	})
}
