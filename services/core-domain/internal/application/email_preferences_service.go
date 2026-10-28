package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// EmailPreferencesService issues and honours the unsubscribe links in the
// platform's emails. A link works without signing in, so its token is the
// whole authorization: it names one user and one kind of email and carries
// a signature over both, so it can't be altered or made up. Tokens never
// expire, because an unsubscribe link must keep working for as long as the
// email that carries it is kept.
type EmailPreferencesService struct {
	users  ports.UserRepository
	secret []byte
}

func NewEmailPreferencesService(users ports.UserRepository, secret []byte) *EmailPreferencesService {
	return &EmailPreferencesService{users: users, secret: secret}
}

// UnsubscribeToken returns the token that turns kind off for userID:
// "<user id>.<kind>.<signature>".
func (s *EmailPreferencesService) UnsubscribeToken(userID string, kind domain.EmailKind) string {
	payload := userID + "." + string(kind)
	return payload + "." + s.sign(payload)
}

// Unsubscribe turns off the kind of email token names, for the user it was
// issued to, and returns that kind. Turning off a kind that is already off
// succeeds. Every refused token — malformed, altered, signed with another
// secret, or naming a user that no longer exists — is a
// *domain.ValidationError on field "token", and changes nothing.
func (s *EmailPreferencesService) Unsubscribe(ctx context.Context, token string) (domain.EmailKind, error) {
	userID, kind, ok := s.verify(token)
	if !ok {
		return "", invalidUnsubscribeToken()
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", invalidUnsubscribeToken()
		}
		return "", err
	}

	off := false
	update := domain.EmailPreferencesUpdate{}
	switch kind {
	case domain.EmailKindProgressReport:
		update.ProgressReport = &off
	case domain.EmailKindReminders:
		update.Reminders = &off
	}
	if err := s.users.UpdatePreferences(ctx, user.ID, domain.ProfileUpdate{EmailPreferences: &update}); err != nil {
		return "", err
	}
	return kind, nil
}

// verify splits token and checks its signature in constant time. ok is
// false unless the token is well formed, names a uuid and a known kind, and
// was signed with this service's secret.
func (s *EmailPreferencesService) verify(token string) (userID string, kind domain.EmailKind, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", false
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(s.sign(payload))) {
		return "", "", false
	}
	if _, err := uuid.Parse(parts[0]); err != nil {
		return "", "", false
	}
	kind = domain.EmailKind(parts[1])
	if kind != domain.EmailKindProgressReport && kind != domain.EmailKindReminders {
		return "", "", false
	}
	return parts[0], kind, true
}

func (s *EmailPreferencesService) sign(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload)) //nolint:errcheck // hash.Hash.Write never returns an error
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func invalidUnsubscribeToken() error {
	return domain.NewValidationError("token", "is not a valid unsubscribe token")
}
