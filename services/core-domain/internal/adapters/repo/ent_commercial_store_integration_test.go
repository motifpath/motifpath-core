//go:build integration

package repo

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

var commercialAt = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func commercialEnvironment(t *testing.T, name string) domain.Environment {
	t.Helper()
	env, err := domain.ParseEnvironment(name)
	require.NoError(t, err)
	return env
}

func commercialKey(t *testing.T, env domain.Environment, actorID, callerKey string) domain.OperationKey {
	t.Helper()
	return domain.OperationKey{
		Environment: env, ActorKind: domain.AuditActorUser, ActorID: actorID,
		Action: "commercial_rollout.flag_changed", ResourceKind: "commercial_flag", ResourceID: "content_ownership",
		CallerKey: callerKey,
	}
}

func commercialEntry(t *testing.T, key domain.OperationKey) domain.AuditEntry {
	t.Helper()
	return domain.AuditEntry{
		AuditEntryID: uuid.NewString(), Operation: key, AttemptID: uuid.NewString(),
		OccurredAt: commercialAt, RecordedAt: commercialAt, Reason: "Foundation rehearsal",
		Outcome: domain.AuditOutcomeApplied, AfterState: json.RawMessage(`{"enabled":true}`),
		PolicyRevision: domain.CommercialRegistryRevision, ConfigurationRevision: "1",
		CorrelationID: uuid.NewString(), CausedByEntryIDs: []string{}, ProviderReferences: []string{},
	}
}

func createCommercialUser(t *testing.T, store *EntCommercialStore, clerkID string) string {
	t.Helper()
	user := domain.User{ID: uuid.NewString(), ClerkUserID: clerkID, Role: domain.RoleStudent, DisplayName: clerkID, Locale: domain.Language{Code: "en"}, RegisteredAt: commercialAt}
	require.NoError(t, NewEntUserRepository(store.client).Create(context.Background(), user))
	return user.ID
}

func TestEntCommercialStore_FreshEnvironment(t *testing.T) {
	client, _ := setupMigratedPostgres(t)
	store := NewEntCommercialStore(client)

	activation, err := store.ReadActivation(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(0), activation.Revision())
	for _, flag := range domain.CommercialFlags() {
		setting := activation.Setting(flag)
		assert.False(t, setting.Enabled, flag)
		assert.False(t, setting.Audience.AllUsers, flag)
		assert.Empty(t, setting.Audience.Entries, flag)
	}
	err = store.InTx(context.Background(), func(ctx context.Context, tx ports.CommercialTx) error {
		groups, err := tx.Groups().List(ctx)
		require.NoError(t, err)
		assert.Empty(t, groups)
		return nil
	})
	require.NoError(t, err)
}

func TestEntCommercialStore_ChangesCommitWithTheirEvidence(t *testing.T) {
	ctx := context.Background()
	env := commercialEnvironment(t, "test")
	adminID := uuid.NewString()
	bea, err := domain.ParseRolloutSubject("user:" + uuid.NewString())
	require.NoError(t, err)
	enabled := domain.FlagSetting{Enabled: true, Audience: domain.RolloutAudience{Entries: []domain.RolloutSubject{bea}}}

	t.Run("a change and its evidence commit together", func(t *testing.T) {
		client, _ := setupMigratedPostgres(t)
		store := NewEntCommercialStore(client)

		err := store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			if _, err := tx.Activation().LockExclusive(ctx); err != nil {
				return err
			}
			if err := tx.Activation().SaveFlag(ctx, "content_ownership", enabled, commercialAt, adminID); err != nil {
				return err
			}
			if _, err := tx.Activation().BumpRevision(ctx); err != nil {
				return err
			}
			return tx.Audit().Append(ctx, commercialEntry(t, commercialKey(t, env, adminID, uuid.NewString())))
		})
		require.NoError(t, err)

		activation, err := store.ReadActivation(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), activation.Revision())
		assert.True(t, activation.Allows("content_ownership", bea))
		last, err := store.Audit().LastOrderKey(ctx, env)
		require.NoError(t, err)
		assert.NotZero(t, last)

		require.NoError(t, store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			changes, err := tx.Activation().FlagChanges(ctx)
			require.NoError(t, err)
			assert.Equal(t, map[domain.FlagName]ports.FlagChange{"content_ownership": {At: changes["content_ownership"].At, By: adminID}}, changes,
				"only the changed flag has a change record")
			assert.True(t, commercialAt.Equal(changes["content_ownership"].At))
			return nil
		}))
	})

	t.Run("evidence that cannot be written leaves the change undone", func(t *testing.T) {
		client, _ := setupMigratedPostgres(t)
		store := NewEntCommercialStore(client)
		first := commercialEntry(t, commercialKey(t, env, adminID, uuid.NewString()))
		require.NoError(t, store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			return tx.Audit().Append(ctx, first)
		}))

		err := store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			if _, err := tx.Activation().LockExclusive(ctx); err != nil {
				return err
			}
			if err := tx.Activation().SaveFlag(ctx, "content_ownership", enabled, commercialAt, adminID); err != nil {
				return err
			}
			if _, err := tx.Activation().BumpRevision(ctx); err != nil {
				return err
			}
			// Reusing an entry id is refused by the database itself.
			duplicate := commercialEntry(t, commercialKey(t, env, adminID, uuid.NewString()))
			duplicate.AuditEntryID = first.AuditEntryID
			return tx.Audit().Append(ctx, duplicate)
		})
		require.Error(t, err)

		activation, err := store.ReadActivation(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), activation.Revision())
		assert.False(t, activation.Setting("content_ownership").Enabled)
	})

	t.Run("an error from the command leaves nothing changed", func(t *testing.T) {
		client, _ := setupMigratedPostgres(t)
		store := NewEntCommercialStore(client)
		refused := errors.New("refused")

		err := store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			if _, err := tx.Activation().LockExclusive(ctx); err != nil {
				return err
			}
			if _, err := tx.Activation().BumpRevision(ctx); err != nil {
				return err
			}
			return refused
		})
		require.ErrorIs(t, err, refused)

		activation, err := store.ReadActivation(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), activation.Revision())
	})
}

func TestEntCommercialStore_EvidenceIsAppendOnly(t *testing.T) {
	ctx := context.Background()
	client, db := setupMigratedPostgres(t)
	store := NewEntCommercialStore(client)
	entry := commercialEntry(t, commercialKey(t, commercialEnvironment(t, "test"), uuid.NewString(), uuid.NewString()))
	require.NoError(t, store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		return tx.Audit().Append(ctx, entry)
	}))

	for name, statement := range map[string]string{
		"update":   "UPDATE commercial_audit_entries SET reason = 'rewritten'",
		"delete":   "DELETE FROM commercial_audit_entries",
		"truncate": "TRUNCATE commercial_audit_entries",
	} {
		t.Run(name+" is refused", func(t *testing.T) {
			_, err := db.ExecContext(ctx, statement)

			require.ErrorContains(t, err, "append-only")
		})
	}

	var reason string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT reason FROM commercial_audit_entries").Scan(&reason))
	assert.Equal(t, "Foundation rehearsal", reason)
}

func TestEntCommercialStore_Operations(t *testing.T) {
	ctx := context.Background()
	env := commercialEnvironment(t, "test")
	adminID := uuid.NewString()

	operation := func(key domain.OperationKey) domain.CommercialOperation {
		return domain.CommercialOperation{
			Key: key, Fingerprint: "f1", Result: json.RawMessage(`{"revision":1}`), AuthorizedRevision: 0, CreatedAt: commercialAt,
		}
	}

	t.Run("a recorded operation is found by its scoped key", func(t *testing.T) {
		client, _ := setupMigratedPostgres(t)
		store := NewEntCommercialStore(client)
		key := commercialKey(t, env, adminID, uuid.NewString())

		require.NoError(t, store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			if err := tx.Operations().Insert(ctx, operation(key)); err != nil {
				return err
			}
			return tx.Operations().RecordAttempt(ctx, domain.OperationAttempt{
				AttemptID: uuid.NewString(), Key: key, Fingerprint: "f1", Outcome: domain.AuditOutcomeApplied, At: commercialAt,
			})
		}))

		require.NoError(t, store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			found, err := tx.Operations().Find(ctx, key)
			require.NoError(t, err)
			assert.Equal(t, "f1", found.Fingerprint)
			assert.JSONEq(t, `{"revision":1}`, string(found.Result))
			assert.Equal(t, key, found.Key)

			other := key
			other.ActorID = uuid.NewString()
			_, err = tx.Operations().Find(ctx, other)
			assert.ErrorIs(t, err, domain.ErrNotFound, "the same caller key from another actor is another operation")
			return nil
		}))
	})

	t.Run("concurrent submissions of one key record one operation", func(t *testing.T) {
		client, _ := setupMigratedPostgres(t)
		store := NewEntCommercialStore(client)
		key := commercialKey(t, env, adminID, uuid.NewString())

		const submissions = 5
		errs := make([]error, submissions)
		var wg sync.WaitGroup
		for i := range submissions {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
					return tx.Operations().Insert(ctx, operation(key))
				})
			}()
		}
		wg.Wait()

		succeeded := 0
		for _, err := range errs {
			if err == nil {
				succeeded++
				continue
			}
			assert.ErrorIs(t, err, domain.ErrConflict)
		}
		assert.Equal(t, 1, succeeded)
	})
}

func TestEntCommercialStore_ActivationFencing(t *testing.T) {
	ctx := context.Background()
	client, _ := setupMigratedPostgres(t)
	store := NewEntCommercialStore(client)

	changeHolding := make(chan struct{})
	releaseChange := make(chan struct{})
	changeDone := make(chan error, 1)
	go func() {
		changeDone <- store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			if _, err := tx.Activation().LockExclusive(ctx); err != nil {
				return err
			}
			if _, err := tx.Activation().BumpRevision(ctx); err != nil {
				return err
			}
			close(changeHolding)
			<-releaseChange
			return nil
		})
	}()
	<-changeHolding

	intentRevision := make(chan int64, 1)
	intentDone := make(chan error, 1)
	go func() {
		intentDone <- store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			revision, err := tx.Activation().LockShared(ctx)
			intentRevision <- revision
			return err
		})
	}()

	select {
	case <-intentRevision:
		t.Fatal("an intent was authorized while an activation change held the exclusive lock")
	case <-time.After(300 * time.Millisecond):
	}

	close(releaseChange)
	require.NoError(t, <-changeDone)
	require.NoError(t, <-intentDone)
	assert.Equal(t, int64(1), <-intentRevision, "the intent sees the committed change, never the revision before it")
}

func TestEntCommercialStore_AuditTraversal(t *testing.T) {
	ctx := context.Background()
	env := commercialEnvironment(t, "test")
	adminID := uuid.NewString()

	appendEntries := func(t *testing.T, store *EntCommercialStore, entries ...domain.AuditEntry) {
		t.Helper()
		for _, entry := range entries {
			require.NoError(t, store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
				return tx.Audit().Append(ctx, entry)
			}))
		}
	}

	t.Run("entries appended after the snapshot never appear in the traversal", func(t *testing.T) {
		client, _ := setupMigratedPostgres(t)
		store := NewEntCommercialStore(client)
		key := commercialKey(t, env, adminID, uuid.NewString())
		first, second, third := commercialEntry(t, key), commercialEntry(t, key), commercialEntry(t, key)
		appendEntries(t, store, first, second)
		filter := ports.AuditFilter{OperationID: key.OperationID()}

		snapshot, err := store.Audit().LastOrderKey(ctx, env)
		require.NoError(t, err)
		page1, err := store.Audit().List(ctx, env, filter, 0, snapshot, 1)
		require.NoError(t, err)
		require.Len(t, page1, 1)
		assert.Equal(t, first.AuditEntryID, page1[0].Entry.AuditEntryID)

		appendEntries(t, store, third)

		page2, err := store.Audit().List(ctx, env, filter, page1[0].OrderKey, snapshot, 10)
		require.NoError(t, err)
		require.Len(t, page2, 1)
		assert.Equal(t, second.AuditEntryID, page2[0].Entry.AuditEntryID)
	})

	t.Run("an entry round-trips unchanged", func(t *testing.T) {
		client, _ := setupMigratedPostgres(t)
		store := NewEntCommercialStore(client)
		key := commercialKey(t, env, adminID, uuid.NewString())
		entry := commercialEntry(t, key)
		entry.BeforeState = nil
		entry.CausedByEntryIDs = []string{uuid.NewString()}
		entry.ProviderReferences = []string{"evt_1"}
		appendEntries(t, store, entry)

		records, err := store.Audit().List(ctx, env, ports.AuditFilter{OperationID: key.OperationID()}, 0, 1<<62, 10)

		require.NoError(t, err)
		require.Len(t, records, 1)
		got := records[0].Entry
		assert.Nil(t, got.BeforeState, "no authoritative state stays null")
		assert.JSONEq(t, string(entry.AfterState), string(got.AfterState))
		got.AfterState = entry.AfterState
		assert.True(t, entry.OccurredAt.Equal(got.OccurredAt))
		got.OccurredAt, got.RecordedAt = entry.OccurredAt, entry.RecordedAt
		assert.Equal(t, entry, got)
	})

	t.Run("filters by resource and keeps environments apart", func(t *testing.T) {
		client, _ := setupMigratedPostgres(t)
		store := NewEntCommercialStore(client)
		staging := commercialEnvironment(t, "staging")
		mine := commercialKey(t, env, adminID, uuid.NewString())
		sameUUIDElsewhere := commercialKey(t, staging, adminID, mine.CallerKey)
		otherResource := commercialKey(t, env, adminID, uuid.NewString())
		otherResource.ResourceID = "private_beta"
		appendEntries(t, store, commercialEntry(t, mine), commercialEntry(t, sameUUIDElsewhere), commercialEntry(t, otherResource))

		byOperation, err := store.Audit().List(ctx, env, ports.AuditFilter{OperationID: mine.OperationID()}, 0, 1<<62, 10)
		require.NoError(t, err)
		assert.Len(t, byOperation, 1)

		byResource, err := store.Audit().List(ctx, env, ports.AuditFilter{ResourceKind: "commercial_flag", ResourceID: "private_beta"}, 0, 1<<62, 10)
		require.NoError(t, err)
		require.Len(t, byResource, 1)
		assert.Equal(t, "private_beta", byResource[0].Entry.Operation.ResourceID)
	})

	t.Run("appends are serialized so order keys commit in order", func(t *testing.T) {
		client, _ := setupMigratedPostgres(t)
		store := NewEntCommercialStore(client)

		holding := make(chan struct{})
		release := make(chan struct{})
		firstDone := make(chan error, 1)
		go func() {
			firstDone <- store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
				if err := tx.Audit().Append(ctx, commercialEntry(t, commercialKey(t, env, adminID, uuid.NewString()))); err != nil {
					return err
				}
				close(holding)
				<-release
				return nil
			})
		}()
		<-holding

		secondAppended := make(chan error, 1)
		go func() {
			secondAppended <- store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
				return tx.Audit().Append(ctx, commercialEntry(t, commercialKey(t, env, adminID, uuid.NewString())))
			})
		}()

		select {
		case <-secondAppended:
			t.Fatal("a second append committed while an earlier one was still open")
		case <-time.After(300 * time.Millisecond):
		}
		close(release)
		require.NoError(t, <-firstDone)
		require.NoError(t, <-secondAppended)
	})
}

func TestEntCommercialStore_Groups(t *testing.T) {
	ctx := context.Background()
	client, _ := setupMigratedPostgres(t)
	store := NewEntCommercialStore(client)
	adminID := uuid.NewString()
	bea := createCommercialUser(t, store, "clerk-bea")
	cleo := createCommercialUser(t, store, "clerk-cleo")

	require.NoError(t, store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		if err := tx.Groups().Save(ctx, "beta", "Private beta", commercialAt, adminID); err != nil {
			return err
		}
		if err := tx.Groups().SetMembers(ctx, "beta", []string{cleo, bea}, commercialAt, adminID); err != nil {
			return err
		}
		return tx.Groups().Save(ctx, "beta", "Closed beta", commercialAt.Add(time.Minute), adminID)
	}))

	require.NoError(t, store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		group, err := tx.Groups().Find(ctx, "beta")
		require.NoError(t, err)
		assert.Equal(t, "Closed beta", group.Name, "renamed")
		assert.ElementsMatch(t, []string{bea, cleo}, group.MemberIDs, "renaming keeps members")
		assert.True(t, commercialAt.Add(time.Minute).Equal(group.UpdatedAt))

		_, err = tx.Groups().Find(ctx, "staff")
		assert.ErrorIs(t, err, domain.ErrNotFound)

		missing, err := tx.Groups().MissingUsers(ctx, []string{bea, "00000000-0000-4000-8000-000000000000"})
		require.NoError(t, err)
		assert.Equal(t, []string{"00000000-0000-4000-8000-000000000000"}, missing)

		return tx.Groups().SetMembers(ctx, "beta", []string{cleo}, commercialAt, adminID)
	}))

	beta, err := domain.ParseRolloutSubject("group:beta")
	require.NoError(t, err)
	require.NoError(t, store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		return tx.Activation().SaveFlag(ctx, "private_beta", domain.FlagSetting{Enabled: true, Audience: domain.RolloutAudience{Entries: []domain.RolloutSubject{beta}}}, commercialAt, adminID)
	}))
	activation, err := store.ReadActivation(ctx)
	require.NoError(t, err)
	assert.True(t, activation.Allows("private_beta", domain.RolloutSubject{Kind: domain.RolloutSubjectUser, ID: cleo}))
	assert.False(t, activation.Allows("private_beta", domain.RolloutSubject{Kind: domain.RolloutSubjectUser, ID: bea}), "removed")
}
