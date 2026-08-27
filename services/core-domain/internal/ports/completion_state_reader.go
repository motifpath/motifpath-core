package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// CompletionStateReader reads per-student, per-content-node completion
// status from the Aggregation Worker's MongoDB `aggregates` collection
// (ADR-011). It is read-only from this service's perspective — the
// Aggregation Worker owns writes to that collection.
type CompletionStateReader interface {
	// GetStatuses returns the student's completion status for each of
	// contentNodeIDs. A node with no recorded status is simply absent from
	// the result; domain.BuildStudentPathItems treats an absent entry as
	// not_started, mirroring the Aggregation Worker's own GetStatus
	// found=false handling.
	GetStatuses(ctx context.Context, studentID string, contentNodeIDs []string) (map[string]domain.CompletionStatus, error)
}
