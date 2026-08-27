package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

var fixedRegisteredAt = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

func newIdentityService(users *fakeUserRepository) *application.IdentityService {
	return application.NewIdentityService(users, idSequence(), func() time.Time { return fixedRegisteredAt })
}

func TestIdentityService_RegisterUser(t *testing.T) {
	t.Run("student registers successfully", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent)

		require.NoError(t, err)
		assert.Equal(t, "id-1", user.ID)
		assert.Equal(t, domain.RoleStudent, user.Role)
		assert.Equal(t, fixedRegisteredAt, user.RegisteredAt)
	})

	t.Run("teacher registers successfully", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-bob", domain.RoleTeacher)

		require.NoError(t, err)
		assert.Equal(t, domain.RoleTeacher, user.Role)
	})

	t.Run("registering the same Clerk identity twice is refused", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent)
		require.NoError(t, err)

		_, err = svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent)
		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("registering the same Clerk identity with a different role is also refused", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent)
		require.NoError(t, err)

		_, err = svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleTeacher)
		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("registration with an unrecognised role is rejected", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.Role("moderator"))

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "role", valErr.Fields[0].Field)
	})

	t.Run("self-registering as admin is rejected", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleAdmin)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "role", valErr.Fields[0].Field)
	})
}

func TestIdentityService_GetProfile(t *testing.T) {
	t.Run("a registered user retrieves their own profile", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		registered, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent)
		require.NoError(t, err)

		profile, err := svc.GetProfile(context.Background(), "clerk-alice")

		require.NoError(t, err)
		assert.Equal(t, registered, profile)
	})

	t.Run("requesting a profile before registering returns not found", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.GetProfile(context.Background(), "clerk-charlie")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
