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
	return newIdentityServiceWithLanguages(users, newFakeLanguageRepository())
}

func newIdentityServiceWithLanguages(users *fakeUserRepository, languages *fakeLanguageRepository) *application.IdentityService {
	return application.NewIdentityService(users, languages, idSequence(), func() time.Time { return fixedRegisteredAt })
}

func TestIdentityService_RegisterUser(t *testing.T) {
	t.Run("student registers successfully", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "")

		require.NoError(t, err)
		assert.Equal(t, "id-1", user.ID)
		assert.Equal(t, domain.RoleStudent, user.Role)
		assert.Equal(t, fixedRegisteredAt, user.RegisteredAt)
	})

	t.Run("teacher registers successfully", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-bob", domain.RoleTeacher, "")

		require.NoError(t, err)
		assert.Equal(t, domain.RoleTeacher, user.Role)
	})

	t.Run("registering the same Clerk identity twice is refused", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "")
		require.NoError(t, err)

		_, err = svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "")
		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("registering the same Clerk identity with a different role is also refused", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "")
		require.NoError(t, err)

		_, err = svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleTeacher, "")
		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("registration with an unrecognised role is rejected", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.Role("moderator"), "")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "role", valErr.Fields[0].Field)
	})

	t.Run("self-registering as admin is rejected", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleAdmin, "")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "role", valErr.Fields[0].Field)
	})

	t.Run("registering with no Accept-Language candidate defaults locale to en", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "")

		require.NoError(t, err)
		assert.Equal(t, "en", user.Locale.Code)
	})

	t.Run("registering with a known Accept-Language candidate adopts it as locale", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "pt_BR")

		require.NoError(t, err)
		assert.Equal(t, "pt_BR", user.Locale.Code)
	})

	t.Run("registering with an unknown Accept-Language candidate falls back to en", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "fr")

		require.NoError(t, err)
		assert.Equal(t, "en", user.Locale.Code)
	})

	t.Run("registering with an Accept-Language candidate of any falls back to en", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, domain.LanguageCodeAny)

		require.NoError(t, err)
		assert.Equal(t, "en", user.Locale.Code)
	})
}

func TestIdentityService_GetProfile(t *testing.T) {
	t.Run("a registered user retrieves their own profile", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		registered, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "")
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

func TestIdentityService_UpdateLocale(t *testing.T) {
	t.Run("a registered user updates their locale to a known code", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)
		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "")
		require.NoError(t, err)

		user, err := svc.UpdateLocale(context.Background(), "clerk-alice", "pt_BR")

		require.NoError(t, err)
		assert.Equal(t, "pt_BR", user.Locale.Code)

		profile, err := svc.GetProfile(context.Background(), "clerk-alice")
		require.NoError(t, err)
		assert.Equal(t, "pt_BR", profile.Locale.Code)
	})

	t.Run("an unknown language code is rejected", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)
		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "")
		require.NoError(t, err)

		_, err = svc.UpdateLocale(context.Background(), "clerk-alice", "xx")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "locale", valErr.Fields[0].Field)
	})

	t.Run("any is rejected as a user locale", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)
		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "")
		require.NoError(t, err)

		_, err = svc.UpdateLocale(context.Background(), "clerk-alice", domain.LanguageCodeAny)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "locale", valErr.Fields[0].Field)
	})

	t.Run("updating locale before registering returns not found", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.UpdateLocale(context.Background(), "clerk-charlie", "pt_BR")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
