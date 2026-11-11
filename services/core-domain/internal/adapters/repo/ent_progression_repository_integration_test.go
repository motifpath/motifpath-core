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

func TestEntProgressionRepository_Authoring(t *testing.T) {
	ctx := context.Background()
	client := setupPostgres(t)
	repository := NewEntProgressionRepository(client)
	creator := uuid.NewString()
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	rock := domain.Progression{
		ID: uuid.NewString(), Key: "major/I-bVII-IV-I", Mode: "major", Bars: []string{"I", "♭VII", "IV", "I"}, NodeIDs: []string{}, Confusable: []string{},
		Names: domain.LocalizedText{"en": "I–♭VII–IV–I"}, CreatedBy: &creator, CreatedAt: &now,
	}
	require.NoError(t, repository.Create(ctx, rock))

	t.Run("it reads back by id and by numerals", func(t *testing.T) {
		byID, err := repository.GetByID(ctx, rock.ID)
		require.NoError(t, err)
		assert.Equal(t, rock.Key, byID.Key)
		assert.Equal(t, creator, *byID.CreatedBy)

		byNumerals, err := repository.FindByNumerals(ctx, "major", rock.Bars)
		require.NoError(t, err)
		assert.Equal(t, rock.ID, byNumerals.ID)
	})

	t.Run("not found is said so", func(t *testing.T) {
		_, err := repository.GetByID(ctx, uuid.NewString())
		assert.ErrorIs(t, err, domain.ErrNotFound)
		_, err = repository.GetByID(ctx, "not-a-uuid")
		assert.ErrorIs(t, err, domain.ErrNotFound)
		_, err = repository.FindByNumerals(ctx, "minor", rock.Bars)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("the same mode and numerals conflict", func(t *testing.T) {
		twin := rock
		twin.ID, twin.Key = uuid.NewString(), "major/other"
		assert.ErrorIs(t, repository.Create(ctx, twin), domain.ErrConflict)
	})

	t.Run("nicknames and names change and can be removed", func(t *testing.T) {
		named := rock
		named.Names, named.Nicknames = domain.LocalizedText{"en": "I–♭VII–IV–I (rock)"}, map[string]string{"en": "rock"}
		require.NoError(t, repository.SaveNames(ctx, named))
		got, err := repository.GetByID(ctx, rock.ID)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"en": "rock"}, got.Nicknames)
		assert.Equal(t, "I–♭VII–IV–I (rock)", got.Names["en"])

		named.Nicknames = nil
		require.NoError(t, repository.SaveNames(ctx, named))
		got, err = repository.GetByID(ctx, rock.ID)
		require.NoError(t, err)
		assert.Empty(t, got.Nicknames)
		assert.Equal(t, rock.Key, got.Key)
	})

	t.Run("a chart-only progression enters the drills once", func(t *testing.T) {
		drilled := rock
		drilled.InDrills, drilled.NodeIDs, drilled.Confusable = true, []string{uuid.NewString()}, []string{"I-V-vi-IV"}
		require.NoError(t, repository.SaveInDrills(ctx, drilled))
		got, err := repository.GetByID(ctx, rock.ID)
		require.NoError(t, err)
		assert.True(t, got.InDrills)
		assert.Equal(t, drilled.NodeIDs, got.NodeIDs)
		assert.Equal(t, drilled.Confusable, got.Confusable)

		again := drilled
		again.NodeIDs = []string{uuid.NewString()}
		assert.ErrorIs(t, repository.SaveInDrills(ctx, again), domain.ErrConflict)
		got, err = repository.GetByID(ctx, rock.ID)
		require.NoError(t, err)
		assert.Equal(t, drilled.NodeIDs, got.NodeIDs)

		missing := drilled
		missing.ID = uuid.NewString()
		assert.ErrorIs(t, repository.SaveInDrills(ctx, missing), domain.ErrNotFound)

		// Put it back for the steps below: only drills keep a progression from removal.
		_, err = client.Progression.UpdateOneID(uuid.MustParse(rock.ID)).SetInDrills(false).Save(ctx)
		require.NoError(t, err)
	})

	t.Run("deleting frees its numerals", func(t *testing.T) {
		require.NoError(t, repository.Delete(ctx, rock.ID))
		_, err := repository.GetByID(ctx, rock.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.ErrorIs(t, repository.Delete(ctx, rock.ID), domain.ErrNotFound)
		assert.NoError(t, repository.Create(ctx, rock))
	})
}
