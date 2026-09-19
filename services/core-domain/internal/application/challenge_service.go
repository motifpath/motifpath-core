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
	exercises  ports.ExerciseRepository
	newID      func() string
	now        func() time.Time
}

func NewChallengeService(nodes ports.ContentNodeRepository, challenges ports.ChallengeRepository, exercises ports.ExerciseRepository, newID func() string, now func() time.Time) *ChallengeService {
	return &ChallengeService{nodes: nodes, challenges: challenges, exercises: exercises, newID: newID, now: now}
}

// CreateChallenge creates a challenge attached to contentNodeID. Only
// teachers and admins may create challenges.
func (s *ChallengeService) CreateChallenge(ctx context.Context, caller domain.User, contentNodeID, subjectTag string, passThreshold int, timeThresholdMS *int, shuffleExercises, shuffleOptions bool) (domain.Challenge, error) {
	if !canManageContent(caller.Role) {
		return domain.Challenge{}, domain.ErrForbidden
	}

	if _, err := s.nodes.GetByID(ctx, contentNodeID); err != nil {
		return domain.Challenge{}, err
	}

	challenge, err := domain.NewChallenge(s.newID(), contentNodeID, subjectTag, passThreshold, timeThresholdMS, shuffleExercises, shuffleOptions, s.now())
	if err != nil {
		return domain.Challenge{}, err
	}
	if err := s.challenges.Create(ctx, challenge); err != nil {
		return domain.Challenge{}, err
	}
	// A brand-new challenge can't have any linked exercises yet, so
	// resolveTimeThreshold's lookup would always resolve to "no exercises
	// linked" — skip straight to that outcome instead of paying for a
	// guaranteed-empty round trip.
	return challenge, nil
}

// GetChallenge returns the challenge with the given id. Any authenticated
// user may retrieve a challenge.
func (s *ChallengeService) GetChallenge(ctx context.Context, id string) (domain.Challenge, error) {
	challenge, err := s.challenges.GetByID(ctx, id)
	if err != nil {
		return domain.Challenge{}, err
	}
	return s.resolveTimeThreshold(ctx, challenge)
}

// ListChallengesForContentNode returns the challenges attached to
// contentNodeID, or domain.ErrNotFound if no such content node exists. Any
// authenticated user may list a node's challenges.
func (s *ChallengeService) ListChallengesForContentNode(ctx context.Context, contentNodeID string) ([]domain.Challenge, error) {
	if _, err := s.nodes.GetByID(ctx, contentNodeID); err != nil {
		return nil, err
	}
	challenges, err := s.challenges.ListByContentNodeID(ctx, contentNodeID)
	if err != nil {
		return nil, err
	}
	return s.resolveTimeThresholds(ctx, challenges)
}

// UpdateChallenge replaces the given challenge's subject tag, pass
// threshold, time threshold override, and shuffle flags. The challenge's
// linked exercises are untouched. Only the teacher who created the
// challenge's content node, or an admin, may update it. Returns
// domain.ErrNotFound if no challenge exists with the given id.
func (s *ChallengeService) UpdateChallenge(ctx context.Context, caller domain.User, id, subjectTag string, passThreshold int, timeThresholdMS *int, shuffleExercises, shuffleOptions bool) (domain.Challenge, error) {
	if !canManageContent(caller.Role) {
		return domain.Challenge{}, domain.ErrForbidden
	}

	existing, err := s.challenges.GetByID(ctx, id)
	if err != nil {
		return domain.Challenge{}, err
	}
	node, err := s.nodes.GetByID(ctx, existing.ContentNodeID)
	if err != nil {
		return domain.Challenge{}, err
	}
	if err := requireOwner(caller, node.TeacherID); err != nil {
		return domain.Challenge{}, err
	}

	updated, err := existing.Update(subjectTag, passThreshold, timeThresholdMS, shuffleExercises, shuffleOptions)
	if err != nil {
		return domain.Challenge{}, err
	}

	if err := s.challenges.Update(ctx, updated); err != nil {
		return domain.Challenge{}, err
	}
	return s.resolveTimeThreshold(ctx, updated)
}

// resolveTimeThreshold returns a copy of challenge with TimeThresholdMS set
// to the value a caller should actually see: the teacher's explicit
// override if one was stored, otherwise the sum of the challenge's
// currently linked exercises' EstimatedDurationSeconds converted to
// milliseconds (an exercise with no estimate contributes 0), or nil if the
// challenge has no linked exercises to sum. Never persisted — recomputed on
// every call so it always reflects the challenge's current exercise links.
func (s *ChallengeService) resolveTimeThreshold(ctx context.Context, challenge domain.Challenge) (domain.Challenge, error) {
	if challenge.TimeThresholdMS != nil {
		return challenge, nil
	}

	linked, err := s.exercises.ListByChallengeID(ctx, challenge.ID)
	if err != nil {
		return domain.Challenge{}, err
	}
	if len(linked) == 0 {
		return challenge, nil
	}

	sum := sumEstimatedDurationMS(linked)
	challenge.TimeThresholdMS = &sum
	return challenge, nil
}

// sumEstimatedDurationMS sums exercises' EstimatedDurationSeconds, converted
// to milliseconds (an exercise with no estimate contributes 0).
func sumEstimatedDurationMS(exercises []domain.Exercise) int {
	sum := 0
	for _, ex := range exercises {
		if ex.EstimatedDurationSeconds != nil {
			sum += *ex.EstimatedDurationSeconds * 1000
		}
	}
	return sum
}

// resolveTimeThresholds is resolveTimeThreshold applied to a whole list,
// batching the exercise lookup into a single ListByChallengeIDs call across
// every challenge that lacks an explicit override, instead of one
// ListByChallengeID round trip per challenge.
func (s *ChallengeService) resolveTimeThresholds(ctx context.Context, challenges []domain.Challenge) ([]domain.Challenge, error) {
	needsResolution := make([]string, 0, len(challenges))
	for _, c := range challenges {
		if c.TimeThresholdMS == nil {
			needsResolution = append(needsResolution, c.ID)
		}
	}

	// Every challenge already carries an explicit override — skip the
	// lookup outright rather than relying on ListByChallengeIDs happening
	// to no-op on empty input, an implementation detail of one adapter.
	var linkedByChallenge map[string][]domain.Exercise
	if len(needsResolution) > 0 {
		var err error
		linkedByChallenge, err = s.exercises.ListByChallengeIDs(ctx, needsResolution)
		if err != nil {
			return nil, err
		}
	}

	result := make([]domain.Challenge, len(challenges))
	for i, c := range challenges {
		if c.TimeThresholdMS != nil {
			result[i] = c
			continue
		}
		linked := linkedByChallenge[c.ID]
		if len(linked) == 0 {
			result[i] = c
			continue
		}
		sum := sumEstimatedDurationMS(linked)
		c.TimeThresholdMS = &sum
		result[i] = c
	}
	return result, nil
}
