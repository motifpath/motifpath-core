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
	ID           string
	ClerkUserID  string
	Role         Role
	RegisteredAt time.Time
}

// NewUser validates role and constructs a User. Only student and teacher are
// self-registrable — admin is provisioned directly in the database and is
// never a valid input here, matching the OpenAPI spec's documented
// constraint and the "self-register as admin is rejected" Gherkin scenario.
func NewUser(id, clerkUserID string, role Role, registeredAt time.Time) (User, error) {
	switch role {
	case RoleStudent, RoleTeacher:
	case RoleAdmin:
		return User{}, NewValidationError("role", "must be student or teacher")
	default:
		return User{}, NewValidationError("role", "must be student or teacher")
	}

	return User{
		ID:           id,
		ClerkUserID:  clerkUserID,
		Role:         role,
		RegisteredAt: registeredAt,
	}, nil
}
