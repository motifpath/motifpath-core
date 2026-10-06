package ports

import (
	"context"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// PracticeSessionRepository keeps one record per (student_id, practice session).
// Its read-modify-write is safe without locking for the same reason as
// PracticeItemStateRepository's: this worker is the single writer for each student.
type PracticeSessionRepository interface {
	// Get reports the stored session; found is false when nothing about it has
	// been recorded yet.
	Get(ctx context.Context, studentID, sessionID string) (session domain.PracticeSession, found bool, err error)

	// Put replaces the stored session.
	Put(ctx context.Context, session domain.PracticeSession) error
}

// TapCheckRepository stores each tap check once per event id. Tap checks are
// never modified or deleted.
type TapCheckRepository interface {
	// Insert stores c and reports false, without error, when a tap check with its
	// event id is already stored.
	Insert(ctx context.Context, c domain.TapCheck) (inserted bool, err error)
}

// LearningActivityRepository stores each content node completion once per event
// id. Completions are never modified or deleted.
type LearningActivityRepository interface {
	// Insert stores a and reports false, without error, when a completion with its
	// event id is already stored.
	Insert(ctx context.Context, a domain.LearningActivity) (inserted bool, err error)
}
