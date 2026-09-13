package application

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ChallengeService manages Challenge — the assessment unit attached to
// content nodes. Exercise management (standalone create/retrieve and
// challenge linking) lives on ExerciseService.
type ChallengeService struct {
	nodes      ports.ContentNodeRepository
	challenges ports.ChallengeRepository
	newID      func() string
	now        func() time.Time
}

func NewChallengeService(nodes ports.ContentNodeRepository, challenges ports.ChallengeRepository, newID func() string, now func() time.Time) *ChallengeService {
	return &ChallengeService{nodes: nodes, challenges: challenges, newID: newID, now: now}
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
