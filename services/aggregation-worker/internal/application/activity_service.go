package application

import (
	"context"
	"time"

	"github.com/motifpath/aggregation-worker/internal/domain"
	"github.com/motifpath/aggregation-worker/internal/ports"
)

// ActivityService keeps the student's raw activity: every practice session's
// start, answers and end, and every content node completed, each with its time.
// Practice days, learning days and any later measure, such as a streak, are
// evaluated from it when read, so none of them needs new tracking.
type ActivityService struct {
	sessions ports.PracticeSessionRepository
	learning ports.LearningActivityRepository
}

func NewActivityService(sessions ports.PracticeSessionRepository, learning ports.LearningActivityRepository) *ActivityService {
	return &ActivityService{sessions: sessions, learning: learning}
}

// SessionStarted records a session's start and its plan.
func (s *ActivityService) SessionStarted(ctx context.Context, e domain.PracticeSessionStart) error {
	return s.update(ctx, e.StudentID, e.PracticeSessionID, func(p domain.PracticeSession) domain.PracticeSession {
		return p.Started(e)
	})
}

// ItemAnswered records that the student answered in a session at the given time,
// whether or not the answer could be graded: either way they were practising.
func (s *ActivityService) ItemAnswered(ctx context.Context, studentID, sessionID string, at time.Time) error {
	return s.update(ctx, studentID, sessionID, func(p domain.PracticeSession) domain.PracticeSession {
		p.ID, p.StudentID = sessionID, studentID
		return p.Answered(at)
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

func (s *ActivityService) update(ctx context.Context, studentID, sessionID string, apply func(domain.PracticeSession) domain.PracticeSession) error {
	current, _, err := s.sessions.Get(ctx, studentID, sessionID)
	if err != nil {
		return err
	}
	return s.sessions.Put(ctx, apply(current))
}
