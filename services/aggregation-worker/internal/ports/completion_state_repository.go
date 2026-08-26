package ports

import (
	"context"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// CompletionStateRepository persists per-student, per-content-node completion
// status to the aggregates collection (ADR-011).
//
// GetStatus and Upsert are read-modify-write from the application layer's
// perspective, which is safe without additional locking: ADR-006 partitions
// motifpath.events by student_id, so a single consumer instance processes all
// of one student's events strictly in order. Two calls for the same
// (student_id, content_node_id) pair never race.
type CompletionStateRepository interface {
	// GetStatus reports the current status for a student/node pair. found is
	// false when no document exists yet — distinct from CompletionStatusNotStarted,
	// which has no meaning as a stored zero value.
	GetStatus(ctx context.Context, studentID, contentNodeID string) (status domain.CompletionStatus, found bool, err error)

	// Upsert writes status unconditionally. Callers are responsible for having
	// already applied domain.NextStatus — this method does not itself guard
	// against downgrading a completed node.
	Upsert(ctx context.Context, studentID, contentNodeID string, status domain.CompletionStatus) error
}
