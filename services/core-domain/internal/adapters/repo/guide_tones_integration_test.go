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

// TestGuideToneItems classifies the guide tones the progression catalog and the
// knowledge map install, so it migrates an empty database rather than creating
// the bare schema.
func TestGuideToneItems(t *testing.T) {
	ctx := context.Background()
	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	source := NewEntNodeItemSource(ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db))))

	// guideTones keeps instrumentID's guide-tone items, each with its nodes.
	guideTones := func(instrumentID string) map[string][]string {
		items, err := source.ClassifiedItems(ctx, instrumentID)
		require.NoError(t, err)
		found := map[string][]string{}
		for _, item := range items {
			if kind, _, _ := strings.Cut(item.ItemKey, ":"); domain.PracticeItemKind(kind) == domain.PracticeItemKindGuideTone {
				found[item.ItemKey] = item.NodeIDs
			}
		}
		return found
	}
	node := func(key string) string { return catalogID("knowledge-node/" + key) }

	t.Run("on guitar, every progression is a guide tone under targeting chord tones and voice leading", func(t *testing.T) {
		guitar := guideTones(acousticGuitarID)
		require.Len(t, guitar, 10)
		assert.Equal(t, []string{node("target-chord-tones"), node("voice-lead")}, guitar["guide_tone:"+acousticGuitarID+":ii-V-I"])
		assert.Contains(t, guitar, "guide_tone:"+acousticGuitarID+":12-bar-blues")
	})

	t.Run("an electric guitar taps the same items, keyed by the layout it shares", func(t *testing.T) {
		assert.Equal(t, guideTones(acousticGuitarID), guideTones(electricGuitarID))
	})

	t.Run("on bass, every progression is a guide tone under targeting chord tones and chord-tone lines", func(t *testing.T) {
		bass := guideTones(electricBassID)
		require.Len(t, bass, 10)
		assert.Equal(t, []string{node("target-chord-tones"), node("play-chord-tone-lines")}, bass["guide_tone:"+electricBassID+":ii-V-I"])
	})
}
