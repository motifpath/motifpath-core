package application_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

var commercialNow = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

const (
	commercialAdminID = "a0000000-0000-4000-8000-000000000001"
	commercialKeyK    = "c0000000-0000-4000-8000-00000000000a"
)

func commercialTestEnvironment(t *testing.T) domain.Environment {
	t.Helper()
	env, err := domain.ParseEnvironment("test")
	require.NoError(t, err)
	return env
}

func testIntent(t *testing.T, body string) application.CommercialIntent {
	t.Helper()
	return application.CommercialIntent{
		Key: domain.OperationKey{
			Environment: commercialTestEnvironment(t), ActorKind: domain.AuditActorUser, ActorID: commercialAdminID,
			Action: "commercial_rollout.flag_changed", ResourceKind: "commercial_flag", ResourceID: "content_ownership",
			CallerKey: commercialKeyK,
		},
		Version:        1,
		Body:           json.RawMessage(body),
		Reason:         "Foundation rehearsal",
		PolicyRevision: domain.CommercialRegistryRevision,
	}
}

func allowCaller(context.Context) error { return nil }

// bumpRevision is a test effect: it changes the activation revision and
// reports it, like an activation change does.
func bumpRevision(calls *int) application.CommercialEffect {
	return func(ctx context.Context, tx ports.CommercialTx, run application.CommercialRun) (application.CommercialEffectResult, error) {
		*calls++
		if _, err := tx.Activation().LockExclusive(ctx); err != nil {
			return application.CommercialEffectResult{}, err
		}
		revision, err := tx.Activation().BumpRevision(ctx)
		if err != nil {
			return application.CommercialEffectResult{}, err
		}
		return application.CommercialEffectResult{
			Result:                json.RawMessage(fmt.Sprintf(`{"revision":%d,"evidence_entry_id":%q}`, revision, run.EvidenceEntryID)),
			AuthorizedRevision:    revision - 1,
			ConfigurationRevision: fmt.Sprint(revision),
			Before:                json.RawMessage(`{"enabled":false}`),
			After:                 json.RawMessage(`{"enabled":true}`),
		}, nil
	}
}

func refuseWith(err error) application.CommercialEffect {
	return func(context.Context, ports.CommercialTx, application.CommercialRun) (application.CommercialEffectResult, error) {
		return application.CommercialEffectResult{}, err
	}
}

func newTestCommercialCommands(store *fakeCommercialStore) *application.CommercialCommands {
	return application.NewCommercialCommands(store, func() time.Time { return commercialNow }, uuid.NewString)
}

func TestCommercialCommands_Run(t *testing.T) {
	ctx := context.Background()

	t.Run("a first submission applies, records the operation and commits evidence", func(t *testing.T) {
		store := newFakeCommercialStore()
		calls := 0

		receipt, err := newTestCommercialCommands(store).Run(ctx, testIntent(t, `{"enabled":true}`), allowCaller, bumpRevision(&calls))

		require.NoError(t, err)
		assert.False(t, receipt.Replayed)
		assert.Equal(t, commercialKeyK, receipt.OperationID)
		assert.Equal(t, 1, calls)
		assert.Equal(t, int64(1), store.state.revision)
		require.Len(t, store.state.operations, 1)
		assert.Equal(t, int64(0), store.state.operations[0].AuthorizedRevision)
		assert.Equal(t, []domain.AuditOutcome{domain.AuditOutcomeApplied}, store.attemptOutcomes())
		require.Equal(t, []domain.AuditOutcome{domain.AuditOutcomeApplied}, store.auditOutcomes())

		evidence := store.state.audit[0].Entry
		assert.JSONEq(t, fmt.Sprintf(`{"revision":1,"evidence_entry_id":%q}`, evidence.AuditEntryID), string(receipt.Result))
		assert.Equal(t, "Foundation rehearsal", evidence.Reason)
		assert.Equal(t, domain.CommercialRegistryRevision, evidence.PolicyRevision)
		assert.Equal(t, "1", evidence.ConfigurationRevision)
		assert.JSONEq(t, `{"enabled":false}`, string(evidence.BeforeState))
		assert.JSONEq(t, `{"enabled":true}`, string(evidence.AfterState))
		assert.Equal(t, commercialNow, evidence.RecordedAt)
	})

	t.Run("a same-intent replay returns the original result without applying again", func(t *testing.T) {
		store := newFakeCommercialStore()
		commands := newTestCommercialCommands(store)
		calls := 0
		first, err := commands.Run(ctx, testIntent(t, `{"enabled":true,"all_users":false}`), allowCaller, bumpRevision(&calls))
		require.NoError(t, err)

		// Key order differs; the intent is the same.
		replay, err := commands.Run(ctx, testIntent(t, `{"all_users":false,"enabled":true}`), allowCaller, bumpRevision(&calls))

		require.NoError(t, err)
		assert.True(t, replay.Replayed)
		assert.Equal(t, first.Result, replay.Result)
		assert.Equal(t, 1, calls)
		assert.Equal(t, int64(1), store.state.revision)
		assert.Equal(t, []domain.AuditOutcome{domain.AuditOutcomeApplied, domain.AuditOutcomeDuplicate}, store.attemptOutcomes())
		assert.Equal(t, []domain.AuditOutcome{domain.AuditOutcomeApplied, domain.AuditOutcomeDuplicate}, store.auditOutcomes())
	})

	t.Run("a replay is answered before the effect could refuse a stale revision", func(t *testing.T) {
		store := newFakeCommercialStore()
		commands := newTestCommercialCommands(store)
		calls := 0
		first, err := commands.Run(ctx, testIntent(t, `{"enabled":true}`), allowCaller, bumpRevision(&calls))
		require.NoError(t, err)

		replay, err := commands.Run(ctx, testIntent(t, `{"enabled":true}`), allowCaller, refuseWith(domain.ErrConflict))

		require.NoError(t, err)
		assert.Equal(t, first.Result, replay.Result)
	})

	t.Run("reusing a key for a different intent is refused and changes nothing", func(t *testing.T) {
		store := newFakeCommercialStore()
		commands := newTestCommercialCommands(store)
		calls := 0
		_, err := commands.Run(ctx, testIntent(t, `{"enabled":true}`), allowCaller, bumpRevision(&calls))
		require.NoError(t, err)

		_, err = commands.Run(ctx, testIntent(t, `{"enabled":false}`), allowCaller, bumpRevision(&calls))

		require.ErrorIs(t, err, domain.ErrConflict)
		assert.Equal(t, 1, calls)
		assert.Equal(t, int64(1), store.state.revision)
		assert.Equal(t, []domain.AuditOutcome{domain.AuditOutcomeApplied, domain.AuditOutcomeDenied}, store.attemptOutcomes(), "the refused reuse stays traceable")
		assert.Equal(t, []domain.AuditOutcome{domain.AuditOutcomeApplied, domain.AuditOutcomeDenied}, store.auditOutcomes())
	})

	t.Run("an unauthorized caller is refused before anything is looked up or applied", func(t *testing.T) {
		store := newFakeCommercialStore()
		calls := 0

		_, err := newTestCommercialCommands(store).Run(ctx, testIntent(t, `{"enabled":true}`),
			func(context.Context) error { return domain.ErrForbidden }, bumpRevision(&calls))

		require.ErrorIs(t, err, domain.ErrForbidden)
		assert.Equal(t, 0, calls)
		assert.Equal(t, int64(0), store.state.revision)
		assert.Empty(t, store.state.operations)
		require.Equal(t, []domain.AuditOutcome{domain.AuditOutcomeDenied}, store.auditOutcomes())
		assert.Nil(t, store.state.audit[0].Entry.BeforeState, "no authoritative state was read")
	})

	t.Run("an unauthorized caller cannot learn a recorded result by replaying its key", func(t *testing.T) {
		store := newFakeCommercialStore()
		commands := newTestCommercialCommands(store)
		calls := 0
		_, err := commands.Run(ctx, testIntent(t, `{"enabled":true}`), allowCaller, bumpRevision(&calls))
		require.NoError(t, err)

		receipt, err := commands.Run(ctx, testIntent(t, `{"enabled":true}`),
			func(context.Context) error { return domain.ErrForbidden }, bumpRevision(&calls))

		require.ErrorIs(t, err, domain.ErrForbidden)
		assert.Empty(t, receipt.Result)
	})

	for name, refusal := range map[string]error{
		"a stale revision":  fmt.Errorf("stale: %w", domain.ErrConflict),
		"an unknown target": fmt.Errorf("flag: %w", domain.ErrNotFound),
		"an invalid change": domain.NewValidationError("subject_ids", "names an unknown group"),
	} {
		t.Run("an effect refused for "+name+" changes nothing and is evidenced", func(t *testing.T) {
			store := newFakeCommercialStore()

			_, err := newTestCommercialCommands(store).Run(ctx, testIntent(t, `{"enabled":true}`), allowCaller, refuseWith(refusal))

			require.ErrorIs(t, err, refusal)
			assert.Empty(t, store.state.operations, "the key stays free for a corrected retry")
			assert.Equal(t, []domain.AuditOutcome{domain.AuditOutcomeDenied}, store.auditOutcomes())
		})
	}

	t.Run("evidence that cannot be written leaves nothing changed", func(t *testing.T) {
		store := newFakeCommercialStore()
		store.auditFails = true
		calls := 0

		_, err := newTestCommercialCommands(store).Run(ctx, testIntent(t, `{"enabled":true}`), allowCaller, bumpRevision(&calls))

		require.ErrorIs(t, err, domain.ErrUnavailable)
		assert.Equal(t, int64(0), store.state.revision)
		assert.Empty(t, store.state.operations)
		assert.Empty(t, store.state.attempts)
	})

	t.Run("unavailable storage is unavailable, not a refusal", func(t *testing.T) {
		store := newFakeCommercialStore()
		store.storageFails = true
		calls := 0

		_, err := newTestCommercialCommands(store).Run(ctx, testIntent(t, `{"enabled":true}`), allowCaller, bumpRevision(&calls))

		require.ErrorIs(t, err, domain.ErrUnavailable)
		assert.Equal(t, 0, calls)
	})

	t.Run("a concurrent submission of the same key converges on one operation", func(t *testing.T) {
		store := newFakeCommercialStore()
		commands := newTestCommercialCommands(store)
		calls := 0
		intent := testIntent(t, `{"enabled":true}`)
		winner := json.RawMessage(`{"revision":1,"winner":true}`)
		fingerprint, err := domain.FingerprintIntent(intent.Key.Action, intent.Version, intent.Key.ResourceKind+":"+intent.Key.ResourceID, intent.Body)
		require.NoError(t, err)
		store.onInsert = func(state *fakeCommercialState) {
			state.operations = append(state.operations, domain.CommercialOperation{
				Key: intent.Key, Fingerprint: fingerprint, Result: winner, CreatedAt: commercialNow,
			})
		}

		receipt, err := commands.Run(ctx, intent, allowCaller, bumpRevision(&calls))

		require.NoError(t, err)
		assert.True(t, receipt.Replayed)
		assert.JSONEq(t, string(winner), string(receipt.Result))
		assert.Len(t, store.state.operations, 1)
	})

	t.Run("the same caller key from another administrator is another operation", func(t *testing.T) {
		store := newFakeCommercialStore()
		commands := newTestCommercialCommands(store)
		calls := 0
		_, err := commands.Run(ctx, testIntent(t, `{"enabled":true}`), allowCaller, bumpRevision(&calls))
		require.NoError(t, err)
		other := testIntent(t, `{"enabled":false}`)
		other.Key.ActorID = "a0000000-0000-4000-8000-000000000002"

		receipt, err := commands.Run(ctx, other, allowCaller, bumpRevision(&calls))

		require.NoError(t, err)
		assert.False(t, receipt.Replayed)
		assert.Equal(t, 2, calls)
	})

	t.Run("an incompletely scoped key is a validation error", func(t *testing.T) {
		store := newFakeCommercialStore()
		intent := testIntent(t, `{"enabled":true}`)
		intent.Key.CallerKey = "not-a-uuid"
		calls := 0

		_, err := newTestCommercialCommands(store).Run(ctx, intent, allowCaller, bumpRevision(&calls))

		require.ErrorIs(t, err, domain.ErrValidation)
		assert.Equal(t, 0, calls)
	})
}
