package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestValidateGroupChange(t *testing.T) {
	t.Run("a valid slug and name", func(t *testing.T) {
		require.NoError(t, domain.ValidateGroupChange("friends-and-family", "Friends and family"))
	})

	for name, tc := range map[string]struct{ slug, groupName string }{
		"a slug with capitals":       {"Beta", "Beta"},
		"a slug starting with digit": {"1beta", "Beta"},
		"an empty slug":              {"", "Beta"},
		"an empty name":              {"beta", ""},
		"a blank name":               {"beta", "   "},
		"a name over 120 characters": {"beta", strings.Repeat("n", 121)},
	} {
		t.Run("invalid: "+name, func(t *testing.T) {
			require.ErrorIs(t, domain.ValidateGroupChange(tc.slug, tc.groupName), domain.ErrInvalidGroupChange)
		})
	}
}

func TestApplyMembershipChange(t *testing.T) {
	const danID = "0b6f4f2e-6a43-4c55-9f53-5a1f6f2b0003"

	for _, tc := range []struct {
		name        string
		current     []string
		add, remove []string
		want        []string
	}{
		{"adds a user", nil, []string{cleoID}, nil, []string{cleoID}},
		{"removes a member", []string{beaID, cleoID}, nil, []string{beaID}, []string{cleoID}},
		{"adding a current member is not an error", []string{cleoID}, []string{cleoID}, nil, []string{cleoID}},
		{"removing a non-member is not an error", []string{cleoID}, nil, []string{beaID}, []string{cleoID}},
		{"adds and removes in one change", []string{beaID}, []string{danID, cleoID}, []string{beaID}, []string{cleoID, danID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ApplyMembershipChange(tc.current, tc.add, tc.remove)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got, "members are returned sorted")
		})
	}

	for _, tc := range []struct {
		name        string
		add, remove []string
	}{
		{"nothing to add or remove", nil, nil},
		{"the same user added and removed", []string{cleoID}, []string{cleoID}},
		{"a user id that is not a UUID", []string{"cleo"}, nil},
		{"a repeated user", []string{cleoID, cleoID}, nil},
	} {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			_, err := domain.ApplyMembershipChange([]string{beaID}, tc.add, tc.remove)

			require.ErrorIs(t, err, domain.ErrInvalidMembershipChange)
		})
	}
}
