//go:build integration

package repo

import (
	"context"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

// TestSongItems classifies the songs of the charts stored, under the nodes the
// progression catalog and the knowledge map install, so it migrates an empty
// database rather than creating the bare schema.
func TestSongItems(t *testing.T) {
	ctx := context.Background()
	db := startMigrationPostgres(t, ctx)
	for _, file := range migrationFiles(t) {
		_, err := execMigrationFile(ctx, db, file)
		require.NoError(t, err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	source := NewEntNodeItemSource(client)
	charts := NewEntSongChartRepository(client)

	// publish stores a chart tagged with progressions and publishes it; later
	// tags make later revisions.
	publish := func(t *testing.T, tags ...[]string) domain.SongChart {
		t.Helper()
		chart := newSongChart("Song "+uuid.NewString()[:8], fixedAt)
		chart.Draft.RightsConfirmation = &domain.RightsConfirmation{ConfirmedBy: uuid.NewString(), ConfirmedAt: fixedAt}
		chart = tagged(chart, tags[0]...)
		require.NoError(t, charts.Create(ctx, chart))
		for i, revisionTags := range tags {
			chart = tagged(chart, revisionTags...)
			published, rev, err := chart.Publish(uuid.NewString(), fixedAt.Add(time.Duration(i)*time.Hour))
			require.NoError(t, err)
			require.NoError(t, charts.Publish(ctx, published, rev))
			chart = published
		}
		return chart
	}
	// songs keeps instrumentID's song items, each with its nodes.
	songs := func(instrumentID string) map[string][]string {
		items, err := source.ClassifiedItems(ctx, instrumentID)
		require.NoError(t, err)
		found := map[string][]string{}
		for _, item := range items {
			if kind, _, _ := strings.Cut(item.ItemKey, ":"); domain.PracticeItemKind(kind) == domain.PracticeItemKindSong {
				found[item.ItemKey] = item.NodeIDs
			}
		}
		return found
	}
	node := func(key string) string { return catalogID("knowledge-node/" + key) }

	amazingGrace := publish(t, []string{"I-IV-I-V", "I-IV-V-I"})
	untagged := publish(t, nil)
	twoSections := newSongChart("Two sections", fixedAt)
	twoSections.Draft.RightsConfirmation = &domain.RightsConfirmation{ConfirmedBy: uuid.NewString(), ConfirmedAt: fixedAt}
	chorus := twoSections.Draft.Body.Sections[0]
	chorus.Kind, chorus.ProgressionKeys = domain.SectionChorus, []string{"12-bar-blues", "I-IV-I-V"}
	twoSections = tagged(twoSections, "I-IV-I-V")
	twoSections.Draft.Body.Sections = append(twoSections.Draft.Body.Sections, chorus)
	require.NoError(t, charts.Create(ctx, twoSections))
	twoSectionsPublished, twoSectionsRev, err := twoSections.Publish(uuid.NewString(), fixedAt)
	require.NoError(t, err)
	require.NoError(t, charts.Publish(ctx, twoSectionsPublished, twoSectionsRev))
	retagged := publish(t, []string{"I-IV-I-V"}, []string{"12-bar-blues"})
	withdrawn := publish(t, []string{"I-IV-I-V"})
	withdrawn, err = withdrawn.Withdraw(uuid.NewString(), fixedAt.Add(time.Hour), "Rights disputed")
	require.NoError(t, err)
	require.NoError(t, charts.Save(ctx, withdrawn))
	draft := tagged(newSongChart("Draft", fixedAt), "I-IV-I-V")
	require.NoError(t, charts.Create(ctx, draft))

	guitar := songs(acousticGuitarID)

	t.Run("a published chart is a song on the guitar layout, under its progressions' nodes", func(t *testing.T) {
		assert.Equal(t, []string{node("common-progressions"), node("cadences")},
			guitar["song:"+acousticGuitarID+":"+amazingGrace.ID])
	})

	t.Run("a song is under the nodes of every progression its sections are tagged with", func(t *testing.T) {
		assert.Equal(t, []string{node("common-progressions"), node("blues-form")},
			guitar["song:"+acousticGuitarID+":"+twoSections.ID])
	})

	t.Run("a song takes the progressions of the chart's latest revision", func(t *testing.T) {
		assert.Equal(t, []string{node("common-progressions"), node("blues-form")},
			guitar["song:"+acousticGuitarID+":"+retagged.ID])
	})

	t.Run("an untagged, withdrawn or never published chart is no song", func(t *testing.T) {
		for _, id := range []string{untagged.ID, withdrawn.ID, draft.ID} {
			assert.NotContains(t, guitar, "song:"+acousticGuitarID+":"+id)
		}
	})

	t.Run("an electric guitar has the same songs, keyed by the layout it shares", func(t *testing.T) {
		assert.Equal(t, guitar, songs(electricGuitarID))
	})

	t.Run("on bass, the same charts are songs on the bass layout", func(t *testing.T) {
		assert.Equal(t, []string{node("common-progressions"), node("cadences")},
			songs(electricBassID)["song:"+electricBassID+":"+amazingGrace.ID])
	})

	t.Run("a session in the head across instruments has no songs", func(t *testing.T) {
		assert.Empty(t, songs(""))
	})
}
