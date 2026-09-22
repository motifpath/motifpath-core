package application

import (
	"context"
	"errors"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// CourseService manages Course — an ordered sequence of learning-path
// checkpoints students progress through. Every method here operates on the
// live, currently-being-authored draft; snapshotting a draft into an
// immutable published CourseVersion is a separate, later capability.
type CourseService struct {
	paths   ports.LearningPathRepository
	courses ports.CourseRepository
	newID   func() string
	now     func() time.Time
}

func NewCourseService(paths ports.LearningPathRepository, courses ports.CourseRepository, newID func() string, now func() time.Time) *CourseService {
	return &CourseService{paths: paths, courses: courses, newID: newID, now: now}
}

// CheckpointInput is one checkpoint the caller wants in a new or replaced
// course: the learning path template it points at and its optional title
// override.
type CheckpointInput struct {
	LearningPathID string
	Title          *string
}

// CreateCourse creates a course draft from the given ordered checkpoints.
// Only teachers and admins may create courses. A learning_path_id that
// doesn't exist is a validation failure (400), not a not-found error — the
// whole request is malformed, not a lookup that simply missed.
func (s *CourseService) CreateCourse(ctx context.Context, caller domain.User, title, summary string, level domain.DifficultyLevel, checkpoints []CheckpointInput) (domain.Course, error) {
	if !canManageContent(caller.Role) {
		return domain.Course{}, domain.ErrForbidden
	}

	resolved, err := s.resolveCheckpoints(ctx, checkpoints)
	if err != nil {
		return domain.Course{}, err
	}

	course, err := domain.NewCourse(s.newID(), caller.ID, title, summary, level, resolved, s.now())
	if err != nil {
		return domain.Course{}, err
	}
	if err := s.courses.Create(ctx, course); err != nil {
		return domain.Course{}, err
	}
	return course, nil
}

// GetCourse returns the course's live, current draft state by id. Only
// teachers and admins may view a course's live draft — a student's view is
// always the latest published version (a separate capability), never this
// live row.
func (s *CourseService) GetCourse(ctx context.Context, caller domain.User, id string) (domain.Course, error) {
	if !canManageContent(caller.Role) {
		return domain.Course{}, domain.ErrForbidden
	}
	return s.courses.GetByID(ctx, id)
}

// ListCourses returns the course catalog. Teachers and admins see courses
// of every status, optionally narrowed by the given status filter; a
// student's results are always implicitly published, regardless of any
// status given — the same rule GET /courses documents.
func (s *CourseService) ListCourses(ctx context.Context, caller domain.User, status *domain.CourseStatus) ([]domain.Course, error) {
	if caller.Role == domain.RoleStudent {
		published := domain.CourseStatusPublished
		status = &published
	}
	return s.courses.List(ctx, status)
}

// ReplaceCourse replaces the given course's title, summary, level, and
// checkpoints wholesale — the same way CreateCourse establishes them
// initially, reusing domain.NewCourse to recompute checkpoint positions and
// effective titles from scratch. The course's id, creator, creation time,
// and current status are preserved — a replace never publishes or
// unpublishes anything on its own, and domain.NewCourse always builds a
// fresh course in CourseStatusDraft only because that's the only status a
// brand-new course can start in. A learning_path_id that doesn't exist is a
// validation failure (400), matching CreateCourse. Only the creating
// teacher or an admin may replace a course.
func (s *CourseService) ReplaceCourse(ctx context.Context, caller domain.User, id, title, summary string, level domain.DifficultyLevel, checkpoints []CheckpointInput) (domain.Course, error) {
	if !canManageContent(caller.Role) {
		return domain.Course{}, domain.ErrForbidden
	}

	existing, err := s.courses.GetByID(ctx, id)
	if err != nil {
		return domain.Course{}, err
	}
	if err := requireOwner(caller, existing.CreatedBy); err != nil {
		return domain.Course{}, err
	}

	resolved, err := s.resolveCheckpoints(ctx, checkpoints)
	if err != nil {
		return domain.Course{}, err
	}

	replaced, err := domain.NewCourse(existing.ID, existing.CreatedBy, title, summary, level, resolved, existing.CreatedAt)
	if err != nil {
		return domain.Course{}, err
	}
	replaced.Status = existing.Status

	if err := s.courses.Replace(ctx, replaced); err != nil {
		return domain.Course{}, err
	}
	return replaced, nil
}

// resolveCheckpoints turns checkpoints into the resolved
// domain.NewCourseCheckpoint slice domain.NewCourse needs. Returns a
// domain.ValidationError under "learning_path_id" naming the first
// checkpoint whose learning_path_id doesn't exist — shared by CreateCourse
// and ReplaceCourse, which resolve checkpoints identically.
func (s *CourseService) resolveCheckpoints(ctx context.Context, checkpoints []CheckpointInput) ([]domain.NewCourseCheckpoint, error) {
	resolved := make([]domain.NewCourseCheckpoint, 0, len(checkpoints))
	for _, checkpoint := range checkpoints {
		path, err := s.paths.GetByID(ctx, checkpoint.LearningPathID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, domain.NewValidationError("learning_path_id", "references a learning path that does not exist: "+checkpoint.LearningPathID)
			}
			return nil, err
		}
		resolved = append(resolved, domain.NewCourseCheckpoint{Path: path, Title: checkpoint.Title})
	}
	return resolved, nil
}
