package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

var unsubscribeSecret = []byte("a-test-secret-that-is-long-enough-to-sign-links")

func newEmailPreferencesFixture() (*application.EmailPreferencesService, *fakeUserRepository) {
	users := newFakeUserRepository()
	users.put(domain.User{
		ID: "11111111-1111-1111-1111-111111111111", ClerkUserID: "clerk-alice", Role: domain.RoleStudent, DisplayName: "Alice Martins",
		EmailPreferences: domain.EmailPreferences{ProgressReport: true, Reminders: true},
	})
	return application.NewEmailPreferencesService(users, unsubscribeSecret), users
}

const aliceID = "11111111-1111-1111-1111-111111111111"

func TestEmailPreferencesService_Unsubscribe(t *testing.T) {
	tests := []struct {
		name      string
		kind      domain.EmailKind
		wantPrefs domain.EmailPreferences
	}{
		{name: "a progress report link turns progress reports off and leaves reminders", kind: domain.EmailKindProgressReport, wantPrefs: domain.EmailPreferences{ProgressReport: false, Reminders: true}},
		{name: "a reminder link turns reminders off and leaves progress reports", kind: domain.EmailKindReminders, wantPrefs: domain.EmailPreferences{ProgressReport: true, Reminders: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, users := newEmailPreferencesFixture()
			token := svc.UnsubscribeToken(aliceID, tt.kind)

			kind, err := svc.Unsubscribe(context.Background(), token)

			require.NoError(t, err)
			assert.Equal(t, tt.kind, kind)
			stored, err := users.GetByID(context.Background(), aliceID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPrefs, stored.EmailPreferences)
		})
	}

	t.Run("using a link twice still succeeds and leaves the kind off", func(t *testing.T) {
		svc, users := newEmailPreferencesFixture()
		token := svc.UnsubscribeToken(aliceID, domain.EmailKindProgressReport)
		_, err := svc.Unsubscribe(context.Background(), token)
		require.NoError(t, err)

		_, err = svc.Unsubscribe(context.Background(), token)

		require.NoError(t, err)
		stored, err := users.GetByID(context.Background(), aliceID)
		require.NoError(t, err)
		assert.False(t, stored.EmailPreferences.ProgressReport)
	})

	t.Run("an old link turns the kind off again after it was turned back on", func(t *testing.T) {
		svc, users := newEmailPreferencesFixture()
		token := svc.UnsubscribeToken(aliceID, domain.EmailKindProgressReport)
		_, err := svc.Unsubscribe(context.Background(), token)
		require.NoError(t, err)
		on := true
		require.NoError(t, users.UpdatePreferences(context.Background(), aliceID, domain.ProfileUpdate{EmailPreferences: &domain.EmailPreferencesUpdate{ProgressReport: &on}}))

		_, err = svc.Unsubscribe(context.Background(), token)

		require.NoError(t, err)
		stored, err := users.GetByID(context.Background(), aliceID)
		require.NoError(t, err)
		assert.False(t, stored.EmailPreferences.ProgressReport)
	})
}

func TestEmailPreferencesService_Unsubscribe_RefusesBadTokens(t *testing.T) {
	svc, _ := newEmailPreferencesFixture()
	valid := svc.UnsubscribeToken(aliceID, domain.EmailKindProgressReport)
	otherSecret := application.NewEmailPreferencesService(newFakeUserRepository(), []byte("another-secret-that-is-long-enough-to-sign"))

	tests := []struct {
		name  string
		token string
	}{
		{name: "an empty token", token: ""},
		{name: "a token with one character changed", token: flipLastChar(valid)},
		{name: "a token signed with another secret", token: otherSecret.UnsubscribeToken(aliceID, domain.EmailKindProgressReport)},
		{name: "a token whose kind was swapped", token: swapKind(valid, string(domain.EmailKindProgressReport), string(domain.EmailKindReminders))},
		{name: "a token for a user that does not exist", token: svc.UnsubscribeToken("22222222-2222-2222-2222-222222222222", domain.EmailKindProgressReport)},
		{name: "a token that is not in the token format", token: "garbage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, users := newEmailPreferencesFixture()

			_, err := svc.Unsubscribe(context.Background(), tt.token)

			var valErr *domain.ValidationError
			require.True(t, errors.As(err, &valErr), "got %v", err)
			assert.Equal(t, "token", valErr.Fields[0].Field)
			stored, getErr := users.GetByID(context.Background(), aliceID)
			require.NoError(t, getErr)
			assert.Equal(t, domain.EmailPreferences{ProgressReport: true, Reminders: true}, stored.EmailPreferences, "a refused token changes nothing")
		})
	}
}

func flipLastChar(s string) string {
	last := s[len(s)-1]
	replacement := byte('A')
	if last == 'A' {
		replacement = 'B'
	}
	return s[:len(s)-1] + string(replacement)
}

func swapKind(token, from, to string) string {
	for i := 0; i+len(from) <= len(token); i++ {
		if token[i:i+len(from)] == from {
			return token[:i] + to + token[i+len(from):]
		}
	}
	return token
}
