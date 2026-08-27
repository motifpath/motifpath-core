package ports

import (
	"context"
	"errors"
)

var (
	// ErrIdentityNotRegistered is returned when the bearer token is valid but
	// maps to no registered MotifPath profile. The caller exists to the
	// identity provider but not to the platform.
	ErrIdentityNotRegistered = errors.New("caller has no registered profile")

	// ErrProfileUnavailable is returned when the caller's profile could not be
	// established right now -- the Core Domain Service was unreachable, timed
	// out, or answered in a way the resolver could not interpret. It is a
	// retryable condition, distinct from a definitive "not registered" answer.
	ErrProfileUnavailable = errors.New("caller profile could not be established")
)

// Profile is the subset of a caller's Core Domain Service user record the
// Event Ingestion Service needs: the MotifPath user id every downstream
// consumer keys on, and the platform role the admin endpoints gate on.
type Profile struct {
	UserID string
	Role   string
}

// ProfileResolver returns the MotifPath Profile of the caller identified by a
// bearer token, by asking the Core Domain Service (GET /users/me) rather than
// trusting claims in the token itself (ADR-013, ADR-014).
type ProfileResolver interface {
	// ResolveProfile returns the caller's profile. It returns
	// ErrIdentityNotRegistered when the token maps to no registered profile,
	// and ErrProfileUnavailable when the profile could not be established now.
	ResolveProfile(ctx context.Context, bearerToken string) (Profile, error)
}

// IdentityResolver returns the caller's MotifPath user id. Implementations
// cache the result keyed on the token's subject claim: the sub -> user_id
// mapping is a registration-time invariant (ADR-007), so a cached answer is
// never stale, and the /events hot path makes no network call once warm
// (ADR-014).
type IdentityResolver interface {
	ResolveUserID(ctx context.Context, sub, bearerToken string) (string, error)
}
