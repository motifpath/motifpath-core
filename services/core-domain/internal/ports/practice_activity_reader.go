package ports

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
)

// PracticeActivityReader reads a student's raw practice and learning
// activity from the Aggregation Worker's MongoDB collections:
// `practice_sessions`, `learning_activity` and `practice_item_history`. It
// is read-only from this service's perspective — the worker is the only
// writer.
type PracticeActivityReader interface {
	// FinishedSessions returns the student's practice sessions that ended
	// at or after since without leaving early.
	FinishedSessions(ctx context.Context, studentID string, since time.Time) ([]domain.FinishedPracticeSession, error)

	// CompletionTimes returns when the student completed a content node,
	// once per completion, at or after since.
	CompletionTimes(ctx context.Context, studentID string, since time.Time) ([]time.Time, error)

	// SnapshotsAt returns the student's state on each of itemKeys as it
	// stood at at: the latest daily snapshot of a day that had ended by
	// then. An item not practised by then is absent from the result.
	SnapshotsAt(ctx context.Context, studentID string, itemKeys []string, at time.Time) (map[string]domain.PracticeItemSnapshot, error)
}
