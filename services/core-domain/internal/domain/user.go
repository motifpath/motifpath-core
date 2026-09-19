package domain

import "time"

// Role is a User's platform role. It is immutable after registration.
type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
)

// User is a registered MotifPath identity, mapped 1:1 to a Clerk identity
// via ClerkUserID.
type User struct {
	ID          string
	ClerkUserID string
	Role        Role
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
// scenario. locale must already be a resolved, non-empty Language code — the
// application layer is responsible for defaulting it (e.g. from
// Accept-Language) before calling this constructor.
func NewUser(id, clerkUserID string, role Role, locale Language, registeredAt time.Time) (User, error) {
	var errs []FieldError

	switch role {
	case RoleStudent, RoleTeacher:
	case RoleAdmin:
		errs = append(errs, FieldError{Field: "role", Reason: "must be student or teacher"})
	default:
		errs = append(errs, FieldError{Field: "role", Reason: "must be student or teacher"})
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
		Locale:       locale,
		RegisteredAt: registeredAt,
	}, nil
}
