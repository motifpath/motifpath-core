//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntDrillThresholdRepository_List(t *testing.T) {
	ctx := context.Background()
	client := setupPostgres(t)
	repository := NewEntDrillThresholdRepository(client)

	t.Run("no thresholds lists none", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	text := client.DrillTemplate.Create().SetID(uuid.New()).SetKey("exercise:text_response").SetItemKind("exercise").
		SetResponseType("option_choice").SetTimed(true).SetNames(map[string]string{"en": "Text"}).SaveX(ctx)
	v1, v2 := uuid.New(), uuid.New()
	october := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	november := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	client.DrillThreshold.Create().SetID(v2).SetTemplateID(text.ID).SetVersion(2).SetEffectiveFrom(november).
		SetFluentNetMs(4500).SetSource("benchmark").SetSessions(0).SetStudents(0).SaveX(ctx)
	client.DrillThreshold.Create().SetID(v1).SetTemplateID(text.ID).SetVersion(1).SetEffectiveFrom(october).
		SetFluentNetMs(6000).SetSource("default").SetSessions(0).SetStudents(0).SaveX(ctx)

	t.Run("lists every version with its template's key, by template then version", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, domain.DrillThreshold{ID: v1.String(), TemplateKey: "exercise:text_response", Version: 1,
			EffectiveFrom: october, FluentNetMs: 6000, Source: domain.DrillThresholdSourceDefault}, got[0])
		assert.Equal(t, 2, got[1].Version)
		assert.True(t, november.Equal(got[1].EffectiveFrom))
		assert.Equal(t, domain.DrillThresholdSourceBenchmark, got[1].Source)
	})
}
