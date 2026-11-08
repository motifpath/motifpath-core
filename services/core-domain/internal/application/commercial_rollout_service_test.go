package application_test

import (
	"context"
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

const (
	rolloutBeaID  = "b0000000-0000-4000-8000-0000000000be"
	rolloutCleoID = "b0000000-0000-4000-8000-0000000000c1"
)

var (
	rolloutAdmin   = domain.User{ID: commercialAdminID, Role: domain.RoleAdmin}
	rolloutTeacher = domain.User{ID: rolloutBeaID, Role: domain.RoleTeacher}
	userBea        = domain.RolloutSubject{Kind: domain.RolloutSubjectUser, ID: rolloutBeaID}
	userCleo       = domain.RolloutSubject{Kind: domain.RolloutSubjectUser, ID: rolloutCleoID}
)

type rolloutFixture struct {
	store   *fakeCommercialStore
	service *application.CommercialRolloutService
}

func newRolloutFixture(t *testing.T) rolloutFixture {
	t.Helper()
	store := newFakeCommercialStore()
	store.state.users[rolloutBeaID] = true
	store.state.users[rolloutCleoID] = true
	now := func() time.Time { return commercialNow }
	commands := application.NewCommercialCommands(store, now, uuid.NewString)
	return rolloutFixture{
		store:   store,
		service: application.NewCommercialRolloutService(store, commands, commercialTestEnvironment(t), now),
	}
}

func (f rolloutFixture) setFlag(t *testing.T, flag string, change application.FlagChangeRequest) application.RolloutReceipt {
	t.Helper()
	change.ExpectedRevision = f.store.state.revision
	if change.Reason == "" {
		change.Reason = "rehearsal"
	}
	if change.SubjectIDs == nil {
		change.SubjectIDs = []string{}
	}
	receipt, err := f.service.ChangeFlag(context.Background(), rolloutAdmin, flag, uuid.NewString(), change)
	require.NoError(t, err)
	return receipt
}

func (f rolloutFixture) saveGroup(t *testing.T, slug string, members ...string) {
	t.Helper()
	ctx := context.Background()
	_, err := f.service.SaveGroup(ctx, rolloutAdmin, slug, uuid.NewString(), application.GroupChangeRequest{Name: slug, Reason: "cohort"})
	require.NoError(t, err)
	if len(members) > 0 {
		_, err = f.service.ChangeMembers(ctx, rolloutAdmin, slug, uuid.NewString(), application.MembershipChangeRequest{Add: members, Remove: []string{}, Reason: "invite"})
		require.NoError(t, err)
	}
}

func (f rolloutFixture) allows(t *testing.T, flag domain.FlagName, subject domain.RolloutSubject) bool {
	t.Helper()
	allowed, err := f.service.Allowed(context.Background(), flag, subject)
	require.NoError(t, err)
	return allowed
}

func TestCommercialRolloutService_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("every flag starts OFF with an empty allowlist", func(t *testing.T) {
		f := newRolloutFixture(t)

		rollout, err := f.service.Get(ctx, rolloutAdmin)

		require.NoError(t, err)
		assert.Equal(t, int64(0), rollout.Revision)
		require.Len(t, rollout.Flags, 17)
		for i, flag := range rollout.Flags {
			assert.Equal(t, domain.CommercialFlags()[i], flag.Flag, "registry order")
			assert.False(t, flag.Enabled)
			assert.False(t, flag.Effective)
			assert.False(t, flag.AllUsers)
			assert.Empty(t, flag.SubjectIDs)
			assert.Nil(t, flag.UpdatedAt)
			assert.Nil(t, flag.UpdatedBy)
		}
		assert.Equal(t, []domain.FlagName{"content_ownership"}, rollout.Flags[1].Prerequisites)
	})

	t.Run("a non-admin cannot read it", func(t *testing.T) {
		_, err := newRolloutFixture(t).service.Get(ctx, rolloutTeacher)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("unreadable storage is an error, not OFF", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.store.storageFails = true

		_, err := f.service.Get(ctx, rolloutAdmin)

		require.ErrorIs(t, err, domain.ErrUnavailable)
	})

	t.Run("an enabled flag whose prerequisite is OFF shows as not effective", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.setFlag(t, "content_versions", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"user:" + rolloutBeaID}})

		rollout, err := f.service.Get(ctx, rolloutAdmin)

		require.NoError(t, err)
		versions := rollout.Flags[1]
		assert.True(t, versions.Enabled)
		assert.False(t, versions.Effective)
		require.NotNil(t, versions.UpdatedBy)
		assert.Equal(t, commercialAdminID, *versions.UpdatedBy)
		assert.False(t, rollout.Flags[0].Enabled, "enabling a flag does not enable its prerequisites")
	})
}

func TestCommercialRolloutService_ChangeFlag(t *testing.T) {
	ctx := context.Background()
	enableForBea := application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"user:" + rolloutBeaID}, Reason: "Foundation rehearsal"}

	t.Run("an admin enables a flag for one subject, with evidence", func(t *testing.T) {
		f := newRolloutFixture(t)

		receipt, err := f.service.ChangeFlag(ctx, rolloutAdmin, "content_ownership", commercialKeyK, enableForBea)

		require.NoError(t, err)
		assert.Equal(t, int64(1), receipt.Revision)
		assert.Equal(t, commercialKeyK, receipt.OperationID)
		assert.Equal(t, "applied", receipt.Status)
		assert.True(t, f.allows(t, "content_ownership", userBea))
		assert.False(t, f.allows(t, "content_ownership", userCleo))

		require.Len(t, f.store.state.audit, 1)
		evidence := f.store.state.audit[0].Entry
		assert.Equal(t, receipt.EvidenceEntryID, evidence.AuditEntryID)
		assert.Equal(t, "commercial_rollout.flag_changed", evidence.Operation.Action)
		assert.Equal(t, commercialAdminID, evidence.Operation.ActorID)
		assert.Equal(t, "Foundation rehearsal", evidence.Reason)
		assert.Equal(t, domain.CommercialRegistryRevision, evidence.PolicyRevision)
		assert.Equal(t, "1", evidence.ConfigurationRevision)
		assert.JSONEq(t, `{"enabled":false,"all_users":false,"subject_ids":[]}`, string(evidence.BeforeState))
		assert.JSONEq(t, `{"enabled":true,"all_users":false,"subject_ids":["user:`+rolloutBeaID+`"]}`, string(evidence.AfterState))
	})

	t.Run("a stale revision is refused", func(t *testing.T) {
		f := newRolloutFixture(t)
		for range 3 {
			f.setFlag(t, "private_beta", application.FlagChangeRequest{})
		}
		stale := enableForBea
		stale.ExpectedRevision = 2

		_, err := f.service.ChangeFlag(ctx, rolloutAdmin, "content_ownership", uuid.NewString(), stale)

		require.ErrorIs(t, err, domain.ErrConflict)
		assert.Equal(t, int64(3), f.store.state.revision)
	})

	t.Run("replaying the same change returns the original receipt", func(t *testing.T) {
		f := newRolloutFixture(t)
		first, err := f.service.ChangeFlag(ctx, rolloutAdmin, "content_ownership", commercialKeyK, enableForBea)
		require.NoError(t, err)

		replay, err := f.service.ChangeFlag(ctx, rolloutAdmin, "content_ownership", commercialKeyK, enableForBea)

		require.NoError(t, err)
		assert.Equal(t, first, replay)
		assert.Equal(t, int64(1), f.store.state.revision)
	})

	t.Run("reusing a key for a different change is refused", func(t *testing.T) {
		f := newRolloutFixture(t)
		_, err := f.service.ChangeFlag(ctx, rolloutAdmin, "content_ownership", commercialKeyK, enableForBea)
		require.NoError(t, err)

		_, err = f.service.ChangeFlag(ctx, rolloutAdmin, "content_ownership", commercialKeyK,
			application.FlagChangeRequest{ExpectedRevision: 1, Enabled: false, SubjectIDs: []string{}, Reason: "off"})

		require.ErrorIs(t, err, domain.ErrConflict)
		assert.True(t, f.store.state.flags["content_ownership"].Enabled)
	})

	t.Run("a non-admin cannot change a flag", func(t *testing.T) {
		f := newRolloutFixture(t)

		_, err := f.service.ChangeFlag(ctx, rolloutTeacher, "content_ownership", uuid.NewString(), enableForBea)

		require.ErrorIs(t, err, domain.ErrForbidden)
		assert.Equal(t, int64(0), f.store.state.revision)
	})

	t.Run("an unregistered flag is not found", func(t *testing.T) {
		_, err := newRolloutFixture(t).service.ChangeFlag(ctx, rolloutAdmin, "free_money", uuid.NewString(), application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{}, Reason: "x"})

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("audit failure prevents the change", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.store.auditFails = true

		_, err := f.service.ChangeFlag(ctx, rolloutAdmin, "content_ownership", uuid.NewString(), enableForBea)

		require.ErrorIs(t, err, domain.ErrUnavailable)
		assert.Equal(t, int64(0), f.store.state.revision)
		assert.False(t, f.store.state.flags["content_ownership"].Enabled)
	})

	t.Run("all users covers every user and only listed services", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.setFlag(t, "content_ownership", application.FlagChangeRequest{Enabled: true, AllUsers: true, SubjectIDs: []string{"service:core-domain"}})

		assert.True(t, f.allows(t, "content_ownership", userBea))
		assert.True(t, f.allows(t, "content_ownership", userCleo))
		assert.True(t, f.allows(t, "content_ownership", domain.RolloutSubject{Kind: domain.RolloutSubjectService, ID: "core-domain"}))
		assert.False(t, f.allows(t, "content_ownership", domain.RolloutSubject{Kind: domain.RolloutSubjectService, ID: "strapi-sync"}))
	})

	t.Run("a change to the current values is still a recorded change", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.setFlag(t, "private_beta", application.FlagChangeRequest{})

		assert.Equal(t, int64(1), f.store.state.revision)
		assert.Len(t, f.store.state.audit, 1)
	})

	invalid := map[string]application.FlagChangeRequest{
		"a group that does not exist": {Enabled: true, SubjectIDs: []string{"group:staff"}, Reason: "x"},
		"a malformed subject":         {Enabled: true, SubjectIDs: []string{"user:bea"}, Reason: "x"},
		"a repeated subject":          {Enabled: true, SubjectIDs: []string{"service:a", "service:a"}, Reason: "x"},
		"no reason":                   {Enabled: true, SubjectIDs: []string{}},
		"a reason over 1000":          {Enabled: true, SubjectIDs: []string{}, Reason: strings.Repeat("r", 1001)},
		"more than 500 subjects":      {Enabled: true, SubjectIDs: manyServices(501), Reason: "x"},
	}
	for name, change := range invalid {
		t.Run("refused: "+name, func(t *testing.T) {
			f := newRolloutFixture(t)

			_, err := f.service.ChangeFlag(ctx, rolloutAdmin, "plus_checkout", uuid.NewString(), change)

			require.ErrorIs(t, err, domain.ErrValidation)
			assert.Equal(t, int64(0), f.store.state.revision)
		})
	}
}

func manyServices(n int) []string {
	subjects := make([]string, n)
	for i := range subjects {
		subjects[i] = "service:s" + strings.Repeat("x", i%60) + string(rune('a'+i%26)) + "-" + string(rune('a'+(i/26)%26))
	}
	return subjects
}

func TestCommercialRolloutService_KillSwitch(t *testing.T) {
	ctx := context.Background()

	t.Run("turns every flag OFF and keeps audiences", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.saveGroup(t, "beta")
		f.setFlag(t, "content_ownership", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"user:" + rolloutBeaID}})
		f.setFlag(t, "plus_catalog", application.FlagChangeRequest{Enabled: true, AllUsers: true})
		f.setFlag(t, "private_beta", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"group:beta"}})
		before := f.store.state.revision

		receipt, err := f.service.KillSwitch(ctx, rolloutAdmin, uuid.NewString(), "Incident")

		require.NoError(t, err)
		assert.Equal(t, before+1, receipt.Revision)
		for flag, setting := range f.store.state.flags {
			assert.False(t, setting.Enabled, flag)
		}
		assert.Equal(t, "user:"+rolloutBeaID, f.store.state.flags["content_ownership"].Audience.Entries[0].String())
		assert.True(t, f.store.state.flags["plus_catalog"].Audience.AllUsers)
		assert.Equal(t, "group:beta", f.store.state.flags["private_beta"].Audience.Entries[0].String())
		assert.False(t, f.allows(t, "content_ownership", userBea))
		assert.Equal(t, "commercial_rollout.kill_switch", f.store.state.audit[len(f.store.state.audit)-1].Entry.Operation.Action)
	})

	t.Run("works from a stale view", func(t *testing.T) {
		f := newRolloutFixture(t)
		for range 4 {
			f.setFlag(t, "private_beta", application.FlagChangeRequest{})
		}

		receipt, err := f.service.KillSwitch(ctx, rolloutAdmin, uuid.NewString(), "Incident")

		require.NoError(t, err)
		assert.Equal(t, int64(5), receipt.Revision)
	})

	t.Run("a non-admin cannot use it", func(t *testing.T) {
		_, err := newRolloutFixture(t).service.KillSwitch(ctx, rolloutTeacher, uuid.NewString(), "Incident")

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestCommercialRolloutService_Groups(t *testing.T) {
	ctx := context.Background()

	t.Run("an admin creates a group with no members", func(t *testing.T) {
		f := newRolloutFixture(t)

		receipt, err := f.service.SaveGroup(ctx, rolloutAdmin, "beta", uuid.NewString(), application.GroupChangeRequest{Name: "Private beta", Reason: "Beta cohort"})

		require.NoError(t, err)
		assert.Equal(t, int64(1), receipt.Revision)
		groups, err := f.service.ListGroups(ctx, rolloutAdmin)
		require.NoError(t, err)
		assert.Equal(t, int64(1), groups.Revision)
		require.Len(t, groups.Groups, 1)
		assert.Equal(t, "Private beta", groups.Groups[0].Name)
		assert.Empty(t, groups.Groups[0].MemberIDs)
	})

	t.Run("renaming keeps members", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.saveGroup(t, "beta", rolloutBeaID, rolloutCleoID)

		_, err := f.service.SaveGroup(ctx, rolloutAdmin, "beta", uuid.NewString(), application.GroupChangeRequest{Name: "Closed beta", Reason: "rename"})

		require.NoError(t, err)
		group := f.store.state.groups["beta"]
		assert.Equal(t, "Closed beta", group.Name)
		assert.ElementsMatch(t, []string{rolloutBeaID, rolloutCleoID}, group.MemberIDs)
	})

	t.Run("a group entry allows its members and no one else", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.saveGroup(t, "beta", rolloutCleoID)
		f.setFlag(t, "private_beta", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"group:beta"}})

		assert.True(t, f.allows(t, "private_beta", userCleo))
		assert.False(t, f.allows(t, "private_beta", userBea))
	})

	t.Run("adding a member admits them to every flag that lists the group", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.saveGroup(t, "friends-and-family")
		f.setFlag(t, "private_beta", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"group:friends-and-family"}})
		f.setFlag(t, "content_ownership", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"group:friends-and-family"}})
		before := f.store.state.revision

		receipt, err := f.service.ChangeMembers(ctx, rolloutAdmin, "friends-and-family", uuid.NewString(),
			application.MembershipChangeRequest{Add: []string{rolloutCleoID}, Remove: []string{}, Reason: "Invited"})

		require.NoError(t, err)
		assert.Equal(t, before+1, receipt.Revision)
		assert.True(t, f.allows(t, "private_beta", userCleo))
		assert.True(t, f.allows(t, "content_ownership", userCleo))
		evidence := f.store.state.audit[len(f.store.state.audit)-1].Entry
		assert.Equal(t, "commercial_rollout.group_members_changed", evidence.Operation.Action)
		assert.JSONEq(t, `{"member_ids":["`+rolloutCleoID+`"],"added":["`+rolloutCleoID+`"],"removed":[]}`, string(evidence.AfterState))
	})

	t.Run("adding a current member or removing a non-member is not an error", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.saveGroup(t, "beta", rolloutCleoID)

		_, err := f.service.ChangeMembers(ctx, rolloutAdmin, "beta", uuid.NewString(),
			application.MembershipChangeRequest{Add: []string{rolloutCleoID}, Remove: []string{rolloutBeaID}, Reason: "x"})

		require.NoError(t, err)
		assert.Equal(t, []string{rolloutCleoID}, f.store.state.groups["beta"].MemberIDs)
	})

	for name, tc := range map[string]struct {
		slug   string
		change application.MembershipChangeRequest
		want   error
	}{
		"an unknown user":               {"beta", application.MembershipChangeRequest{Add: []string{"b0000000-0000-4000-8000-000000000999"}, Remove: []string{}, Reason: "x"}, domain.ErrValidation},
		"one user added and removed":    {"beta", application.MembershipChangeRequest{Add: []string{rolloutCleoID}, Remove: []string{rolloutCleoID}, Reason: "x"}, domain.ErrValidation},
		"an unknown group":              {"staff", application.MembershipChangeRequest{Add: []string{rolloutCleoID}, Remove: []string{}, Reason: "x"}, domain.ErrNotFound},
		"nothing to add or remove":      {"beta", application.MembershipChangeRequest{Add: []string{}, Remove: []string{}, Reason: "x"}, domain.ErrValidation},
		"a membership change no reason": {"beta", application.MembershipChangeRequest{Add: []string{rolloutCleoID}, Remove: []string{}}, domain.ErrValidation},
	} {
		t.Run("membership refused: "+name, func(t *testing.T) {
			f := newRolloutFixture(t)
			f.saveGroup(t, "beta")
			before := f.store.state.revision

			_, err := f.service.ChangeMembers(ctx, rolloutAdmin, tc.slug, uuid.NewString(), tc.change)

			require.ErrorIs(t, err, tc.want)
			assert.Equal(t, before, f.store.state.revision)
			assert.Empty(t, f.store.state.groups["beta"].MemberIDs)
		})
	}

	t.Run("a malformed membership change is refused whether or not the group exists", func(t *testing.T) {
		f := newRolloutFixture(t)

		_, err := f.service.ChangeMembers(ctx, rolloutAdmin, "staff", uuid.NewString(),
			application.MembershipChangeRequest{Add: []string{"b0000000-0000-4000-8000-000000000999"}, Remove: []string{}, Reason: "x"})

		require.ErrorIs(t, err, domain.ErrValidation)
	})

	t.Run("invalid group slugs and names are refused", func(t *testing.T) {
		f := newRolloutFixture(t)
		for slug, name := range map[string]string{"Beta": "Beta", "beta": "", "staff": strings.Repeat("n", 121)} {
			_, err := f.service.SaveGroup(ctx, rolloutAdmin, slug, uuid.NewString(), application.GroupChangeRequest{Name: name, Reason: "x"})
			assert.ErrorIs(t, err, domain.ErrValidation, slug)
		}
	})

	t.Run("a non-admin cannot change or list groups", func(t *testing.T) {
		f := newRolloutFixture(t)

		_, err := f.service.SaveGroup(ctx, rolloutTeacher, "beta", uuid.NewString(), application.GroupChangeRequest{Name: "Mine", Reason: "x"})
		require.ErrorIs(t, err, domain.ErrForbidden)
		_, err = f.service.ChangeMembers(ctx, rolloutTeacher, "beta", uuid.NewString(), application.MembershipChangeRequest{Add: []string{rolloutBeaID}, Remove: []string{}, Reason: "x"})
		require.ErrorIs(t, err, domain.ErrForbidden)
		_, err = f.service.ListGroups(ctx, rolloutTeacher)
		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("audit failure prevents a membership change", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.saveGroup(t, "beta")
		f.store.auditFails = true

		_, err := f.service.ChangeMembers(ctx, rolloutAdmin, "beta", uuid.NewString(), application.MembershipChangeRequest{Add: []string{rolloutCleoID}, Remove: []string{}, Reason: "x"})

		require.ErrorIs(t, err, domain.ErrUnavailable)
		assert.Empty(t, f.store.state.groups["beta"].MemberIDs)
	})
}

func TestCommercialRolloutService_Intents(t *testing.T) {
	ctx := context.Background()

	authorize := func(f rolloutFixture, flag domain.FlagName, subject domain.RolloutSubject) (int64, error) {
		var revision int64
		err := f.store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			var err error
			revision, err = f.service.AuthorizeIntent(ctx, tx, flag, subject)
			return err
		})
		return revision, err
	}

	t.Run("an allowed intent records the revision it was authorized under", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.setFlag(t, "content_ownership", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"user:" + rolloutBeaID}})

		revision, err := authorize(f, "content_ownership", userBea)

		require.NoError(t, err)
		assert.Equal(t, int64(1), revision)
	})

	t.Run("an intent started after OFF commits is refused", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.setFlag(t, "content_ownership", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"user:" + rolloutBeaID}})
		_, err := f.service.KillSwitch(ctx, rolloutAdmin, uuid.NewString(), "Incident")
		require.NoError(t, err)

		_, err = authorize(f, "content_ownership", userBea)

		require.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("removing a member fences their new intents", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.saveGroup(t, "beta", rolloutCleoID)
		f.setFlag(t, "private_beta", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"group:beta"}})
		earlier, err := authorize(f, "private_beta", userCleo)
		require.NoError(t, err)

		_, err = f.service.ChangeMembers(ctx, rolloutAdmin, "beta", uuid.NewString(), application.MembershipChangeRequest{Add: []string{}, Remove: []string{rolloutCleoID}, Reason: "x"})
		require.NoError(t, err)

		_, err = authorize(f, "private_beta", userCleo)
		require.ErrorIs(t, err, domain.ErrForbidden)
		assert.Equal(t, int64(3), earlier, "the earlier intent keeps the revision it recorded")
	})

	t.Run("an unreadable activation decision denies", func(t *testing.T) {
		f := newRolloutFixture(t)
		f.setFlag(t, "content_ownership", application.FlagChangeRequest{Enabled: true, SubjectIDs: []string{"user:" + rolloutBeaID}})
		f.store.storageFails = true

		allowed, err := f.service.Allowed(ctx, "content_ownership", userBea)

		require.ErrorIs(t, err, domain.ErrUnavailable)
		assert.False(t, allowed)
	})
}
