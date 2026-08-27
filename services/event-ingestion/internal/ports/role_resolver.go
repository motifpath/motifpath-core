package ports

import (
	"context"
	"errors"
)

var (
	// ErrIdentityNotRegistered is returned by a RoleResolver when the bearer
	// token is valid but maps to no registered MotifPath profile. The caller
	// exists to the identity provider but not to the platform, so no role can
	// be assigned.
	ErrIdentityNotRegistered = errors.New("caller has no registered profile")

	// ErrRoleUnavailable is returned by a RoleResolver when the caller's role
	// could not be established right now -- the identity/authorization
	// capability was unreachable, timed out, or answered in a way the resolver
	// could not interpret. It is a retryable condition, distinct from a
	// definitive "not registered" answer.
	ErrRoleUnavailable = errors.New("caller role could not be established")
)

// RoleResolver returns the platform role of the caller identified by a bearer
// token, by asking the identity/authorization capability (the Core Domain
// Service, per ADR-013) rather than trusting a role claim in the token itself.
type RoleResolver interface {
	// ResolveRole returns the caller's role (e.g. "student", "teacher",
	// "admin"). It returns ErrIdentityNotRegistered when the token maps to no
	// registered profile, and ErrRoleUnavailable when the role could not be
	// established at this time.
	ResolveRole(ctx context.Context, bearerToken string) (string, error)
}
