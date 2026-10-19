package application_test

import (
	"context"
	"errors"
	"strings"
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

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Test User")

		require.NoError(t, err)
		assert.Equal(t, "id-1", user.ID)
		assert.Equal(t, domain.RoleStudent, user.Role)
		assert.Equal(t, fixedRegisteredAt, user.RegisteredAt)
	})

	t.Run("teacher registers successfully", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-bob", domain.RoleTeacher, "", "Test User")

		require.NoError(t, err)
		assert.Equal(t, domain.RoleTeacher, user.Role)
	})

	t.Run("registering the same Clerk identity twice is refused", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Test User")
		require.NoError(t, err)

		_, err = svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Test User")
		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("registering the same Clerk identity with a different role is also refused", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Test User")
		require.NoError(t, err)

		_, err = svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleTeacher, "", "Test User")
		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("registration with an unrecognised role is rejected", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.Role("moderator"), "", "Test User")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "role", valErr.Fields[0].Field)
	})

	t.Run("self-registering as admin is rejected", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleAdmin, "", "Test User")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "role", valErr.Fields[0].Field)
	})

	t.Run("registering with no Accept-Language candidate defaults locale to en", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Test User")

		require.NoError(t, err)
		assert.Equal(t, "en", user.Locale.Code)
	})

	t.Run("registering with a known Accept-Language candidate adopts it as locale", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "pt_BR", "Test User")

		require.NoError(t, err)
		assert.Equal(t, "pt_BR", user.Locale.Code)
	})

	t.Run("registering with an unknown Accept-Language candidate falls back to en", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "fr", "Test User")

		require.NoError(t, err)
		assert.Equal(t, "en", user.Locale.Code)
	})

	t.Run("registering with an Accept-Language candidate of any falls back to en", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, domain.LanguageCodeAny, "Test User")

		require.NoError(t, err)
		assert.Equal(t, "en", user.Locale.Code)
	})
}

func TestIdentityService_ResolveCaller(t *testing.T) {
	t.Run("a registered user retrieves their own profile", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		registered, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Test User")
		require.NoError(t, err)

		profile, err := svc.ResolveCaller(context.Background(), "clerk-alice", "")

		require.NoError(t, err)
		assert.Equal(t, registered, profile)
	})

	t.Run("requesting a profile before registering returns not found", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)

		_, err := svc.ResolveCaller(context.Background(), "clerk-charlie", "")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestIdentityService_UpdateLocale(t *testing.T) {
	t.Run("a registered user updates their locale to a known code", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)
		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Test User")
		require.NoError(t, err)

		user, err := svc.UpdateLocale(context.Background(), "clerk-alice", "pt_BR")

		require.NoError(t, err)
		assert.Equal(t, "pt_BR", user.Locale.Code)

		profile, err := svc.ResolveCaller(context.Background(), "clerk-alice", "")
		require.NoError(t, err)
		assert.Equal(t, "pt_BR", profile.Locale.Code)
	})

	t.Run("an unknown language code is rejected", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)
		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Test User")
		require.NoError(t, err)

		_, err = svc.UpdateLocale(context.Background(), "clerk-alice", "xx")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "locale", valErr.Fields[0].Field)
	})

	t.Run("any is rejected as a user locale", func(t *testing.T) {
		users := newFakeUserRepository()
		svc := newIdentityService(users)
		_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Test User")
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

func TestIdentityService_RegisterUser_DisplayName(t *testing.T) {
	tests := []struct {
		name        string
		claimedName string
		wantName    string
		wantInvalid bool
	}{
		{name: "records the claimed name", claimedName: "Alice Martins", wantName: "Alice Martins"},
		{name: "trims surrounding whitespace", claimedName: "  Alice Martins  ", wantName: "Alice Martins"},
		{name: "cuts a long name to its first 200 characters", claimedName: strings.Repeat("á", 250), wantName: strings.Repeat("á", 200)},
		{name: "does not leave trailing whitespace where the cut falls", claimedName: strings.Repeat("a", 199) + "  tail", wantName: strings.Repeat("a", 199)},
		{name: "rejects a missing name", claimedName: "", wantInvalid: true},
		{name: "rejects a blank name", claimedName: "   ", wantInvalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := newFakeUserRepository()
			svc := newIdentityService(users)

			user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", tt.claimedName)

			if tt.wantInvalid {
				var valErr *domain.ValidationError
				require.True(t, errors.As(err, &valErr))
				assert.Equal(t, "name", valErr.Fields[0].Field)
				_, getErr := users.GetByClerkUserID(context.Background(), "clerk-alice")
				assert.ErrorIs(t, getErr, domain.ErrNotFound, "no user record may exist without a name")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, user.DisplayName)
			stored, err := users.GetByClerkUserID(context.Background(), "clerk-alice")
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, stored.DisplayName)
		})
	}
}

func TestIdentityService_ResolveCaller_RefreshesDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		claimedName string
		wantName    string
		wantWrite   bool
	}{
		{name: "a changed name is stored", claimedName: "Bob Ferreira Lima", wantName: "Bob Ferreira Lima", wantWrite: true},
		{name: "a changed name is normalized before it is stored", claimedName: "  Bob Ferreira Lima ", wantName: "Bob Ferreira Lima", wantWrite: true},
		{name: "an unchanged name writes nothing", claimedName: "Bob Ferreira", wantName: "Bob Ferreira"},
		{name: "an unchanged name that only differs by whitespace writes nothing", claimedName: " Bob Ferreira ", wantName: "Bob Ferreira"},
		{name: "a missing claim leaves the stored name unchanged", claimedName: "", wantName: "Bob Ferreira"},
		{name: "a blank claim leaves the stored name unchanged", claimedName: "   ", wantName: "Bob Ferreira"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := newFakeUserRepository()
			svc := newIdentityService(users)
			_, err := svc.RegisterUser(context.Background(), "clerk-bob", domain.RoleTeacher, "", "Bob Ferreira")
			require.NoError(t, err)

			caller, err := svc.ResolveCaller(context.Background(), "clerk-bob", tt.claimedName)

			require.NoError(t, err)
			assert.Equal(t, tt.wantName, caller.DisplayName)
			stored, err := users.GetByClerkUserID(context.Background(), "clerk-bob")
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, stored.DisplayName)
			assert.Equal(t, tt.wantWrite, users.displayNameWrites > 0)
		})
	}
}

func TestIdentityService_DisplayNames(t *testing.T) {
	setup := func(t *testing.T) (*application.IdentityService, *fakeUserRepository, domain.User, domain.User) {
		t.Helper()
		users := newFakeUserRepository()
		svc := newIdentityService(users)
		alice, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", "Alice Martins")
		require.NoError(t, err)
		bob, err := svc.RegisterUser(context.Background(), "clerk-bob", domain.RoleTeacher, "", "Bob Ferreira")
		require.NoError(t, err)
		return svc, users, alice, bob
	}

	tests := []struct {
		name    string
		ids     func(alice, bob domain.User) []string
		want    func(alice, bob domain.User) map[string]string
		wantErr error
		// wantLookups, when set, is the exact list of id batches the
		// repository must have been asked for.
		wantLookups func(alice, bob domain.User) [][]string
	}{
		{
			name: "returns each user's current name by id",
			ids:  func(alice, bob domain.User) []string { return []string{alice.ID, bob.ID} },
			want: func(alice, bob domain.User) map[string]string {
				return map[string]string{alice.ID: "Alice Martins", bob.ID: "Bob Ferreira"}
			},
		},
		{
			name: "repeated ids are looked up once",
			ids:  func(alice, bob domain.User) []string { return []string{bob.ID, alice.ID, bob.ID, bob.ID} },
			want: func(alice, bob domain.User) map[string]string {
				return map[string]string{alice.ID: "Alice Martins", bob.ID: "Bob Ferreira"}
			},
			wantLookups: func(alice, bob domain.User) [][]string { return [][]string{{bob.ID, alice.ID}} },
		},
		{
			name:        "no ids asks the repository for nothing",
			ids:         func(alice, bob domain.User) []string { return nil },
			want:        func(alice, bob domain.User) map[string]string { return map[string]string{} },
			wantLookups: func(alice, bob domain.User) [][]string { return nil },
		},
		{
			name:    "an id with no user is an error, not a blank name",
			ids:     func(alice, bob domain.User) []string { return []string{alice.ID, "no-such-user"} },
			wantErr: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, users, alice, bob := setup(t)

			names, err := svc.DisplayNames(context.Background(), tt.ids(alice, bob))

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want(alice, bob), names)
			if tt.wantLookups != nil {
				assert.Equal(t, tt.wantLookups(alice, bob), users.displayNameLookups)
			}
		})
	}
}
