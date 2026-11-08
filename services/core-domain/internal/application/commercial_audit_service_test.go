package application_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

var (
	auditAdmin    = domain.User{ID: commercialAdminID, Role: domain.RoleAdmin}
	auditOther    = domain.User{ID: "a0000000-0000-4000-8000-000000000002", Role: domain.RoleAdmin}
	auditCustomer = domain.User{ID: "b0000000-0000-4000-8000-000000000001", Role: domain.RoleStudent}
)

// appendAudit stores evidence for a resource directly, as earlier commands would have.
func appendAudit(t *testing.T, store *fakeCommercialStore, resourceID, callerKey string) domain.AuditEntry {
	t.Helper()
	intent := testIntent(t, `{}`)
	intent.Key.ResourceID = resourceID
	intent.Key.CallerKey = callerKey
	entry := domain.AuditEntry{
		AuditEntryID: uuid.NewString(), Operation: intent.Key, AttemptID: uuid.NewString(),
		OccurredAt: commercialNow, RecordedAt: commercialNow, Reason: "rehearsal", Outcome: domain.AuditOutcomeApplied,
		PolicyRevision: domain.CommercialRegistryRevision, ConfigurationRevision: "1", CorrelationID: uuid.NewString(),
		CausedByEntryIDs: []string{}, ProviderReferences: []string{},
	}
	require.NoError(t, store.InTx(context.Background(), func(ctx context.Context, tx ports.CommercialTx) error {
		return tx.Audit().Append(ctx, entry)
	}))
	return entry
}

func newTestAuditService(t *testing.T, store *fakeCommercialStore) *application.CommercialAuditService {
	t.Helper()
	return application.NewCommercialAuditService(store, commercialTestEnvironment(t), []byte("cursor-secret"),
		func() time.Time { return commercialNow }, uuid.NewString)
}

func flagHistory(flag string) application.AuditQuery {
	return application.AuditQuery{ResourceKind: "commercial_flag", ResourceID: flag}
}

func entryIDs(entries []domain.AuditEntry) []string {
	ids := make([]string, len(entries))
	for i, entry := range entries {
		ids[i] = entry.AuditEntryID
	}
	return ids
}

func TestCommercialAuditService_List(t *testing.T) {
	ctx := context.Background()

	t.Run("an administrator reads a resource's evidence in append order", func(t *testing.T) {
		store := newFakeCommercialStore()
		first := appendAudit(t, store, "content_ownership", uuid.NewString())
		appendAudit(t, store, "private_beta", uuid.NewString())
		second := appendAudit(t, store, "content_ownership", uuid.NewString())

		page, err := newTestAuditService(t, store).List(ctx, auditAdmin, flagHistory("content_ownership"))

		require.NoError(t, err)
		assert.Equal(t, []string{first.AuditEntryID, second.AuditEntryID}, entryIDs(page.Items))
		assert.Nil(t, page.NextCursor)
	})

	t.Run("a customer is refused without disclosing matching records", func(t *testing.T) {
		store := newFakeCommercialStore()
		appendAudit(t, store, "content_ownership", uuid.NewString())

		page, err := newTestAuditService(t, store).List(ctx, auditCustomer, flagHistory("content_ownership"))

		require.ErrorIs(t, err, domain.ErrForbidden)
		assert.Empty(t, page.Items)
	})

	t.Run("appends do not shift a traversal, and later entries need a new one", func(t *testing.T) {
		store := newFakeCommercialStore()
		service := newTestAuditService(t, store)
		a := appendAudit(t, store, "content_ownership", uuid.NewString())
		b := appendAudit(t, store, "content_ownership", uuid.NewString())
		c := appendAudit(t, store, "content_ownership", uuid.NewString())
		query := flagHistory("content_ownership")
		query.Limit = 2

		page1, err := service.List(ctx, auditAdmin, query)
		require.NoError(t, err)
		require.NotNil(t, page1.NextCursor)
		late := appendAudit(t, store, "content_ownership", uuid.NewString())

		query.Cursor = *page1.NextCursor
		page2, err := service.List(ctx, auditAdmin, query)

		require.NoError(t, err)
		assert.Equal(t, []string{a.AuditEntryID, b.AuditEntryID}, entryIDs(page1.Items))
		assert.Equal(t, []string{c.AuditEntryID}, entryIDs(page2.Items))
		assert.Nil(t, page2.NextCursor)

		fresh, err := service.List(ctx, auditAdmin, flagHistory("content_ownership"))
		require.NoError(t, err)
		assert.Contains(t, entryIDs(fresh.Items), late.AuditEntryID)
	})

	t.Run("a read creates access evidence outside its own snapshot", func(t *testing.T) {
		store := newFakeCommercialStore()
		appendAudit(t, store, "content_ownership", uuid.NewString())
		service := newTestAuditService(t, store)

		page, err := service.List(ctx, auditAdmin, flagHistory("content_ownership"))
		require.NoError(t, err)

		require.Len(t, store.state.audit, 2)
		access := store.state.audit[1].Entry
		assert.Equal(t, "commercial_audit.read", access.Operation.Action)
		assert.Equal(t, commercialAdminID, access.Operation.ActorID)
		var scope map[string]string
		require.NoError(t, json.Unmarshal(access.AfterState, &scope))
		assert.Equal(t, "commercial_flag", scope["resource_kind"])
		assert.Equal(t, "content_ownership", scope["resource_id"])
		assert.NotContains(t, entryIDs(page.Items), access.AuditEntryID)
	})

	t.Run("an empty history is a completed traversal", func(t *testing.T) {
		page, err := newTestAuditService(t, newFakeCommercialStore()).List(ctx, auditAdmin, flagHistory("plus_checkout"))

		require.NoError(t, err)
		assert.Empty(t, page.Items)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("unavailable evidence is never an empty history", func(t *testing.T) {
		store := newFakeCommercialStore()
		store.storageFails = true

		page, err := newTestAuditService(t, store).List(ctx, auditAdmin, flagHistory("content_ownership"))

		require.ErrorIs(t, err, domain.ErrUnavailable)
		assert.Nil(t, page.Items)
	})

	t.Run("a read that cannot be evidenced is unavailable", func(t *testing.T) {
		store := newFakeCommercialStore()
		appendAudit(t, store, "content_ownership", uuid.NewString())
		store.auditFails = true

		_, err := newTestAuditService(t, store).List(ctx, auditAdmin, flagHistory("content_ownership"))

		require.ErrorIs(t, err, domain.ErrUnavailable)
	})

	t.Run("reused UUIDs stay distinguishable by their namespace", func(t *testing.T) {
		store := newFakeCommercialStore()
		shared := uuid.NewString()
		appendAudit(t, store, "content_ownership", shared)
		appendAudit(t, store, "private_beta", shared)

		page, err := newTestAuditService(t, store).List(ctx, auditAdmin, application.AuditQuery{OperationID: shared})

		require.NoError(t, err)
		require.Len(t, page.Items, 2)
		assert.NotEqual(t, page.Items[0].Operation, page.Items[1].Operation)
		assert.NotEqual(t, page.Items[0].AuditEntryID, page.Items[1].AuditEntryID)
	})

	t.Run("operation and resource filters combine with AND", func(t *testing.T) {
		store := newFakeCommercialStore()
		shared := uuid.NewString()
		appendAudit(t, store, "content_ownership", shared)
		wanted := appendAudit(t, store, "private_beta", shared)

		page, err := newTestAuditService(t, store).List(ctx, auditAdmin, application.AuditQuery{
			OperationID: shared, ResourceKind: "commercial_flag", ResourceID: "private_beta",
		})

		require.NoError(t, err)
		assert.Equal(t, []string{wanted.AuditEntryID}, entryIDs(page.Items))
	})

	continuation := func(t *testing.T, store *fakeCommercialStore) (application.AuditQuery, string) {
		t.Helper()
		for range 3 {
			appendAudit(t, store, "content_ownership", uuid.NewString())
		}
		query := flagHistory("content_ownership")
		query.Limit = 1
		page, err := newTestAuditService(t, store).List(ctx, auditAdmin, query)
		require.NoError(t, err)
		require.NotNil(t, page.NextCursor)
		return query, *page.NextCursor
	}

	for name, misuse := range map[string]func(query *application.AuditQuery, cursor string) domain.User{
		"another resource": func(q *application.AuditQuery, c string) domain.User {
			q.ResourceID, q.Cursor = "private_beta", c
			return auditAdmin
		},
		"another limit": func(q *application.AuditQuery, c string) domain.User {
			q.Limit, q.Cursor = 5, c
			return auditAdmin
		},
		"another administrator": func(q *application.AuditQuery, c string) domain.User {
			q.Cursor = c
			return auditOther
		},
		"a tampered cursor": func(q *application.AuditQuery, c string) domain.User {
			q.Cursor = strings.Replace(c, c[:1], map[bool]string{true: "B", false: "A"}[c[:1] == "A"], 1)
			return auditAdmin
		},
		"garbage": func(q *application.AuditQuery, _ string) domain.User {
			q.Cursor = "not-a-cursor"
			return auditAdmin
		},
	} {
		t.Run("a continuation used with "+name+" is refused", func(t *testing.T) {
			store := newFakeCommercialStore()
			query, cursor := continuation(t, store)
			caller := misuse(&query, cursor)

			_, err := newTestAuditService(t, store).List(ctx, caller, query)

			require.ErrorIs(t, err, domain.ErrValidation)
		})
	}

	t.Run("a continuation from another environment is refused", func(t *testing.T) {
		store := newFakeCommercialStore()
		query, cursor := continuation(t, store)
		staging, err := domain.ParseEnvironment("staging")
		require.NoError(t, err)
		elsewhere := application.NewCommercialAuditService(store, staging, []byte("cursor-secret"), func() time.Time { return commercialNow }, uuid.NewString)
		query.Cursor = cursor

		_, err = elsewhere.List(ctx, auditAdmin, query)

		require.ErrorIs(t, err, domain.ErrValidation)
	})

	for name, query := range map[string]application.AuditQuery{
		"no filter":                   {},
		"a resource kind without id":  {ResourceKind: "commercial_flag"},
		"a resource id without kind":  {ResourceID: "content_ownership"},
		"an operation id not a UUID":  {OperationID: "op-1"},
		"a limit above 100":           {OperationID: commercialKeyK, Limit: 101},
		"a negative limit":            {OperationID: commercialKeyK, Limit: -1},
		"a resource kind over 100":    {ResourceKind: strings.Repeat("k", 101), ResourceID: "x"},
		"a resource id over 255":      {ResourceKind: "k", ResourceID: strings.Repeat("r", 256)},
	} {
		t.Run("refused: "+name, func(t *testing.T) {
			_, err := newTestAuditService(t, newFakeCommercialStore()).List(ctx, auditAdmin, query)

			require.ErrorIs(t, err, domain.ErrValidation)
		})
	}
}
