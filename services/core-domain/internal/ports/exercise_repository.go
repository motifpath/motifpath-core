package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ExerciseRepository persists Exercise records, including their many-to-many
// links to Challenge.
type ExerciseRepository interface {
	Create(ctx context.Context, exercise domain.Exercise) error

	// GetByID returns domain.ErrNotFound if no exercise exists with the
	// given id. The returned Exercise's ChallengeIDs reflects its current
	// links.
	GetByID(ctx context.Context, id string) (domain.Exercise, error)

	// LinkChallenge links exerciseID into challengeID. Callers are
	// responsible for checking both ids exist and are not already linked
	// before calling — this method assumes the link is new.
	LinkChallenge(ctx context.Context, exerciseID, challengeID string) error

	// UnlinkChallenge removes the link between exerciseID and challengeID.
	// Callers are responsible for checking the link exists before calling.
	UnlinkChallenge(ctx context.Context, exerciseID, challengeID string) error
}
