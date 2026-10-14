package ports

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
)

// AlphaActivityReader reads what the alpha observation measures from the
// Aggregation Worker's MongoDB collections: `practice_sessions`,
// `learning_activity`, `aggregates` and `student_activity`. It is
// read-only from this service's perspective — the worker is the only
// writer.
type AlphaActivityReader interface {
	// SessionsStartedBetween returns the practice sessions of studentIDs
	// that started at or after from and before to, in any state. A session
	// whose start hasn't arrived is left out.
	SessionsStartedBetween(ctx context.Context, studentIDs []string, from, to time.Time) ([]domain.PracticeSessionRecord, error)

	// FirstCompletions returns when the student first completed each of
	// contentNodeIDs; a node never completed is absent.
	FirstCompletions(ctx context.Context, studentID string, contentNodeIDs []string) (map[string]time.Time, error)

	// FirstStarts returns when the student first started each of
	// contentNodeIDs; a node with no start kept is absent.
	FirstStarts(ctx context.Context, studentID string, contentNodeIDs []string) (map[string]time.Time, error)

	// LastActive returns when each of studentIDs was last active; a student
	// with no activity at all is absent.
	LastActive(ctx context.Context, studentIDs []string) (map[string]time.Time, error)
}
