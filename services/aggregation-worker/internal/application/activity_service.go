package application

import (
	"context"
	"time"

	"github.com/motifpath/aggregation-worker/internal/domain"
	"github.com/motifpath/aggregation-worker/internal/ports"
)

// ActivityService keeps the student's raw activity: every practice session's
// start, answers and end, every content node completed and every tap check, each
// with its time.
// Practice days, learning days and any later measure, such as a streak, are
// evaluated from it when read, so none of them needs new tracking.
type ActivityService struct {
	sessions  ports.PracticeSessionRepository
	learning  ports.LearningActivityRepository
	tapChecks ports.TapCheckRepository
}

func NewActivityService(sessions ports.PracticeSessionRepository, learning ports.LearningActivityRepository, tapChecks ports.TapCheckRepository) *ActivityService {
	return &ActivityService{sessions: sessions, learning: learning, tapChecks: tapChecks}
}

// SessionStarted records a session's start and its plan.
func (s *ActivityService) SessionStarted(ctx context.Context, e domain.PracticeSessionStart) error {
	return s.update(ctx, e.StudentID, e.PracticeSessionID, func(p domain.PracticeSession) domain.PracticeSession {
		return p.Started(e)
	})
}

// ItemAnswered records that the student answered in a session at the given time,
// whether or not the answer could be graded: either way they were practising.
// template is the timed drill the answer practised, empty when it practised none
// or couldn't be graded.
func (s *ActivityService) ItemAnswered(ctx context.Context, studentID, sessionID string, at time.Time, template string) error {
	return s.update(ctx, studentID, sessionID, func(p domain.PracticeSession) domain.PracticeSession {
		p.ID, p.StudentID = sessionID, studentID
		return p.Answered(at, template)
	})
}

// SessionEnded records a session's end.
func (s *ActivityService) SessionEnded(ctx context.Context, e domain.PracticeSessionEnd) error {
	return s.update(ctx, e.StudentID, e.PracticeSessionID, func(p domain.PracticeSession) domain.PracticeSession {
		return p.Ended(e)
	})
}

// LessonCompleted keeps one completion of a content node; a redelivered event
// is kept once.
func (s *ActivityService) LessonCompleted(ctx context.Context, a domain.LearningActivity) error {
	_, err := s.learning.Insert(ctx, a)
	return err
}

// TapCheckCompleted keeps one tap check; a redelivered event is kept once.
func (s *ActivityService) TapCheckCompleted(ctx context.Context, c domain.TapCheck) error {
	_, err := s.tapChecks.Insert(ctx, c)
	return err
}

func (s *ActivityService) update(ctx context.Context, studentID, sessionID string, apply func(domain.PracticeSession) domain.PracticeSession) error {
	current, _, err := s.sessions.Get(ctx, studentID, sessionID)
	if err != nil {
		return err
	}
	return s.sessions.Put(ctx, apply(current))
}
