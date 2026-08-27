package application

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ChallengeService manages Challenge and Exercise — the assessment units
// attached to content nodes and the practice items within them.
type ChallengeService struct {
	nodes      ports.ContentNodeRepository
	challenges ports.ChallengeRepository
	exercises  ports.ExerciseRepository
	newID      func() string
	now        func() time.Time
}

func NewChallengeService(nodes ports.ContentNodeRepository, challenges ports.ChallengeRepository, exercises ports.ExerciseRepository, newID func() string, now func() time.Time) *ChallengeService {
	return &ChallengeService{nodes: nodes, challenges: challenges, exercises: exercises, newID: newID, now: now}
}

// CreateChallenge creates a challenge attached to contentNodeID. Only
// teachers and admins may create challenges.
func (s *ChallengeService) CreateChallenge(ctx context.Context, caller domain.User, contentNodeID, subjectTag string, passThreshold int, remediationTarget *string) (domain.Challenge, error) {
	if !canManageContent(caller.Role) {
		return domain.Challenge{}, domain.ErrForbidden
	}

	if _, err := s.nodes.GetByID(ctx, contentNodeID); err != nil {
		return domain.Challenge{}, err
	}

	challenge, err := domain.NewChallenge(s.newID(), contentNodeID, subjectTag, passThreshold, remediationTarget, s.now())
	if err != nil {
		return domain.Challenge{}, err
	}
	if err := s.challenges.Create(ctx, challenge); err != nil {
		return domain.Challenge{}, err
	}
	return challenge, nil
}

// GetChallenge returns the challenge with the given id. Any authenticated
// user may retrieve a challenge.
func (s *ChallengeService) GetChallenge(ctx context.Context, id string) (domain.Challenge, error) {
	return s.challenges.GetByID(ctx, id)
}

// CreateExercise creates a pre-defined exercise within challengeID. Only
// teachers and admins may create exercises.
func (s *ChallengeService) CreateExercise(ctx context.Context, caller domain.User, challengeID string, exerciseType domain.ExerciseType, prompt string) (domain.Exercise, error) {
	if !canManageContent(caller.Role) {
		return domain.Exercise{}, domain.ErrForbidden
	}

	if _, err := s.challenges.GetByID(ctx, challengeID); err != nil {
		return domain.Exercise{}, err
	}

	exercise, err := domain.NewExercise(s.newID(), challengeID, exerciseType, prompt, s.now())
	if err != nil {
		return domain.Exercise{}, err
	}
	if err := s.exercises.Create(ctx, exercise); err != nil {
		return domain.Exercise{}, err
	}
	return exercise, nil
}

// GetExercise returns the exercise with the given id. Any authenticated
// user may retrieve an exercise.
func (s *ChallengeService) GetExercise(ctx context.Context, id string) (domain.Exercise, error) {
	return s.exercises.GetByID(ctx, id)
}
