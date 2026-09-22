package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// StudentLearningStateRepository persists each student's single current
// pointer — a course enrollment or a standalone path, never both.
type StudentLearningStateRepository interface {
	// GetByStudentID returns domain.ErrNotFound if studentID has no
	// StudentLearningState row yet (never having been assigned or enrolled
	// in anything).
	GetByStudentID(ctx context.Context, studentID string) (domain.StudentLearningState, error)

	// Upsert creates or replaces the StudentLearningState row for
	// state.StudentID.
	Upsert(ctx context.Context, state domain.StudentLearningState) error
}
