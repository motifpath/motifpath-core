package application

import (
	"context"

	"github.com/motifpath/aggregation-worker/internal/domain"
	"github.com/motifpath/aggregation-worker/internal/ports"
)

// ProcessEventService routes each tracking event to what it changes: lesson
// events derive per-student, per-content-node completion status; practice events
// feed the evidence processor; completions and practice sessions are kept as raw
// activity. Any other event_type is accepted without error and changes nothing.
type ProcessEventService struct {
	repo     ports.CompletionStateRepository
	practice *PracticeEvidenceService
	activity *ActivityService
}

func NewProcessEventService(repo ports.CompletionStateRepository, practice *PracticeEvidenceService, activity *ActivityService) *ProcessEventService {
	return &ProcessEventService{repo: repo, practice: practice, activity: activity}
}

// Handle routes event to what it changes. Re-processing the same event
// (at-least-once Kafka delivery) is safe for every route.
func (s *ProcessEventService) Handle(ctx context.Context, event domain.TrackingEvent) error {
	switch event.EventType {
	case domain.EventTypePracticeItemAnswered:
		return s.handleAnswer(ctx, event)
	case domain.EventTypePracticeSessionStarted:
		if event.SessionStart == nil {
			return nil
		}
		return s.activity.SessionStarted(ctx, *event.SessionStart)
	case domain.EventTypePracticeSessionEnded:
		if event.SessionEnd == nil {
			return nil
		}
		return s.activity.SessionEnded(ctx, *event.SessionEnd)
	case domain.EventTypeLessonCompleted:
		if err := s.activity.LessonCompleted(ctx, domain.LearningActivity{
			EventID:       event.EventID,
			StudentID:     event.StudentID,
			ContentNodeID: event.ContentNodeID,
			CompletedAt:   event.OccurredAt,
		}); err != nil {
			return err
		}
		return s.updateCompletion(ctx, event)
	case domain.EventTypeLessonStarted, domain.EventTypeLessonResumed:
		return s.updateCompletion(ctx, event)
	}
	return nil
}

// updateCompletion applies the completion transition rule to event and persists
// the result if it changed. Re-processing the same event is safe: recomputing
// NextStatus from the same current value yields the same next value, so the
// repeated Upsert is a no-op in effect.
func (s *ProcessEventService) updateCompletion(ctx context.Context, event domain.TrackingEvent) error {
	current, found, err := s.repo.GetStatus(ctx, event.StudentID, event.ContentNodeID)
	if err != nil {
		return err
	}
	if !found {
		current = domain.CompletionStatusNotStarted
	}

	next := domain.NextStatus(current, event.EventType)
	if next == current {
		return nil
	}

	return s.repo.Upsert(ctx, event.StudentID, event.ContentNodeID, next)
}

// handleAnswer grades the answer, then records it in its session, if it was given
// in one, with the timed drill it practised.
func (s *ProcessEventService) handleAnswer(ctx context.Context, event domain.TrackingEvent) error {
	var template string
	if event.PracticeAnswer != nil {
		var err error
		if template, err = s.practice.process(ctx, *event.PracticeAnswer); err != nil {
			return err
		}
	}
	if event.PracticeSessionID == "" {
		return nil
	}
	return s.activity.ItemAnswered(ctx, event.StudentID, event.PracticeSessionID, event.OccurredAt, template)
}
