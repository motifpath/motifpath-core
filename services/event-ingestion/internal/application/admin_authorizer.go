package application

import (
	"context"
	"errors"

	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// adminRole is the role a caller's profile must carry to use the outbox admin
// endpoints. Per ADR-013 it is read from the Core Domain Service, never from a
// claim in the caller's JWT.
const adminRole = "admin"

// AdminAuthorizer is the single authorization seam for the outbox admin
// endpoints (ADR-013). Both RetryEntry and ResolveEntry gate through
// RequireAdmin, so the check has exactly one place to evolve when role gives
// way to a permission or group model.
type AdminAuthorizer struct {
	profiles ports.ProfileResolver
}

func NewAdminAuthorizer(profiles ports.ProfileResolver) *AdminAuthorizer {
	return &AdminAuthorizer{profiles: profiles}
}

// RequireAdmin resolves the caller's profile from the Core Domain Service and
// returns nil only when its role is the admin role. It returns
// domain.ErrForbidden when the caller is not an admin -- including a caller
// with no registered profile -- and domain.ErrAuthorizationUnavailable when
// the profile could not be established right now, so callers fail closed.
func (a *AdminAuthorizer) RequireAdmin(ctx context.Context, bearerToken string) error {
	profile, err := a.profiles.ResolveProfile(ctx, bearerToken)
	if err != nil {
		if errors.Is(err, ports.ErrIdentityNotRegistered) {
			return domain.ErrForbidden
		}
		return domain.ErrAuthorizationUnavailable
	}
	if profile.Role != adminRole {
		return domain.ErrForbidden
	}
	return nil
}
