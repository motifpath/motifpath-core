package application

import (
	"context"

	"github.com/motifpath/aggregation-worker/internal/domain"
	"github.com/motifpath/aggregation-worker/internal/ports"
)

// ProcessEventService derives per-student, per-content-node completion status
// from lesson-family tracking events, per ADR-011. Exercise-family events (and
// any other event_type) are accepted without error but produce no state
// change — this worker's MVP scope stops at node-completion status.
type ProcessEventService struct {
	repo ports.CompletionStateRepository
}

func NewProcessEventService(repo ports.CompletionStateRepository) *ProcessEventService {
	return &ProcessEventService{repo: repo}
}

// Handle applies ADR-011's transition rule to event and persists the result if
// it changed. Re-processing the same event (at-least-once Kafka delivery) is
// safe: recomputing NextStatus from the same current value yields the same
// next value, so the repeated Upsert is a no-op in effect.
func (s *ProcessEventService) Handle(ctx context.Context, event domain.TrackingEvent) error {
	if !domain.IsLessonEvent(event.EventType) {
		return nil
	}

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
