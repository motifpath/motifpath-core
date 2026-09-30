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

	// GetCreatorIDs returns the creator user id of every course in ids in one
	// round-trip, keyed by course id. It reads the course rows alone —
	// never the live draft's checkpoints — so it still answers for a course
	// whose draft no longer resolves. An unknown id is simply absent.
	GetCreatorIDs(ctx context.Context, ids []string) (map[string]string, error)

	// List returns one page of the courses matching filter, ordered by title
	// then id (the published version's title when filter.PublishedView is
	// set), with the count of all matches across pages.
	List(ctx context.Context, filter domain.CourseListFilter, page domain.PageRequest) (domain.Page[domain.Course], error)

	// ListCreatorIDs returns the distinct creator user ids of every course
	// matching filter, in no particular order; an empty, non-nil slice when
	// nothing matches.
	ListCreatorIDs(ctx context.Context, filter domain.CourseListFilter) ([]string, error)

	// Replace replaces course's title, summary, level, and checkpoints
	// wholesale — its current checkpoints are deleted and
	// course.Checkpoints inserted in their place, in one transaction.
	// Returns domain.ErrNotFound if no course exists with the given id.
	Replace(ctx context.Context, course domain.Course) error

	// UpdateStatus sets the status of the course with the given id.
	// Returns domain.ErrNotFound if no course exists with that id.
	UpdateStatus(ctx context.Context, id string, status domain.CourseStatus) error
}
