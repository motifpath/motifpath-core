//go:build integration

package repo

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntAppConfigRepository(t *testing.T) {
	ctx := context.Background()
	client := setupPostgres(t)
	repository := NewEntAppConfigRepository(client)
	ana, bob := uuid.NewString(), uuid.NewString()
	october := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	november := time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)

	t.Run("a key never set is not found", func(t *testing.T) {
		_, err := repository.Get(ctx, "starting_points")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("nothing set lists nothing", func(t *testing.T) {
		got, err := repository.List(ctx)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a stored setting reads back as stored", func(t *testing.T) {
		entry := domain.AppConfigEntry{Key: "starting_points", Value: json.RawMessage(`{"lists":[]}`), UpdatedAt: october, UpdatedBy: ana}

		require.NoError(t, repository.Put(ctx, entry))
		got, err := repository.Get(ctx, "starting_points")

		require.NoError(t, err)
		assert.JSONEq(t, `{"lists":[]}`, string(got.Value))
		assert.Equal(t, "starting_points", got.Key)
		assert.True(t, october.Equal(got.UpdatedAt))
		assert.Equal(t, ana, got.UpdatedBy)
	})

	t.Run("putting a key again replaces its value and who changed it", func(t *testing.T) {
		replaced := domain.AppConfigEntry{Key: "starting_points", Value: json.RawMessage(`{"lists":[{"language":"en"}]}`), UpdatedAt: november, UpdatedBy: bob}

		require.NoError(t, repository.Put(ctx, replaced))
		got, err := repository.Get(ctx, "starting_points")

		require.NoError(t, err)
		assert.JSONEq(t, `{"lists":[{"language":"en"}]}`, string(got.Value))
		assert.True(t, november.Equal(got.UpdatedAt))
		assert.Equal(t, bob, got.UpdatedBy)
	})

	t.Run("every stored setting is listed, ordered by key", func(t *testing.T) {
		require.NoError(t, repository.Put(ctx, domain.AppConfigEntry{Key: "a_first", Value: json.RawMessage(`{}`), UpdatedAt: october, UpdatedBy: ana}))

		got, err := repository.List(ctx)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "a_first", got[0].Key)
		assert.Equal(t, "starting_points", got[1].Key)
	})
}
