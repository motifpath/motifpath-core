package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// ExerciseRepository persists Exercise records, including their many-to-many
// links to Challenge and to ContentNode (as a path exercise).
type ExerciseRepository interface {
	Create(ctx context.Context, exercise domain.Exercise) error

	// GetByID returns domain.ErrNotFound if no exercise exists with the
	// given id. The returned Exercise's ChallengeIDs and ContentNodeIDs
	// reflect its current links.
	GetByID(ctx context.Context, id string) (domain.Exercise, error)

	// LinkChallenge links exerciseID into challengeID. Callers are
	// responsible for checking both ids exist and are not already linked
	// before calling — this method assumes the link is new.
	LinkChallenge(ctx context.Context, exerciseID, challengeID string) error

	// UnlinkChallenge removes the link between exerciseID and challengeID.
	// Callers are responsible for checking the link exists before calling.
	UnlinkChallenge(ctx context.Context, exerciseID, challengeID string) error

	// ListByChallengeID returns the exercises linked to challengeID, in
	// link order, or an empty slice if it has none. Does not itself verify
	// the challenge exists — callers check that separately. Randomizing the
	// order per the challenge's shuffle settings is an application-layer
	// concern.
	ListByChallengeID(ctx context.Context, challengeID string) ([]domain.Exercise, error)

	// LinkContentNode links exerciseID into contentNodeID as a path
	// exercise. Callers are responsible for checking both ids exist and are
	// not already linked before calling.
	LinkContentNode(ctx context.Context, exerciseID, contentNodeID string) error

	// UnlinkContentNode removes the path-exercise link between exerciseID
	// and contentNodeID. Callers are responsible for checking the link
	// exists before calling.
	UnlinkContentNode(ctx context.Context, exerciseID, contentNodeID string) error

	// ListByContentNodeID returns the path exercises linked to
	// contentNodeID, always in link order, or an empty slice if it has
	// none. Does not itself verify the content node exists.
	ListByContentNodeID(ctx context.Context, contentNodeID string) ([]domain.Exercise, error)

	// ListBySkillTag returns every exercise carrying skillTag among its
	// skill tags, in no particular order — the practice-session pool for
	// that skill. Selecting and randomizing a subset is an
	// application-layer concern.
	ListBySkillTag(ctx context.Context, skillTag string) ([]domain.Exercise, error)

	// List returns exercises from the whole pool, optionally narrowed by
	// skillTag and/or exerciseType — an empty string on either means no
	// filter on that dimension. Order is stable but otherwise unspecified.
	List(ctx context.Context, skillTag string, exerciseType domain.ExerciseType) ([]domain.Exercise, error)

	// Update replaces exercise's title, prompt, skill_tags, image_url,
	// audio_url, options, and estimated_duration_seconds. exercise.ID
	// identifies which row to update; exercise.ExerciseType,
	// exercise.ChallengeIDs, and exercise.ContentNodeIDs are not applied —
	// exercise_type cannot change after creation and links are managed
	// exclusively through LinkChallenge/UnlinkChallenge and
	// LinkContentNode/UnlinkContentNode. Returns domain.ErrNotFound if no
	// exercise exists with the given id.
	Update(ctx context.Context, exercise domain.Exercise) error
}
