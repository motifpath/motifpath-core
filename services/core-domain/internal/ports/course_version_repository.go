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

	// GetLatestByCourseIDs is GetLatestByCourseID batched across several
	// courses in one round-trip — for a catalog listing that needs every
	// course's latest version rather than looping GetLatestByCourseID once
	// per course. A courseID with no published version is simply absent
	// from the result map, never an error.
	GetLatestByCourseIDs(ctx context.Context, courseIDs []string) (map[string]domain.CourseVersion, error)

	// IsLearningPathReferenced reports whether learningPathID is the
	// template behind any checkpoint of any CourseVersion ever published —
	// every CourseVersion row is itself a point-in-time publish snapshot,
	// so this never needs to consult a Course's current status: a version
	// belonging to a since-retired course still counts, since that
	// version's checkpoint sequence must always resolve for anyone still
	// reading it.
	IsLearningPathReferenced(ctx context.Context, learningPathID string) (bool, error)
}
