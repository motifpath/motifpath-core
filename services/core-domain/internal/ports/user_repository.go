package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// UserRepository persists User records.
type UserRepository interface {
	// Create persists a new user. Returns domain.ErrAlreadyExists if a user
	// already exists for user.ClerkUserID.
	Create(ctx context.Context, user domain.User) error

	// GetByClerkUserID returns the user registered for the given Clerk
	// identity. Returns domain.ErrNotFound if none exists.
	GetByClerkUserID(ctx context.Context, clerkUserID string) (domain.User, error)

	// GetByID returns the user with the given MotifPath user_id. Returns
	// domain.ErrNotFound if none exists.
	GetByID(ctx context.Context, id string) (domain.User, error)
}
