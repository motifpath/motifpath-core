package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

const (
	beaID  = "0b6f4f2e-6a43-4c55-9f53-5a1f6f2b0001"
	cleoID = "0b6f4f2e-6a43-4c55-9f53-5a1f6f2b0002"
)

func TestCommercialFlagRegistry(t *testing.T) {
	t.Run("registers the 17 flags in registry order", func(t *testing.T) {
		flags := domain.CommercialFlags()

		require.Len(t, flags, 17)
		assert.Equal(t, domain.FlagName("content_ownership"), flags[0])
		assert.Equal(t, domain.FlagName("private_beta"), flags[16])
	})

	for _, tc := range []struct {
		flag          string
		prerequisites []domain.FlagName
	}{
		{"content_ownership", nil},
		{"content_versions", []domain.FlagName{"content_ownership"}},
		{"authorization_enforcement", []domain.FlagName{"entitlement_resolution", "access_grants", "progress_retention"}},
		{"plus_checkout", []domain.FlagName{"plus_catalog", "plus_webhooks", "plus_billing"}},
		{"path_purchase", []domain.FlagName{"teacher_paid_paths", "plus_webhooks", "teacher_settlement"}},
		{"private_beta", nil},
	} {
		t.Run("prerequisites of "+tc.flag, func(t *testing.T) {
			flag, err := domain.ParseFlagName(tc.flag)

			require.NoError(t, err)
			assert.Equal(t, tc.prerequisites, flag.Prerequisites())
		})
	}

	t.Run("every prerequisite is itself registered", func(t *testing.T) {
		for _, flag := range domain.CommercialFlags() {
			for _, prerequisite := range flag.Prerequisites() {
				_, err := domain.ParseFlagName(string(prerequisite))
				assert.NoError(t, err, "%s requires %s", flag, prerequisite)
			}
		}
	})

	t.Run("an unregistered flag is refused", func(t *testing.T) {
		_, err := domain.ParseFlagName("free_money")

		require.ErrorIs(t, err, domain.ErrUnknownFlag)
	})
}

func TestParseRolloutSubject(t *testing.T) {
	for _, tc := range []struct {
		value string
		kind  domain.RolloutSubjectKind
		id    string
	}{
		{"user:" + beaID, domain.RolloutSubjectUser, beaID},
		{"group:friends-and-family", domain.RolloutSubjectGroup, "friends-and-family"},
		{"service:core-domain", domain.RolloutSubjectService, "core-domain"},
	} {
		t.Run("accepted: "+tc.value, func(t *testing.T) {
			subject, err := domain.ParseRolloutSubject(tc.value)

			require.NoError(t, err)
			assert.Equal(t, tc.kind, subject.Kind)
			assert.Equal(t, tc.id, subject.ID)
			assert.Equal(t, tc.value, subject.String())
		})
	}

	for name, value := range map[string]string{
		"no kind":                    beaID,
		"an unknown kind":            "team:beta",
		"a user id that is no UUID":  "user:bea",
		"an upper-case user id":      "user:0B6F4F2E-6A43-4C55-9F53-5A1F6F2B0001",
		"a group slug with capitals": "group:Beta",
		"a slug starting with digit": "group:1beta",
		"a wildcard":                 "user:*",
		"an empty service":           "service:",
	} {
		t.Run("rejected: "+name, func(t *testing.T) {
			_, err := domain.ParseRolloutSubject(value)

			require.ErrorIs(t, err, domain.ErrInvalidRolloutSubject)
		})
	}
}

func TestCommercialActivation_Allows(t *testing.T) {
	user := func(id string) domain.RolloutSubject {
		return domain.RolloutSubject{Kind: domain.RolloutSubjectUser, ID: id}
	}
	service := func(name string) domain.RolloutSubject {
		return domain.RolloutSubject{Kind: domain.RolloutSubjectService, ID: name}
	}
	entries := func(values ...string) []domain.RolloutSubject {
		subjects := make([]domain.RolloutSubject, 0, len(values))
		for _, value := range values {
			subject, err := domain.ParseRolloutSubject(value)
			require.NoError(t, err)
			subjects = append(subjects, subject)
		}
		return subjects
	}
	on := func(audience domain.RolloutAudience) domain.FlagSetting {
		return domain.FlagSetting{Enabled: true, Audience: audience}
	}

	for _, tc := range []struct {
		name    string
		flags   map[domain.FlagName]domain.FlagSetting
		groups  map[string][]string
		flag    domain.FlagName
		subject domain.RolloutSubject
		allowed bool
	}{
		{
			name:    "a fresh environment allows no one",
			flag:    "content_ownership",
			subject: user(beaID),
		},
		{
			name:    "an enabled flag allows a listed user",
			flags:   map[domain.FlagName]domain.FlagSetting{"content_ownership": on(domain.RolloutAudience{Entries: entries("user:" + beaID)})},
			flag:    "content_ownership",
			subject: user(beaID),
			allowed: true,
		},
		{
			name:    "an enabled flag does not allow an unlisted user",
			flags:   map[domain.FlagName]domain.FlagSetting{"content_ownership": on(domain.RolloutAudience{Entries: entries("user:" + beaID)})},
			flag:    "content_ownership",
			subject: user(cleoID),
		},
		{
			name:    "an enabled flag with no subjects allows no one",
			flags:   map[domain.FlagName]domain.FlagSetting{"content_ownership": on(domain.RolloutAudience{})},
			flag:    "content_ownership",
			subject: user(beaID),
		},
		{
			name:    "a disabled flag keeps its audience but allows no one",
			flags:   map[domain.FlagName]domain.FlagSetting{"content_ownership": {Audience: domain.RolloutAudience{AllUsers: true}}},
			flag:    "content_ownership",
			subject: user(beaID),
		},
		{
			name:    "all users covers any user",
			flags:   map[domain.FlagName]domain.FlagSetting{"content_ownership": on(domain.RolloutAudience{AllUsers: true})},
			flag:    "content_ownership",
			subject: user(cleoID),
			allowed: true,
		},
		{
			name:    "all users never covers an unlisted service",
			flags:   map[domain.FlagName]domain.FlagSetting{"content_ownership": on(domain.RolloutAudience{AllUsers: true})},
			flag:    "content_ownership",
			subject: service("core-domain"),
		},
		{
			name:    "a listed service is allowed alongside all users",
			flags:   map[domain.FlagName]domain.FlagSetting{"content_ownership": on(domain.RolloutAudience{AllUsers: true, Entries: entries("service:core-domain")})},
			flag:    "content_ownership",
			subject: service("core-domain"),
			allowed: true,
		},
		{
			name:    "a group entry allows a member",
			flags:   map[domain.FlagName]domain.FlagSetting{"private_beta": on(domain.RolloutAudience{Entries: entries("group:beta")})},
			groups:  map[string][]string{"beta": {cleoID}},
			flag:    "private_beta",
			subject: user(cleoID),
			allowed: true,
		},
		{
			name:    "a group entry does not allow a non-member",
			flags:   map[domain.FlagName]domain.FlagSetting{"private_beta": on(domain.RolloutAudience{Entries: entries("group:beta")})},
			groups:  map[string][]string{"beta": {cleoID}},
			flag:    "private_beta",
			subject: user(beaID),
		},
		{
			name:    "a group entry never covers a service",
			flags:   map[domain.FlagName]domain.FlagSetting{"private_beta": on(domain.RolloutAudience{Entries: entries("group:beta")})},
			groups:  map[string][]string{"beta": {cleoID}},
			flag:    "private_beta",
			subject: service(cleoID),
		},
		{
			name: "a disabled prerequisite denies a dependent that is ON",
			flags: map[domain.FlagName]domain.FlagSetting{
				"content_versions": on(domain.RolloutAudience{Entries: entries("user:" + beaID)}),
			},
			flag:    "content_versions",
			subject: user(beaID),
		},
		{
			name: "a prerequisite that does not allow the subject denies the dependent",
			flags: map[domain.FlagName]domain.FlagSetting{
				"content_ownership": on(domain.RolloutAudience{Entries: entries("user:" + cleoID)}),
				"content_versions":  on(domain.RolloutAudience{Entries: entries("user:" + beaID)}),
			},
			flag:    "content_versions",
			subject: user(beaID),
		},
		{
			name: "a dependent is allowed when every prerequisite allows the subject",
			flags: map[domain.FlagName]domain.FlagSetting{
				"content_ownership": on(domain.RolloutAudience{AllUsers: true}),
				"content_versions":  on(domain.RolloutAudience{Entries: entries("user:" + beaID)}),
			},
			flag:    "content_versions",
			subject: user(beaID),
			allowed: true,
		},
		{
			name: "a disabled transitive prerequisite denies",
			flags: map[domain.FlagName]domain.FlagSetting{
				"content_versions":      on(domain.RolloutAudience{AllUsers: true}),
				"commercial_validation": on(domain.RolloutAudience{AllUsers: true}),
			},
			flag:    "commercial_validation",
			subject: user(beaID),
		},
		{
			name:    "a group subject is never an operation subject",
			flags:   map[domain.FlagName]domain.FlagSetting{"private_beta": on(domain.RolloutAudience{AllUsers: true, Entries: entries("group:beta")})},
			groups:  map[string][]string{"beta": {cleoID}},
			flag:    "private_beta",
			subject: domain.RolloutSubject{Kind: domain.RolloutSubjectGroup, ID: "beta"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := domain.NewCommercialActivation(3, tc.flags, tc.groups)

			assert.Equal(t, tc.allowed, state.Allows(tc.flag, tc.subject))
		})
	}
}

func TestCommercialActivation_Effective(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flags     map[domain.FlagName]domain.FlagSetting
		flag      domain.FlagName
		effective bool
	}{
		{"a fresh flag is not effective", nil, "content_ownership", false},
		{"an enabled flag without prerequisites is effective", map[domain.FlagName]domain.FlagSetting{"content_ownership": {Enabled: true}}, "content_ownership", true},
		{"enabled with a disabled prerequisite is not effective", map[domain.FlagName]domain.FlagSetting{"content_versions": {Enabled: true}}, "content_versions", false},
		{"enabled with every prerequisite effective is effective", map[domain.FlagName]domain.FlagSetting{"content_ownership": {Enabled: true}, "content_versions": {Enabled: true}}, "content_versions", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := domain.NewCommercialActivation(0, tc.flags, nil)

			assert.Equal(t, tc.effective, state.Effective(tc.flag))
		})
	}

	t.Run("an unregistered flag is neither effective nor allowing", func(t *testing.T) {
		state := domain.NewCommercialActivation(0, map[domain.FlagName]domain.FlagSetting{"free_money": {Enabled: true, Audience: domain.RolloutAudience{AllUsers: true}}}, nil)

		assert.False(t, state.Effective("free_money"))
		assert.False(t, state.Allows("free_money", domain.RolloutSubject{Kind: domain.RolloutSubjectUser, ID: beaID}))
	})
}
