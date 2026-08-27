package application

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// IdentityService maps Clerk identities to MotifPath User records.
// GetProfile doubles as the "resolve caller" lookup every other application
// service depends on: the HTTP handler layer calls it once per request to
// turn the JWT's Clerk sub claim into a domain.User carrying the real
// user_id and role, then passes that resolved User into whichever service
// handles the request. This is the first real implementation of User.role
// in the codebase — event-ingestion's admin endpoints currently fake role
// via a Clerk JWT custom claim specifically because this service didn't
// exist yet (ADR-012 Part 3 flags that as reconciliation debt).
type IdentityService struct {
	users ports.UserRepository
	newID func() string
	now   func() time.Time
}

func NewIdentityService(users ports.UserRepository, newID func() string, now func() time.Time) *IdentityService {
	return &IdentityService{users: users, newID: newID, now: now}
}

// RegisterUser creates a new user for clerkUserID. Returns
// domain.ErrAlreadyExists if a user record already exists for that Clerk
// identity — registration is callable once per identity.
func (s *IdentityService) RegisterUser(ctx context.Context, clerkUserID string, role domain.Role) (domain.User, error) {
	user, err := domain.NewUser(s.newID(), clerkUserID, role, s.now())
	if err != nil {
		return domain.User{}, err
	}
	if err := s.users.Create(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

// GetProfile returns the user registered for clerkUserID. Returns
// domain.ErrNotFound if the identity has never registered.
func (s *IdentityService) GetProfile(ctx context.Context, clerkUserID string) (domain.User, error) {
	return s.users.GetByClerkUserID(ctx, clerkUserID)
}
