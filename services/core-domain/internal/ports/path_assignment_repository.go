package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// PathAssignmentRepository persists PathAssignment records. Only one active
// assignment exists per student at MVP.
type PathAssignmentRepository interface {
	// ReplaceActive deletes any existing assignment for
	// assignment.StudentID and inserts assignment as the new active one,
	// atomically. This covers both "first ever assignment" and "replacing
	// an existing one" — the Gherkin scenarios treat both identically, with
	// a fresh assignment_id returned either way.
	ReplaceActive(ctx context.Context, assignment domain.PathAssignment) error

	// GetActiveByStudentID returns domain.ErrNotFound if the student has no
	// active assignment.
	GetActiveByStudentID(ctx context.Context, studentID string) (domain.PathAssignment, error)
}
