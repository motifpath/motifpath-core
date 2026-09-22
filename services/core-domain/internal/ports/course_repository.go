package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// CourseRepository persists Course and CourseCheckpoint records — the live,
// currently-being-authored draft. Snapshotting a draft into an immutable
// CourseVersion is a separate concern this repository does not implement.
type CourseRepository interface {
	Create(ctx context.Context, course domain.Course) error

	// GetByID returns domain.ErrNotFound if no course exists with the given id.
	GetByID(ctx context.Context, id string) (domain.Course, error)

	// List returns every course in the catalog, optionally restricted to a
	// single status. A nil status returns every course regardless of status.
	List(ctx context.Context, status *domain.CourseStatus) ([]domain.Course, error)

	// Replace replaces course's title, summary, level, and checkpoints
	// wholesale — its current checkpoints are deleted and
	// course.Checkpoints inserted in their place, in one transaction.
	// Returns domain.ErrNotFound if no course exists with the given id.
	Replace(ctx context.Context, course domain.Course) error
}
