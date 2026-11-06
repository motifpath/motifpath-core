//go:build integration

package repo

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntProgressionRepository_List(t *testing.T) {
	ctx := context.Background()
	client := setupPostgres(t)
	repository := NewEntProgressionRepository(client)

	t.Run("no progressions lists none", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	pop, doowop := uuid.New(), uuid.New()
	client.Progression.Create().SetID(pop).SetKey("I-V-vi-IV").SetMode("major").SetBars([]string{"I", "V", "vi", "IV"}).
		SetNodeIds([]string{"node-cp"}).SetConfusable([]string{"I-vi-IV-V"}).SetNames(map[string]string{"en": "I–V–vi–IV (pop)"}).SetInDrills(true).SaveX(ctx)
	client.Progression.Create().SetID(doowop).SetKey("I-vi-IV-V").SetMode("major").SetBars([]string{"I", "vi", "IV", "V"}).
		SetNodeIds([]string{"node-cp"}).SetConfusable([]string{"I-V-vi-IV"}).SetNames(map[string]string{"en": "I–vi–IV–V"}).SetInDrills(true).SaveX(ctx)

	t.Run("lists every progression by key", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, domain.Progression{
			ID: pop.String(), Key: "I-V-vi-IV", Mode: "major", Bars: []string{"I", "V", "vi", "IV"}, NodeIDs: []string{"node-cp"},
			Confusable: []string{"I-vi-IV-V"}, Names: domain.LocalizedText{"en": "I–V–vi–IV (pop)"}, InDrills: true,
		}, got[0])
		assert.Equal(t, "I-vi-IV-V", got[1].Key)
	})

	creator := uuid.New()
	createdAt := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	client.Progression.Create().SetID(uuid.New()).SetKey("major/I-bVII-IV-I").SetMode("major").SetBars([]string{"I", "♭VII", "IV", "I"}).
		SetNodeIds([]string{}).SetConfusable([]string{}).SetNames(map[string]string{"en": "I–♭VII–IV–I (rock)"}).
		SetNicknames(map[string]string{"en": "rock"}).SetCreatedBy(creator).SetCreatedAt(createdAt).SaveX(ctx)

	t.Run("a created progression is chart only unless it says otherwise, and keeps who created it", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		require.Len(t, got, 3)
		created := got[2]
		assert.Equal(t, "major/I-bVII-IV-I", created.Key)
		assert.False(t, created.InDrills)
		assert.Equal(t, map[string]string{"en": "rock"}, created.Nicknames)
		require.NotNil(t, created.CreatedBy)
		assert.Equal(t, creator.String(), *created.CreatedBy)
		require.NotNil(t, created.CreatedAt)
		assert.True(t, createdAt.Equal(*created.CreatedAt))
	})

	t.Run("a catalog progression has no creator", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		assert.Nil(t, got[0].CreatedBy)
		assert.Nil(t, got[0].CreatedAt)
	})

	t.Run("the drills read only the progressions in the drills", func(t *testing.T) {
		got, err := repository.ListInDrills(ctx)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "I-V-vi-IV", got[0].Key)
		assert.Equal(t, "I-vi-IV-V", got[1].Key)
	})

	t.Run("the same mode and numerals can't be stored twice", func(t *testing.T) {
		err := client.Progression.Create().SetID(uuid.New()).SetKey("major/I-V-vi-IV").SetMode("major").SetBars([]string{"I", "V", "vi", "IV"}).
			SetNodeIds([]string{}).SetConfusable([]string{}).SetNames(map[string]string{"en": "I–V–vi–IV"}).Exec(ctx)

		assert.True(t, ent.IsConstraintError(err), "got %v", err)
	})

	t.Run("the same numerals in the other mode are another progression", func(t *testing.T) {
		err := client.Progression.Create().SetID(uuid.New()).SetKey("minor/I-V-vi-IV").SetMode("minor").SetBars([]string{"I", "V", "vi", "IV"}).
			SetNodeIds([]string{}).SetConfusable([]string{}).SetNames(map[string]string{"en": "I–V–vi–IV"}).Exec(ctx)

		assert.NoError(t, err)
	})

	t.Run("the longest key the grammar allows fits", func(t *testing.T) {
		numeral := "bVIImaj7"
		key := "major/" + strings.TrimSuffix(strings.Repeat(numeral+"-", 16), "-")
		bars := slices.Repeat([]string{"♭VIImaj7"}, 16)

		err := client.Progression.Create().SetID(uuid.New()).SetKey(key).SetMode("major").SetBars(bars).
			SetNodeIds([]string{}).SetConfusable([]string{}).SetNames(map[string]string{"en": "long"}).Exec(ctx)

		assert.NoError(t, err)
	})
}
