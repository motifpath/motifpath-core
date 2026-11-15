package application_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

const (
	contentAdminID   = "0c1a0000-0000-4000-8000-0000000000a1"
	contentBeaID     = "0c1a0000-0000-4000-8000-0000000000b1"
	contentDanID     = "0c1a0000-0000-4000-8000-0000000000d1"
	contentPathID    = "0c1a0000-0000-4000-8000-0000000000c1"
	contentUnknownID = "0c1a0000-0000-4000-8000-0000000000ff"
)

var (
	contentAdmin   = domain.User{ID: contentAdminID, Role: domain.RoleAdmin}
	contentBea     = domain.User{ID: contentBeaID, Role: domain.RoleTeacher}
	contentDan     = domain.User{ID: contentDanID, Role: domain.RoleTeacher}
	contentStudent = domain.User{ID: "0c1a0000-0000-4000-8000-0000000000e1", Role: domain.RoleStudent}
)

type commercialContentFixture struct {
	store   *fakeCommercialStore
	service *application.CommercialContentService
}

func newCommercialContentFixture(t *testing.T) commercialContentFixture {
	t.Helper()
	store := newFakeCommercialStore()
	store.state.users[contentAdminID] = true
	store.state.users[contentBeaID] = true
	store.state.users[contentDanID] = true
	commands := newTestCommercialCommands(store)
	environment := commercialTestEnvironment(t)
	now := func() time.Time { return commercialNow }
	rollout := application.NewCommercialRolloutService(store, commands, environment, now)
	return commercialContentFixture{
		store:   store,
		service: application.NewCommercialContentService(store, commands, rollout, environment),
	}
}

// own stores content as created by author.
func (f commercialContentFixture) own(t *testing.T, kind domain.ContentKind, id string, author domain.User) {
	t.Helper()
	ref := domain.ContentRef{Kind: kind, ID: id}
	f.store.state.ownership[ref] = []domain.ContentOwnership{domain.NativeOwnership(ref, author, uuid.NewString())}
}

func (f commercialContentFixture) ownUnresolved(t *testing.T, kind domain.ContentKind, id string) {
	t.Helper()
	ref := domain.ContentRef{Kind: kind, ID: id}
	f.store.state.ownership[ref] = []domain.ContentOwnership{domain.LegacyOwnership(ref, nil, uuid.NewString())}
}

func (f commercialContentFixture) switchOwnershipCorrections(on bool) {
	f.store.state.flags[domain.FlagName("content_ownership")] = domain.FlagSetting{Enabled: on, Audience: domain.RolloutAudience{AllUsers: on}}
}

func (f commercialContentFixture) current(t *testing.T, id string) domain.ContentOwnership {
	t.Helper()
	history := f.store.state.ownership[domain.ContentRef{Kind: domain.ContentPath, ID: id}]
	require.NotEmpty(t, history)
	return history[len(history)-1]
}

func blockerCodes(content application.CommercialContent) []string {
	codes := make([]string, 0, len(content.SaleBlockers))
	for _, blocker := range content.SaleBlockers {
		codes = append(codes, blocker.Code)
	}
	return codes
}

func TestCommercialContentService_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("the owning teacher reads ownership with the commercial defaults", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.own(t, domain.ContentPath, contentPathID, contentBea)

		content, err := f.service.Get(ctx, contentBea, "path", contentPathID)

		require.NoError(t, err)
		assert.Equal(t, domain.OwnerTeacher, content.Ownership.OwnerKind)
		assert.Equal(t, contentBeaID, *content.Ownership.OwnerID)
		assert.Equal(t, 1, content.Revision)
		assert.Equal(t, application.SaleStateFree, content.SaleState)
		assert.ElementsMatch(t, []string{"configuration_missing", "new_sales_paused"}, blockerCodes(content))
	})

	t.Run("an admin reads any content", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.own(t, domain.ContentPath, contentPathID, contentBea)

		content, err := f.service.Get(ctx, contentAdmin, "path", contentPathID)

		require.NoError(t, err)
		assert.Equal(t, contentBeaID, *content.Ownership.OwnerID)
	})

	t.Run("unresolved content lists unresolved ownership as a blocker", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.ownUnresolved(t, domain.ContentExercise, contentPathID)

		content, err := f.service.Get(ctx, contentAdmin, "exercise", contentPathID)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"configuration_missing", "unresolved_ownership"}, blockerCodes(content))
	})

	t.Run("new sales are not paused once the applicable rollout allows them", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.own(t, domain.ContentPath, contentPathID, contentBea)
		for _, flag := range domain.CommercialFlags() {
			f.store.state.flags[flag] = domain.FlagSetting{Enabled: true, Audience: domain.RolloutAudience{AllUsers: true}}
		}

		content, err := f.service.Get(ctx, contentBea, "path", contentPathID)

		require.NoError(t, err)
		assert.Equal(t, []string{"configuration_missing"}, blockerCodes(content))
	})

	hidden := map[string]domain.User{"another teacher": contentDan, "a student": contentStudent}
	for name, caller := range hidden {
		t.Run(name+" cannot tell the content exists", func(t *testing.T) {
			f := newCommercialContentFixture(t)
			f.own(t, domain.ContentPath, contentPathID, contentBea)

			_, err := f.service.Get(ctx, caller, "path", contentPathID)

			assert.ErrorIs(t, err, domain.ErrNotFound)
		})
	}

	t.Run("content without an ownership record is not found", func(t *testing.T) {
		f := newCommercialContentFixture(t)

		_, err := f.service.Get(ctx, contentAdmin, "path", contentUnknownID)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("an unknown content kind is invalid", func(t *testing.T) {
		f := newCommercialContentFixture(t)

		_, err := f.service.Get(ctx, contentAdmin, "student_path", contentPathID)

		assert.ErrorIs(t, err, domain.ErrValidation)
	})
}

func TestCommercialContentService_CorrectOwnership(t *testing.T) {
	ctx := context.Background()
	toBea := application.OwnershipCorrectionRequest{ExpectedRevision: 1, OwnerKind: "teacher", OwnerID: contentBeaID, Reason: "Bea licensed this content to MotifPath"}

	t.Run("an admin corrects ownership; the creator stays and the evidence keeps both owners", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchOwnershipCorrections(true)
		f.own(t, domain.ContentPath, contentPathID, contentAdmin)
		key := uuid.NewString()

		receipt, err := f.service.CorrectOwnership(ctx, contentAdmin, "path", contentPathID, key, toBea)

		require.NoError(t, err)
		assert.Equal(t, key, receipt.OperationID)
		assert.Equal(t, contentPathID, receipt.ResourceID)
		assert.Equal(t, "applied", receipt.Status)
		got := f.current(t, contentPathID)
		assert.Equal(t, domain.OwnerTeacher, got.OwnerKind)
		assert.Equal(t, contentBeaID, *got.OwnerID)
		assert.Equal(t, contentAdminID, *got.CreatorID)
		assert.Equal(t, 2, got.Revision)
		assert.Equal(t, receipt.EvidenceEntryID, got.EvidenceEntryID)
		require.Len(t, f.store.state.ownership[domain.ContentRef{Kind: domain.ContentPath, ID: contentPathID}], 2, "revision 1 stays as history")

		applied := f.store.state.audit[len(f.store.state.audit)-1].Entry
		assert.Equal(t, application.ActionCorrectContentOwnership, applied.Operation.Action)
		var before, after map[string]any
		require.NoError(t, json.Unmarshal(applied.BeforeState, &before))
		require.NoError(t, json.Unmarshal(applied.AfterState, &after))
		assert.Equal(t, "platform", before["owner_kind"])
		assert.EqualValues(t, 1, before["revision"])
		assert.Equal(t, "teacher", after["owner_kind"])
		assert.EqualValues(t, 2, after["revision"])
	})

	t.Run("replaying the same correction returns its original result", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchOwnershipCorrections(true)
		f.own(t, domain.ContentPath, contentPathID, contentAdmin)
		key := uuid.NewString()
		first, err := f.service.CorrectOwnership(ctx, contentAdmin, "path", contentPathID, key, toBea)
		require.NoError(t, err)

		again, err := f.service.CorrectOwnership(ctx, contentAdmin, "path", contentPathID, key, toBea)

		require.NoError(t, err)
		assert.Equal(t, first, again)
		assert.Equal(t, 2, f.current(t, contentPathID).Revision)
	})

	refusals := map[string]struct {
		caller  domain.User
		flagOn  bool
		request application.OwnershipCorrectionRequest
		want    error
	}{
		"the owning teacher cannot transfer it": {
			caller: contentBea, flagOn: true, want: domain.ErrForbidden,
			request: application.OwnershipCorrectionRequest{ExpectedRevision: 1, OwnerKind: "teacher", OwnerID: contentDanID, Reason: "gift"},
		},
		"corrections are refused while the rollout is OFF": {
			caller: contentAdmin, flagOn: false, want: domain.ErrForbidden,
			request: application.OwnershipCorrectionRequest{ExpectedRevision: 1, OwnerKind: "platform", OwnerID: domain.CatalogProfileID, Reason: "catalog"},
		},
		"a stale ownership revision": {
			caller: contentAdmin, flagOn: true, want: domain.ErrConflict,
			request: application.OwnershipCorrectionRequest{ExpectedRevision: 2, OwnerKind: "platform", OwnerID: domain.CatalogProfileID, Reason: "catalog"},
		},
		"a teacher who is not a core user": {
			caller: contentAdmin, flagOn: true, want: domain.ErrValidation,
			request: application.OwnershipCorrectionRequest{ExpectedRevision: 1, OwnerKind: "teacher", OwnerID: contentUnknownID, Reason: "unknown"},
		},
		"no reason": {
			caller: contentAdmin, flagOn: true, want: domain.ErrValidation,
			request: application.OwnershipCorrectionRequest{ExpectedRevision: 1, OwnerKind: "platform", OwnerID: domain.CatalogProfileID},
		},
	}
	for name, tc := range refusals {
		t.Run("refuses "+name+" and changes nothing", func(t *testing.T) {
			f := newCommercialContentFixture(t)
			f.switchOwnershipCorrections(tc.flagOn)
			f.own(t, domain.ContentPath, contentPathID, contentBea)

			_, err := f.service.CorrectOwnership(ctx, tc.caller, "path", contentPathID, uuid.NewString(), tc.request)

			assert.ErrorIs(t, err, tc.want)
			got := f.current(t, contentPathID)
			assert.Equal(t, contentBeaID, *got.OwnerID)
			assert.Equal(t, 1, got.Revision)
		})
	}

	t.Run("correcting content that has no ownership record is not found", func(t *testing.T) {
		f := newCommercialContentFixture(t)
		f.switchOwnershipCorrections(true)

		_, err := f.service.CorrectOwnership(ctx, contentAdmin, "path", contentUnknownID, uuid.NewString(), toBea)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
