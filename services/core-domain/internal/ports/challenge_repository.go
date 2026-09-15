package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ChallengeRepository persists Challenge records.
type ChallengeRepository interface {
	Create(ctx context.Context, challenge domain.Challenge) error

	// GetByID returns domain.ErrNotFound if no challenge exists with the
	// given id.
	GetByID(ctx context.Context, id string) (domain.Challenge, error)

	// ListByContentNodeID returns the challenges attached to contentNodeID,
	// or an empty slice if it has none. Does not itself verify the content
	// node exists — callers check that separately.
	ListByContentNodeID(ctx context.Context, contentNodeID string) ([]domain.Challenge, error)
}
