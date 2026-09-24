package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Role is a User's platform role. It is immutable after registration.
type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
)

// MaxDisplayNameLength is the longest display name, in characters (not
// bytes), that MotifPath stores. A longer name is cut, not rejected: the
// name comes from the identity provider, which the user can't be asked to
// shorten mid-request.
const MaxDisplayNameLength = 200

// NormalizeDisplayName trims raw, cuts it to MaxDisplayNameLength
// characters and trims again, so a cut that lands on a space leaves no
// trailing whitespace. ok is false when nothing but whitespace remains.
func NormalizeDisplayName(raw string) (name string, ok bool) {
	name = strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) > MaxDisplayNameLength {
		name = strings.TrimSpace(string([]rune(name)[:MaxDisplayNameLength]))
	}
	return name, name != ""
}

// User is a registered MotifPath identity, mapped 1:1 to a Clerk identity
// via ClerkUserID.
type User struct {
	ID          string
	ClerkUserID string
	Role        Role
	// DisplayName is the user's full name as held by the identity provider,
	// already normalized (see NormalizeDisplayName) and never empty. It is
	// the only copy of the name MotifPath keeps: anything that shows another
	// user's name reads it from here when the response is built.
	DisplayName string
	// Locale is this user's resolved locale preference, never LanguageCodeAny
	// (a user reads or speaks a language; they aren't language-agnostic
	// content). Every user has one: resolved at registration time in the
	// application layer, settable afterwards via UpdateLocale.
	Locale       Language
	RegisteredAt time.Time
}

// NewUser validates role and locale and constructs a User. Only student and
// teacher are self-registrable — admin is provisioned directly in the
// database and is never a valid input here, matching the OpenAPI spec's
// documented constraint and the "self-register as admin is rejected" Gherkin
// scenario. displayName is the raw name claimed by the identity provider;
// it is normalized here and rejected on field "name" when blank, so no user
// can exist without a name. locale must already be a resolved, non-empty Language code — the
// application layer is responsible for defaulting it (e.g. from
// Accept-Language) before calling this constructor.
func NewUser(id, clerkUserID string, role Role, displayName string, locale Language, registeredAt time.Time) (User, error) {
	var errs []FieldError

	switch role {
	case RoleStudent, RoleTeacher:
	case RoleAdmin:
		errs = append(errs, FieldError{Field: "role", Reason: "must be student or teacher"})
	default:
		errs = append(errs, FieldError{Field: "role", Reason: "must be student or teacher"})
	}

	name, ok := NormalizeDisplayName(displayName)
	if !ok {
		errs = append(errs, FieldError{Field: "name", Reason: "must not be blank"})
	}

	if locale.Code == "" {
		errs = append(errs, FieldError{Field: "locale", Reason: "must not be empty"})
	}

	if len(errs) > 0 {
		return User{}, &ValidationError{Fields: errs}
	}

	return User{
		ID:           id,
		ClerkUserID:  clerkUserID,
		Role:         role,
		DisplayName:  name,
		Locale:       locale,
		RegisteredAt: registeredAt,
	}, nil
}
