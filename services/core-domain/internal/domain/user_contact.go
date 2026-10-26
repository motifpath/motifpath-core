package domain

import (
	"net/mail"
	"strings"
	"time"
)

// MaxEmailLength is the longest address MotifPath stores: the length of the
// longest path an SMTP server must accept.
const MaxEmailLength = 254

// IdentityClaims are the facts about a user that their identity token
// carries, verified by the identity provider's signature. They are the only
// source of a user's name and email address: neither is ever taken from a
// request body.
type IdentityClaims struct {
	Name  string
	Email string
}

// NormalizeEmail trims raw and accepts it only when it is a bare address
// (no display name, no angle brackets) of at most MaxEmailLength
// characters. ok is false for anything else, including an empty value.
func NormalizeEmail(raw string) (email string, ok bool) {
	email = strings.TrimSpace(raw)
	if email == "" || len(email) > MaxEmailLength {
		return "", false
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Name != "" || parsed.Address != email {
		return "", false
	}
	return email, true
}

// ValidateTimeZone accepts only a named IANA time zone. "Local" and the
// empty name are refused: they describe the server, not the user.
func ValidateTimeZone(name string) error {
	if name == "" || name == "Local" {
		return NewValidationError("time_zone", "must be a known IANA time zone")
	}
	if _, err := time.LoadLocation(name); err != nil {
		return NewValidationError("time_zone", "must be a known IANA time zone")
	}
	return nil
}

// EmailKind names one kind of email a student can get, each with its own
// preference switch.
type EmailKind string

const (
	EmailKindProgressReport EmailKind = "progress_report"
	EmailKindReminders      EmailKind = "reminders"
)

// EmailPreferences holds one switch per EmailKind. Both start on.
type EmailPreferences struct {
	ProgressReport bool
	Reminders      bool
}

// DefaultEmailPreferences are a new user's preferences: every kind on.
func DefaultEmailPreferences() EmailPreferences {
	return EmailPreferences{ProgressReport: true, Reminders: true}
}

// EmailPreferencesUpdate changes the switches that are set and keeps the
// others.
type EmailPreferencesUpdate struct {
	ProgressReport *bool
	Reminders      *bool
}

// IsEmpty reports whether the update names no switch.
func (u EmailPreferencesUpdate) IsEmpty() bool {
	return u.ProgressReport == nil && u.Reminders == nil
}

// Apply returns p with the update's switches applied.
func (u EmailPreferencesUpdate) Apply(p EmailPreferences) EmailPreferences {
	if u.ProgressReport != nil {
		p.ProgressReport = *u.ProgressReport
	}
	if u.Reminders != nil {
		p.Reminders = *u.Reminders
	}
	return p
}

// ProfileUpdate changes the fields of a user's preferences that are set
// and keeps the others. Locale is a Language code.
type ProfileUpdate struct {
	Locale           *string
	TimeZone         *string
	EmailPreferences *EmailPreferencesUpdate
}

// IsEmpty reports whether the update changes nothing.
func (u ProfileUpdate) IsEmpty() bool {
	return u.Locale == nil && u.TimeZone == nil && u.EmailPreferences == nil
}
