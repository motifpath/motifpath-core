//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		SetNodeIds([]string{"node-cp"}).SetConfusable([]string{"I-vi-IV-V"}).SetNames(map[string]string{"en": "I–V–vi–IV (pop)"}).SaveX(ctx)
	client.Progression.Create().SetID(doowop).SetKey("I-vi-IV-V").SetMode("major").SetBars([]string{"I", "vi", "IV", "V"}).
		SetNodeIds([]string{"node-cp"}).SetConfusable([]string{"I-V-vi-IV"}).SetNames(map[string]string{"en": "I–vi–IV–V"}).SaveX(ctx)

	t.Run("lists every progression by key", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, domain.Progression{
			ID: pop.String(), Key: "I-V-vi-IV", Mode: "major", Bars: []string{"I", "V", "vi", "IV"}, NodeIDs: []string{"node-cp"},
			Confusable: []string{"I-vi-IV-V"}, Names: domain.LocalizedText{"en": "I–V–vi–IV (pop)"},
		}, got[0])
		assert.Equal(t, "I-vi-IV-V", got[1].Key)
	})
}
