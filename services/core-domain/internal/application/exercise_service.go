package application

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ExerciseService manages Exercise — a reusable, standalone practice item
// that may be linked to any number of Challenges.
type ExerciseService struct {
	challenges ports.ChallengeRepository
	exercises  ports.ExerciseRepository
	newID      func() string
	now        func() time.Time
}

func NewExerciseService(challenges ports.ChallengeRepository, exercises ports.ExerciseRepository, newID func() string, now func() time.Time) *ExerciseService {
	return &ExerciseService{challenges: challenges, exercises: exercises, newID: newID, now: now}
}

// CreateExercise creates a standalone exercise, not linked to any challenge.
// Only teachers and admins may create exercises.
func (s *ExerciseService) CreateExercise(ctx context.Context, caller domain.User, title, prompt string, exerciseType domain.ExerciseType, skillTags []string, imageURL, audioURL *string, options []domain.Option) (domain.Exercise, error) {
	if !canManageContent(caller.Role) {
		return domain.Exercise{}, domain.ErrForbidden
	}

	exercise, err := domain.NewExercise(s.newID(), title, prompt, exerciseType, skillTags, imageURL, audioURL, options, s.now())
	if err != nil {
		return domain.Exercise{}, err
	}
	if err := s.exercises.Create(ctx, exercise); err != nil {
		return domain.Exercise{}, err
	}
	return exercise, nil
}

// GetExercise returns the exercise with the given id. Any authenticated user
// may retrieve an exercise.
func (s *ExerciseService) GetExercise(ctx context.Context, id string) (domain.Exercise, error) {
	return s.exercises.GetByID(ctx, id)
}

// LinkExerciseToChallenge links an existing exercise into a challenge. Only
// teachers and admins may link exercises. Returns domain.ErrAlreadyExists if
// the exercise is already linked to the challenge.
func (s *ExerciseService) LinkExerciseToChallenge(ctx context.Context, caller domain.User, challengeID, exerciseID string) (domain.Exercise, error) {
	if !canManageContent(caller.Role) {
		return domain.Exercise{}, domain.ErrForbidden
	}

	if _, err := s.challenges.GetByID(ctx, challengeID); err != nil {
		return domain.Exercise{}, err
	}
	exercise, err := s.exercises.GetByID(ctx, exerciseID)
	if err != nil {
		return domain.Exercise{}, err
	}
	for _, id := range exercise.ChallengeIDs {
		if id == challengeID {
			return domain.Exercise{}, domain.ErrAlreadyExists
		}
	}

	if err := s.exercises.LinkChallenge(ctx, exerciseID, challengeID); err != nil {
		return domain.Exercise{}, err
	}
	return s.exercises.GetByID(ctx, exerciseID)
}

// UnlinkExerciseFromChallenge removes the link between an exercise and a
// challenge. Only teachers and admins may unlink exercises. Returns
// domain.ErrNotFound if the exercise is not currently linked to the
// challenge.
func (s *ExerciseService) UnlinkExerciseFromChallenge(ctx context.Context, caller domain.User, challengeID, exerciseID string) error {
	if !canManageContent(caller.Role) {
		return domain.ErrForbidden
	}

	if _, err := s.challenges.GetByID(ctx, challengeID); err != nil {
		return err
	}
	exercise, err := s.exercises.GetByID(ctx, exerciseID)
	if err != nil {
		return err
	}
	linked := false
	for _, id := range exercise.ChallengeIDs {
		if id == challengeID {
			linked = true
			break
		}
	}
	if !linked {
		return domain.ErrNotFound
	}

	return s.exercises.UnlinkChallenge(ctx, exerciseID, challengeID)
}
