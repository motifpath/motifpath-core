package ports

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
)

// StudentPathRepository persists StudentPath records — a student's own
// copy of a learning path template, created at assign or enrollment time.
// A student may hold many StudentPaths at once.
type StudentPathRepository interface {
	// Create returns domain.ErrAlreadyExists when path is standalone and
	// the student already holds a non-archived standalone copy of the same
	// template — a learner holds at most one.
	Create(ctx context.Context, path domain.StudentPath) error

	// FindActiveStandalone returns studentID's non-archived standalone copy
	// of templateID, or domain.ErrNotFound when there is none.
	FindActiveStandalone(ctx context.Context, studentID, templateID string) (domain.StudentPath, error)

	// GetByID returns domain.ErrNotFound if no StudentPath exists with the
	// given id.
	GetByID(ctx context.Context, id string) (domain.StudentPath, error)

	// ListActiveStandaloneByStudentID returns every non-archived, non-course
	// StudentPath owned by studentID — used to decide whether another path
	// is eligible to become current when the caller's current one is
	// archived or abandoned.
	ListActiveStandaloneByStudentID(ctx context.Context, studentID string) ([]domain.StudentPath, error)

	// ListStandaloneByStudentID returns every non-course StudentPath owned by
	// studentID, active and archived alike, newest assigned first — or an
	// empty slice if there are none.
	ListStandaloneByStudentID(ctx context.Context, studentID string) ([]domain.StudentPath, error)

	// Archive sets archivedAt on the StudentPath with the given id.
	// Returns domain.ErrNotFound if no StudentPath exists with that id.
	Archive(ctx context.Context, id string, archivedAt time.Time) error
}
