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

	// List returns one page of the courses matching filter, ordered by title
	// then id (the published version's title when filter.PublishedView is
	// set), with the count of all matches across pages.
	List(ctx context.Context, filter domain.CourseListFilter, page domain.PageRequest) (domain.Page[domain.Course], error)

	// Replace replaces course's title, summary, level, and checkpoints
	// wholesale — its current checkpoints are deleted and
	// course.Checkpoints inserted in their place, in one transaction.
	// Returns domain.ErrNotFound if no course exists with the given id.
	Replace(ctx context.Context, course domain.Course) error

	// UpdateStatus sets the status of the course with the given id.
	// Returns domain.ErrNotFound if no course exists with that id.
	UpdateStatus(ctx context.Context, id string, status domain.CourseStatus) error
}
