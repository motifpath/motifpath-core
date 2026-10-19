package application

import (
	"context"
	"time"

	"github.com/motifpath/aggregation-worker/internal/domain"
	"github.com/motifpath/aggregation-worker/internal/ports"
)

// ActivityService keeps the student's raw activity: every practice session's
// start, answers and end, every content node completed, every tap check and every
// song chart marked as played, each with its time, and when each student was
// last active.
// Practice days, learning days and any later measure, such as a streak, are
// evaluated from it when read, so none of them needs new tracking.
type ActivityService struct {
	sessions  ports.PracticeSessionRepository
	learning  ports.LearningActivityRepository
	tapChecks ports.TapCheckRepository
	songs     ports.SongChartCompletionRepository
	last      ports.LastActivityRepository
	now       func() time.Time
}

func NewActivityService(
	sessions ports.PracticeSessionRepository,
	learning ports.LearningActivityRepository,
	tapChecks ports.TapCheckRepository,
	songs ports.SongChartCompletionRepository,
	last ports.LastActivityRepository,
	now func() time.Time,
) *ActivityService {
	return &ActivityService{sessions: sessions, learning: learning, tapChecks: tapChecks, songs: songs, last: last, now: now}
}

// Active records that the student did something at the given time; an earlier
// time than their last activity changes nothing. The time comes from the
// student's device, so one later than now is taken as now: last activity never
// moves back, and a device clock running ahead would otherwise pin it in the
// future for good.
func (s *ActivityService) Active(ctx context.Context, studentID string, at time.Time) error {
	return s.last.Touch(ctx, studentID, minTime(at, s.now()))
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// SessionStarted records a session's start and its plan.
func (s *ActivityService) SessionStarted(ctx context.Context, e domain.PracticeSessionStart) error {
	return s.update(ctx, e.StudentID, e.PracticeSessionID, func(p domain.PracticeSession) domain.PracticeSession {
		return p.Started(e)
	})
}

// ItemAnswered records the answer eventID given in a session at the given time,
// whether or not the answer could be graded: either way they were practising.
// template is the timed drill the answer practised, empty when it practised none
// or couldn't be graded.
func (s *ActivityService) ItemAnswered(ctx context.Context, studentID, sessionID, eventID string, at time.Time, template string) error {
	return s.update(ctx, studentID, sessionID, func(p domain.PracticeSession) domain.PracticeSession {
		p.ID, p.StudentID = sessionID, studentID
		return p.Answered(eventID, at, template)
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

// SongChartCompleted keeps one time a song chart was marked as played; a
// redelivered event is kept once.
func (s *ActivityService) SongChartCompleted(ctx context.Context, c domain.SongChartCompletion) error {
	_, err := s.songs.Insert(ctx, c)
	return err
}

func (s *ActivityService) update(ctx context.Context, studentID, sessionID string, apply func(domain.PracticeSession) domain.PracticeSession) error {
	current, _, err := s.sessions.Get(ctx, studentID, sessionID)
	if err != nil {
		return err
	}
	return s.sessions.Put(ctx, apply(current))
}
