//go:build integration

package repo

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntEmailSendLog(t *testing.T) {
	report := domain.EmailSendClaim{Recipient: "user-alice", Kind: domain.EmailSendProgressReport, PeriodKey: "2026-W44", At: fixedAt}

	t.Run("a first claim succeeds and is then claimed", func(t *testing.T) {
		log := NewEntEmailSendLog(setupPostgres(t))
		ctx := context.Background()

		claimed, err := log.Claim(ctx, report)
		require.NoError(t, err)
		assert.True(t, claimed)

		done, err := log.Claimed(ctx, report)
		require.NoError(t, err)
		assert.True(t, done)
	})

	t.Run("the same recipient, kind and period can be claimed only once", func(t *testing.T) {
		log := NewEntEmailSendLog(setupPostgres(t))
		ctx := context.Background()
		_, err := log.Claim(ctx, report)
		require.NoError(t, err)

		later := report
		later.At = fixedAt.Add(15 * time.Minute)
		claimed, err := log.Claim(ctx, later)

		require.NoError(t, err)
		assert.False(t, claimed)
	})

	t.Run("another period, kind or recipient is a claim of its own", func(t *testing.T) {
		log := NewEntEmailSendLog(setupPostgres(t))
		ctx := context.Background()
		_, err := log.Claim(ctx, report)
		require.NoError(t, err)

		for _, other := range []domain.EmailSendClaim{
			{Recipient: "user-alice", Kind: domain.EmailSendProgressReport, PeriodKey: "2026-W45", At: fixedAt},
			{Recipient: "user-alice", Kind: domain.EmailSendReminder, PeriodKey: "2026-W44", At: fixedAt},
			{Recipient: "user-bob", Kind: domain.EmailSendProgressReport, PeriodKey: "2026-W44", At: fixedAt},
		} {
			claimed, err := log.Claim(ctx, other)
			require.NoError(t, err)
			assert.True(t, claimed, "%+v", other)
		}
	})

	t.Run("a released claim can be claimed again", func(t *testing.T) {
		log := NewEntEmailSendLog(setupPostgres(t))
		ctx := context.Background()
		_, err := log.Claim(ctx, report)
		require.NoError(t, err)

		require.NoError(t, log.Release(ctx, report))

		done, err := log.Claimed(ctx, report)
		require.NoError(t, err)
		assert.False(t, done)
		claimed, err := log.Claim(ctx, report)
		require.NoError(t, err)
		assert.True(t, claimed)
	})

	t.Run("of claims racing for the same email exactly one wins", func(t *testing.T) {
		log := NewEntEmailSendLog(setupPostgres(t))
		ctx := context.Background()

		var wg sync.WaitGroup
		var mu sync.Mutex
		wins := 0
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				claimed, err := log.Claim(ctx, report)
				assert.NoError(t, err)
				if claimed {
					mu.Lock()
					wins++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		assert.Equal(t, 1, wins)
	})
}
