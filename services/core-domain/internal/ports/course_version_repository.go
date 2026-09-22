package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// CourseVersionRepository persists CourseVersion snapshots — each an
// immutable record of a course's published title, summary, level, and
// checkpoint identities at a point in time.
type CourseVersionRepository interface {
	Create(ctx context.Context, version domain.CourseVersion) error

	// GetLatestByCourseID returns the highest-version_number CourseVersion
	// for courseID, with its checkpoint snapshots ordered by position.
	// Returns domain.ErrNotFound if the course has never been published.
	GetLatestByCourseID(ctx context.Context, courseID string) (domain.CourseVersion, error)
}
