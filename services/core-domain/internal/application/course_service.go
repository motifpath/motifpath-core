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
	paths    ports.LearningPathRepository
	courses  ports.CourseRepository
	versions ports.CourseVersionRepository
	newID    func() string
	now      func() time.Time
}

func NewCourseService(paths ports.LearningPathRepository, courses ports.CourseRepository, versions ports.CourseVersionRepository, newID func() string, now func() time.Time) *CourseService {
	return &CourseService{paths: paths, courses: courses, versions: versions, newID: newID, now: now}
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

// PublishCourse snapshots course's current title, summary, level, and
// checkpoint identities into a new immutable CourseVersion — the course's
// next version_number, one greater than whatever was last published (or 1
// if this is the first publish). The course's status becomes published if
// this is its first publication; it stays published on every later
// publish. Every already-enrolled student is unaffected — enrollment
// pinning is a separate, later capability. Publishing is admin-only: it
// exposes the draft to students for the first time, so even the creating
// teacher gets forbidden.
func (s *CourseService) PublishCourse(ctx context.Context, caller domain.User, id string) (domain.CourseVersion, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.CourseVersion{}, domain.ErrForbidden
	}

	course, err := s.courses.GetByID(ctx, id)
	if err != nil {
		return domain.CourseVersion{}, err
	}

	nextVersionNumber := 1
	if latest, err := s.versions.GetLatestByCourseID(ctx, id); err == nil {
		nextVersionNumber = latest.VersionNumber + 1
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.CourseVersion{}, err
	}

	version := domain.NewCourseVersionSnapshot(s.newID(), course, nextVersionNumber, s.now())
	if err := s.versions.Create(ctx, version); err != nil {
		return domain.CourseVersion{}, err
	}

	if course.Status == domain.CourseStatusDraft {
		if err := s.courses.UpdateStatus(ctx, id, domain.CourseStatusPublished); err != nil {
			return domain.CourseVersion{}, err
		}
	}

	return version, nil
}

// RetireCourse removes course from the catalog for new enrollment only. This
// is not a delete: it does not cascade, and every CourseEnrollment already
// created against the course, and the StudentPaths under it, continue to
// resolve normally. Admin-only.
func (s *CourseService) RetireCourse(ctx context.Context, caller domain.User, id string) (domain.Course, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.Course{}, domain.ErrForbidden
	}

	course, err := s.courses.GetByID(ctx, id)
	if err != nil {
		return domain.Course{}, err
	}

	if err := s.courses.UpdateStatus(ctx, id, domain.CourseStatusRetired); err != nil {
		return domain.Course{}, err
	}
	course.Status = domain.CourseStatusRetired
	return course, nil
}

// LatestVersion returns the latest published CourseVersion for the course
// with the given id. Returns domain.ErrNotFound if the course has never
// been published. Exposed so the HTTP layer can compute
// has_unpublished_changes and latest_published_version without duplicating
// the version lookup.
func (s *CourseService) LatestVersion(ctx context.Context, id string) (domain.CourseVersion, error) {
	return s.versions.GetLatestByCourseID(ctx, id)
}

// CourseOutlineItem is one content node's title within a published
// checkpoint's outline — resolved live from its LearningPath template
// rather than from any snapshot, the same "resolve live from current
// state" pattern StudentPathItem already uses for its title/content_type.
type CourseOutlineItem struct {
	Title        string
	SectionLabel *string
}

// CourseOutlineCheckpoint is a published checkpoint as shown to a
// prospective or enrolled student: the title pinned at publish time, plus
// its items resolved live.
type CourseOutlineCheckpoint struct {
	Position int
	Title    string
	Items    []CourseOutlineItem
}

// PublishedCourseView is a course's latest published version, rendered as
// an outline — the composed result GetPublishedCourse returns.
type PublishedCourseView struct {
	Title       string
	Summary     string
	Level       domain.DifficultyLevel
	Status      domain.CourseStatus
	PublishedAt time.Time
	Checkpoints []CourseOutlineCheckpoint
}

// GetPublishedCourse returns course's latest published version rendered as
// an outline: each checkpoint's title, pinned at the moment it was
// published, and its ordered item titles resolved live from the
// checkpoint's LearningPath template — never lesson content or authoring
// detail such as a checkpoint's learning_path_id. Accessible by any
// authenticated role. Returns domain.ErrNotFound if no course exists with
// the given id, or if it has never been published — "the published
// version" genuinely does not exist yet, regardless of caller role.
func (s *CourseService) GetPublishedCourse(ctx context.Context, id string) (PublishedCourseView, error) {
	course, err := s.courses.GetByID(ctx, id)
	if err != nil {
		return PublishedCourseView{}, err
	}

	latest, err := s.versions.GetLatestByCourseID(ctx, id)
	if err != nil {
		return PublishedCourseView{}, err
	}

	checkpoints := make([]CourseOutlineCheckpoint, len(latest.Checkpoints))
	for i, cp := range latest.Checkpoints {
		path, err := s.paths.GetByID(ctx, cp.LearningPathID)
		if err != nil {
			return PublishedCourseView{}, err
		}
		items := make([]CourseOutlineItem, len(path.Items))
		for j, item := range path.Items {
			items[j] = CourseOutlineItem{Title: item.Title, SectionLabel: item.SectionLabel}
		}
		checkpoints[i] = CourseOutlineCheckpoint{Position: cp.Position, Title: cp.EffectiveTitle, Items: items}
	}

	return PublishedCourseView{
		Title:       latest.TitleSnapshot,
		Summary:     latest.SummarySnapshot,
		Level:       latest.LevelSnapshot,
		Status:      course.Status,
		PublishedAt: latest.PublishedAt,
		Checkpoints: checkpoints,
	}, nil
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
