package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// CourseEnrollmentRepository persists CourseEnrollment records — a
// student's self-enrollment in one course, pinned to the CourseVersion
// published at enrollment time.
type CourseEnrollmentRepository interface {
	Create(ctx context.Context, enrollment domain.CourseEnrollment) error

	// GetByID returns domain.ErrNotFound if no CourseEnrollment exists with
	// the given id.
	GetByID(ctx context.Context, id string) (domain.CourseEnrollment, error)

	// GetActiveByCourseID returns studentID's active CourseEnrollment for
	// courseID, or domain.ErrNotFound if none exists — used to refuse a
	// second concurrent active enrollment in the same course. A prior
	// enrollment in this course that was later abandoned or completed does
	// not count, so re-enrolling after leaving a course is allowed.
	GetActiveByCourseID(ctx context.Context, studentID, courseID string) (domain.CourseEnrollment, error)

	// ListByStudentID returns every CourseEnrollment studentID has ever
	// held — active, completed, and abandoned.
	ListByStudentID(ctx context.Context, studentID string) ([]domain.CourseEnrollment, error)

	// ListActiveByStudentID returns every active CourseEnrollment owned by
	// studentID — used to decide whether another enrollment is eligible to
	// become current when the caller's current one is abandoned, or when a
	// standalone StudentPath is archived.
	ListActiveByStudentID(ctx context.Context, studentID string) ([]domain.CourseEnrollment, error)

	// Abandon sets the CourseEnrollment with the given id to abandoned,
	// clearing its active checkpoint pointer and position. Returns
	// domain.ErrNotFound if no CourseEnrollment exists with that id.
	Abandon(ctx context.Context, id string) error
}
