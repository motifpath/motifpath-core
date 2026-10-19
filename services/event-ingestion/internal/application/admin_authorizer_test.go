package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/application"
	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

func adminProfile(role string) *fakeProfileResolver {
	return &fakeProfileResolver{profile: ports.Profile{UserID: "caller-user", Role: role}}
}

func TestAdminAuthorizer_RequireAdmin_AllowsAdminRole(t *testing.T) {
	authorizer := application.NewAdminAuthorizer(adminProfile("admin"))

	err := authorizer.RequireAdmin(context.Background(), "token-abc")

	require.NoError(t, err)
}

func TestAdminAuthorizer_RequireAdmin_ForwardsTheBearerToken(t *testing.T) {
	resolver := adminProfile("admin")
	authorizer := application.NewAdminAuthorizer(resolver)

	err := authorizer.RequireAdmin(context.Background(), "token-abc")

	require.NoError(t, err)
	assert.Equal(t, "token-abc", resolver.lastToken)
}

func TestAdminAuthorizer_RequireAdmin_RejectsNonAdminRole(t *testing.T) {
	for _, role := range []string{"student", "teacher", "", "administrator"} {
		t.Run(role, func(t *testing.T) {
			authorizer := application.NewAdminAuthorizer(adminProfile(role))

			err := authorizer.RequireAdmin(context.Background(), "token-abc")

			require.ErrorIs(t, err, domain.ErrForbidden)
		})
	}
}

func TestAdminAuthorizer_RequireAdmin_TreatsUnregisteredCallerAsForbidden(t *testing.T) {
	authorizer := application.NewAdminAuthorizer(&fakeProfileResolver{err: ports.ErrIdentityNotRegistered})

	err := authorizer.RequireAdmin(context.Background(), "token-abc")

	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestAdminAuthorizer_RequireAdmin_FailsClosedWhenProfileUnavailable(t *testing.T) {
	authorizer := application.NewAdminAuthorizer(&fakeProfileResolver{err: ports.ErrProfileUnavailable})

	err := authorizer.RequireAdmin(context.Background(), "token-abc")

	require.ErrorIs(t, err, domain.ErrAuthorizationUnavailable)
}

func TestAdminAuthorizer_RequireAdmin_FailsClosedOnAnUnexpectedResolverError(t *testing.T) {
	authorizer := application.NewAdminAuthorizer(&fakeProfileResolver{err: errors.New("boom")})

	err := authorizer.RequireAdmin(context.Background(), "token-abc")

	require.ErrorIs(t, err, domain.ErrAuthorizationUnavailable)
}
