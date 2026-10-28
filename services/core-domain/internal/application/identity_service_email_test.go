package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func claimsWithEmail(email string) domain.IdentityClaims {
	return domain.IdentityClaims{Name: "Alice Martins", Email: email}
}

func TestIdentityService_RegisterUser_Email(t *testing.T) {
	tests := []struct {
		name        string
		claimed     string
		wantEmail   string
		wantInvalid bool
	}{
		{name: "records the claimed address", claimed: "alice@example.com", wantEmail: "alice@example.com"},
		{name: "trims surrounding whitespace", claimed: "  alice@example.com  ", wantEmail: "alice@example.com"},
		{name: "rejects a missing address", claimed: "", wantInvalid: true},
		{name: "rejects a blank address", claimed: "   ", wantInvalid: true},
		{name: "rejects a value that is not an address", claimed: "not-an-address", wantInvalid: true},
		{name: "rejects an address with a display name", claimed: "Alice <alice@example.com>", wantInvalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := newFakeUserRepository()
			svc := newIdentityService(users)

			user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", claimsWithEmail(tt.claimed))

			if tt.wantInvalid {
				var valErr *domain.ValidationError
				require.True(t, errors.As(err, &valErr), "got %v", err)
				assert.Equal(t, "email", valErr.Fields[0].Field)
				_, getErr := users.GetByClerkUserID(context.Background(), "clerk-alice")
				assert.ErrorIs(t, getErr, domain.ErrNotFound, "no user may register without an address")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantEmail, user.Email)
			stored, err := users.GetByClerkUserID(context.Background(), "clerk-alice")
			require.NoError(t, err)
			assert.Equal(t, tt.wantEmail, stored.Email)
		})
	}
}

func TestIdentityService_RegisterUser_EmailPreferencesStartOn(t *testing.T) {
	users := newFakeUserRepository()
	svc := newIdentityService(users)

	user, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", claimsWithEmail("alice@example.com"))

	require.NoError(t, err)
	assert.Equal(t, domain.EmailPreferences{ProgressReport: true, Reminders: true}, user.EmailPreferences)
	assert.Empty(t, user.TimeZone, "a new user has no time zone until one is set")
}

func TestIdentityService_ResolveCaller_RefreshesEmail(t *testing.T) {
	tests := []struct {
		name      string
		stored    string
		claimed   string
		wantEmail string
		wantWrite bool
	}{
		{name: "a changed address is stored", stored: "alice@example.com", claimed: "alice.martins@example.com", wantEmail: "alice.martins@example.com", wantWrite: true},
		{name: "a changed address is trimmed before it is stored", stored: "alice@example.com", claimed: " alice.martins@example.com ", wantEmail: "alice.martins@example.com", wantWrite: true},
		{name: "a user without a stored address gets the claimed one", stored: "", claimed: "alice@example.com", wantEmail: "alice@example.com", wantWrite: true},
		{name: "an unchanged address writes nothing", stored: "alice@example.com", claimed: "alice@example.com", wantEmail: "alice@example.com"},
		{name: "a missing claim leaves the stored address unchanged", stored: "alice@example.com", claimed: "", wantEmail: "alice@example.com"},
		{name: "an invalid claim leaves the stored address unchanged", stored: "alice@example.com", claimed: "not-an-address", wantEmail: "alice@example.com"},
		{name: "a user without an address and without a claim still has none", stored: "", claimed: "", wantEmail: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := newFakeUserRepository()
			users.put(domain.User{ID: "user-alice", ClerkUserID: "clerk-alice", Role: domain.RoleStudent, DisplayName: "Alice Martins", Email: tt.stored})
			svc := newIdentityService(users)

			caller, err := svc.ResolveCaller(context.Background(), "clerk-alice", domain.IdentityClaims{Name: "Alice Martins", Email: tt.claimed})

			require.NoError(t, err)
			assert.Equal(t, tt.wantEmail, caller.Email)
			stored, err := users.GetByID(context.Background(), "user-alice")
			require.NoError(t, err)
			assert.Equal(t, tt.wantEmail, stored.Email)
			assert.Equal(t, tt.wantWrite, users.emailWrites > 0)
		})
	}
}

func TestIdentityService_UpdateProfile(t *testing.T) {
	ptr := func(s string) *string { return &s }
	on, off := true, false

	tests := []struct {
		name         string
		update       domain.ProfileUpdate
		wantInvalid  string // the field a validation error must name; "" for success
		wantLocale   string
		wantTimeZone string
		wantPrefs    domain.EmailPreferences
	}{
		{
			name:       "sets the time zone and leaves the locale",
			update:     domain.ProfileUpdate{TimeZone: ptr("Europe/Lisbon")},
			wantLocale: "pt_BR", wantTimeZone: "Europe/Lisbon",
			wantPrefs: domain.EmailPreferences{ProgressReport: true, Reminders: true},
		},
		{
			name:       "sets the locale and the time zone together",
			update:     domain.ProfileUpdate{Locale: ptr("en"), TimeZone: ptr("Europe/Lisbon")},
			wantLocale: "en", wantTimeZone: "Europe/Lisbon",
			wantPrefs: domain.EmailPreferences{ProgressReport: true, Reminders: true},
		},
		{
			name:       "turns one email preference off and leaves the other",
			update:     domain.ProfileUpdate{EmailPreferences: &domain.EmailPreferencesUpdate{ProgressReport: &off}},
			wantLocale: "pt_BR", wantTimeZone: "America/Sao_Paulo",
			wantPrefs: domain.EmailPreferences{ProgressReport: false, Reminders: true},
		},
		{
			name:       "turns an email preference back on",
			update:     domain.ProfileUpdate{EmailPreferences: &domain.EmailPreferencesUpdate{ProgressReport: &off, Reminders: &on}},
			wantLocale: "pt_BR", wantTimeZone: "America/Sao_Paulo",
			wantPrefs: domain.EmailPreferences{ProgressReport: false, Reminders: true},
		},
		{name: "rejects an unknown time zone", update: domain.ProfileUpdate{TimeZone: ptr("Mars/Olympus_Mons")}, wantInvalid: "time_zone"},
		{name: "rejects a blank time zone", update: domain.ProfileUpdate{TimeZone: ptr("")}, wantInvalid: "time_zone"},
		{name: "rejects the local time zone name", update: domain.ProfileUpdate{TimeZone: ptr("Local")}, wantInvalid: "time_zone"},
		{name: "rejects an unknown locale", update: domain.ProfileUpdate{Locale: ptr("xx_YY")}, wantInvalid: "locale"},
		{name: "rejects email preferences that name no known switch", update: domain.ProfileUpdate{EmailPreferences: &domain.EmailPreferencesUpdate{}}, wantInvalid: "email_preferences"},
		{name: "rejects an update that changes nothing", update: domain.ProfileUpdate{}, wantInvalid: "body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := newFakeUserRepository()
			users.put(domain.User{
				ID: "user-alice", ClerkUserID: "clerk-alice", Role: domain.RoleStudent, DisplayName: "Alice Martins",
				Locale: domain.Language{Code: "pt_BR"}, TimeZone: "America/Sao_Paulo",
				EmailPreferences: domain.EmailPreferences{ProgressReport: true, Reminders: true},
			})
			svc := newIdentityService(users)

			user, err := svc.UpdateProfile(context.Background(), "clerk-alice", tt.update)

			if tt.wantInvalid != "" {
				var valErr *domain.ValidationError
				require.True(t, errors.As(err, &valErr), "got %v", err)
				assert.Equal(t, tt.wantInvalid, valErr.Fields[0].Field)
				stored, getErr := users.GetByID(context.Background(), "user-alice")
				require.NoError(t, getErr)
				assert.Equal(t, "America/Sao_Paulo", stored.TimeZone, "a rejected update changes nothing")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLocale, user.Locale.Code)
			assert.Equal(t, tt.wantTimeZone, user.TimeZone)
			assert.Equal(t, tt.wantPrefs, user.EmailPreferences)
			stored, err := users.GetByID(context.Background(), "user-alice")
			require.NoError(t, err)
			assert.Equal(t, tt.wantLocale, stored.Locale.Code)
			assert.Equal(t, tt.wantTimeZone, stored.TimeZone)
			assert.Equal(t, tt.wantPrefs, stored.EmailPreferences)
		})
	}

	t.Run("updating before registering returns not found", func(t *testing.T) {
		svc := newIdentityService(newFakeUserRepository())
		zone := "America/Sao_Paulo"

		_, err := svc.UpdateProfile(context.Background(), "clerk-charlie", domain.ProfileUpdate{TimeZone: &zone})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestIdentityService_RegisterUser_ReportsNameAndEmailTogether(t *testing.T) {
	svc := newIdentityService(newFakeUserRepository())

	_, err := svc.RegisterUser(context.Background(), "clerk-alice", domain.RoleStudent, "", domain.IdentityClaims{Name: " ", Email: ""})

	var valErr *domain.ValidationError
	require.True(t, errors.As(err, &valErr), "got %v", err)
	fields := make([]string, len(valErr.Fields))
	for i, f := range valErr.Fields {
		fields[i] = f.Field
	}
	assert.ElementsMatch(t, []string{"name", "email"}, fields)
}
