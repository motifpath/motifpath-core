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

	// UpdateLocale sets the locale (a Language.Code) on the user with the
	// given id. Returns domain.ErrNotFound if no such user exists.
	UpdateLocale(ctx context.Context, id, locale string) error

	// UpdateDisplayName sets the display name of the user with the given
	// id. displayName is already normalized. Returns domain.ErrNotFound if
	// no such user exists.
	UpdateDisplayName(ctx context.Context, id, displayName string) error

	// GetDisplayNames returns the display name of every user in ids that
	// exists, keyed by user_id, in a single query. Ids with no user are
	// simply absent from the result.
	GetDisplayNames(ctx context.Context, ids []string) (map[string]string, error)
}
