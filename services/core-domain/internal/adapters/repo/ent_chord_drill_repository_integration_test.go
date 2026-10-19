//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent/changetarget"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntChordDrillRepository_Catalog(t *testing.T) {
	ctx := context.Background()
	client := setupPostgres(t)
	repository := NewEntChordDrillRepository(client)

	t.Run("nothing installed is an empty catalog", func(t *testing.T) {
		got, err := repository.Catalog(ctx)

		require.NoError(t, err)
		assert.Empty(t, got.Drills)
		assert.Empty(t, got.Changes)
		assert.Empty(t, got.Targets)
	})

	c, am := uuid.New(), uuid.New()
	cDiagram, amDiagram := uuid.New(), uuid.New()
	// Inserted out of catalog order, to show the catalog comes back in order.
	client.ChordDrill.Create().SetID(c).SetDiagramID(cDiagram).SetChord("C").SetOrdinal(2).SaveX(ctx)
	client.ChordDrill.Create().SetID(am).SetDiagramID(amDiagram).SetChord("Am").SetOrdinal(1).SaveX(ctx)
	pair := uuid.New()
	client.ChordChangePair.Create().SetID(pair).SetFromChordDrillID(am).SetToChordDrillID(c).SetOrdinal(1).SaveX(ctx)
	from := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	v2, v1 := uuid.New(), uuid.New()
	client.ChangeTarget.Create().SetID(v2).SetVersion(2).SetEffectiveFrom(from.AddDate(0, 1, 0)).SetChangesPerMinute(70).SetSource(changetarget.SourceBenchmark).SaveX(ctx)
	client.ChangeTarget.Create().SetID(v1).SetVersion(1).SetEffectiveFrom(from).SetChangesPerMinute(60).SetSource(changetarget.SourceDefault).SaveX(ctx)

	t.Run("lists the drills and pairs in catalog order and the targets by version", func(t *testing.T) {
		got, err := repository.Catalog(ctx)

		require.NoError(t, err)
		assert.Equal(t, []domain.ChordDrill{
			{ID: am.String(), DiagramID: amDiagram.String(), Chord: "Am", Ordinal: 1},
			{ID: c.String(), DiagramID: cDiagram.String(), Chord: "C", Ordinal: 2},
		}, got.Drills)
		assert.Equal(t, []domain.ChordChangePair{{ID: pair.String(), FromChordDrillID: am.String(), ToChordDrillID: c.String(), Ordinal: 1}}, got.Changes)
		assert.Equal(t, []domain.ChangeTarget{
			{ID: v1.String(), Version: 1, EffectiveFrom: from, ChangesPerMinute: 60, Source: domain.DrillThresholdSourceDefault},
			{ID: v2.String(), Version: 2, EffectiveFrom: from.AddDate(0, 1, 0), ChangesPerMinute: 70, Source: domain.DrillThresholdSourceBenchmark},
		}, got.Targets)
	})
}
