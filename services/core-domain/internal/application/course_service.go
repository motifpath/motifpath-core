package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/search"

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
	users    ports.UserRepository
	newID    func() string
	now      func() time.Time
}

func NewCourseService(paths ports.LearningPathRepository, courses ports.CourseRepository, versions ports.CourseVersionRepository, users ports.UserRepository, newID func() string, now func() time.Time) *CourseService {
	return &CourseService{paths: paths, courses: courses, versions: versions, users: users, newID: newID, now: now}
}

// CheckpointInput is one checkpoint the caller wants in a new or replaced
// course: the learning path template it points at and its optional title
// override.
type CheckpointInput struct {
	LearningPathID string
	Title          *string
}

// CourseInput is what a caller writes to create or replace a course draft.
type CourseInput struct {
	Title       string
	Summary     string
	Level       domain.DifficultyLevel
	Checkpoints []CheckpointInput
}

// fields resolves input into the domain's CourseFields, given its
// checkpoints already resolved against their learning paths.
func (input CourseInput) fields(checkpoints []domain.NewCourseCheckpoint) domain.CourseFields {
	return domain.CourseFields{Title: input.Title, Summary: input.Summary, Level: input.Level, Checkpoints: checkpoints}
}

// CreateCourse creates a course draft from the given ordered checkpoints.
// Only teachers and admins may create courses. A learning_path_id that
// doesn't exist is a validation failure (400), not a not-found error — the
// whole request is malformed, not a lookup that simply missed.
func (s *CourseService) CreateCourse(ctx context.Context, caller domain.User, input CourseInput) (domain.Course, error) {
	if !canManageContent(caller.Role) {
		return domain.Course{}, domain.ErrForbidden
	}

	resolved, err := s.resolveCheckpoints(ctx, input.Checkpoints)
	if err != nil {
		return domain.Course{}, err
	}

	course, err := domain.NewCourse(s.newID(), caller.ID, input.fields(resolved), s.now())
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

// ListCourses returns one page of the authoring course list matching
// filter, evaluated against each course's live draft. It is for teachers and
// admins only — a student is refused, since learners of every role browse
// ListCatalogCourses instead. A teacher sees courses of every status but
// only their own — naming another creator is forbidden. An admin may see
// every creator's courses, or narrow to one.
func (s *CourseService) ListCourses(ctx context.Context, caller domain.User, filter domain.CourseListFilter, page domain.PageRequest) (domain.Page[domain.Course], error) {
	if !canManageContent(caller.Role) {
		return domain.Page[domain.Course]{}, domain.ErrForbidden
	}
	if caller.Role == domain.RoleTeacher {
		if filter.CreatedBy != "" && filter.CreatedBy != caller.ID {
			return domain.Page[domain.Course]{}, domain.ErrForbidden
		}
		filter.CreatedBy = caller.ID
	}
	filter.PublishedView = false
	return s.courses.List(ctx, filter, page)
}

// ListCatalogCourses returns one page of the learner catalog matching
// filter: published courses only, whoever created them, with every filter
// evaluated against each course's latest published version. It is the same
// for every caller whatever their role — anyone can learn — so it takes no
// caller, and a status in filter never widens it beyond published.
func (s *CourseService) ListCatalogCourses(ctx context.Context, filter domain.CourseListFilter, page domain.PageRequest) (domain.Page[domain.Course], error) {
	published := domain.CourseStatusPublished
	filter.Status = &published
	filter.PublishedView = true
	return s.courses.List(ctx, filter, page)
}

// CourseCreator is a user who created at least one course, with their
// current display name.
type CourseCreator struct {
	UserID      string
	DisplayName string
}

// ListCourseCreators returns the distinct creators of the courses in the
// caller's authoring list (ListCourses), so an authoring screen can offer a
// complete creator filter without paging: a teacher gets at most
// themselves, an admin the creator of every course. A student is refused;
// learners use ListCatalogCreators. A non-empty nameQuery keeps only the
// creators whose display name contains it; see creatorsNamed for matching
// and ordering.
func (s *CourseService) ListCourseCreators(ctx context.Context, caller domain.User, nameQuery string) ([]CourseCreator, error) {
	if !canManageContent(caller.Role) {
		return nil, domain.ErrForbidden
	}
	var filter domain.CourseListFilter
	if caller.Role == domain.RoleTeacher {
		filter.CreatedBy = caller.ID
	}
	return s.creatorsNamed(ctx, filter, nameQuery)
}

// ListCatalogCreators returns the distinct creators of published courses,
// the creator filter's options for the learner catalog. Like
// ListCatalogCourses it is the same for every caller.
func (s *CourseService) ListCatalogCreators(ctx context.Context, nameQuery string) ([]CourseCreator, error) {
	published := domain.CourseStatusPublished
	return s.creatorsNamed(ctx, domain.CourseListFilter{Status: &published}, nameQuery)
}

// creatorsNamed returns the distinct creators of the courses matching
// filter, with their current display names, keeping only those whose name
// contains nameQuery when it is non-empty. Matching and ordering both ignore
// case and accents, the way a person reads a list of names; equal names fall
// back to user id.
func (s *CourseService) creatorsNamed(ctx context.Context, filter domain.CourseListFilter, nameQuery string) ([]CourseCreator, error) {
	ids, err := s.courses.ListCreatorIDs(ctx, filter)
	if err != nil {
		return nil, err
	}
	creators := []CourseCreator{}
	if len(ids) == 0 {
		return creators, nil
	}
	names, err := s.users.GetDisplayNames(ctx, ids)
	if err != nil {
		return nil, err
	}

	matcher := search.New(language.Und, search.Loose)
	for _, id := range ids {
		name, ok := names[id]
		if !ok {
			return nil, fmt.Errorf("display name for course creator %s: %w", id, domain.ErrNotFound)
		}
		if nameQuery != "" {
			if start, _ := matcher.IndexString(name, nameQuery); start < 0 {
				continue
			}
		}
		creators = append(creators, CourseCreator{UserID: id, DisplayName: name})
	}

	collator := collate.New(language.Und)
	sort.Slice(creators, func(i, j int) bool {
		if c := collator.CompareString(creators[i].DisplayName, creators[j].DisplayName); c != 0 {
			return c < 0
		}
		return creators[i].UserID < creators[j].UserID
	})
	return creators, nil
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
func (s *CourseService) ReplaceCourse(ctx context.Context, caller domain.User, id string, input CourseInput) (domain.Course, error) {
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

	resolved, err := s.resolveCheckpoints(ctx, input.Checkpoints)
	if err != nil {
		return domain.Course{}, err
	}

	replaced, err := domain.NewCourse(existing.ID, existing.CreatedBy, input.fields(resolved), existing.CreatedAt)
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

// ReactivateCourse returns a retired course to published: back in the
// catalog, open to new enrollment, with the latest version it already has.
// It never creates a version, so draft edits made since the last publish
// stay unpublished. Only a retired course can be reactivated. Admin-only,
// like retiring.
func (s *CourseService) ReactivateCourse(ctx context.Context, caller domain.User, id string) (domain.Course, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.Course{}, domain.ErrForbidden
	}

	course, err := s.courses.GetByID(ctx, id)
	if err != nil {
		return domain.Course{}, err
	}
	if course.Status != domain.CourseStatusRetired {
		return domain.Course{}, domain.NewValidationError("status", "only a retired course can be reactivated")
	}

	if err := s.courses.UpdateStatus(ctx, id, domain.CourseStatusPublished); err != nil {
		return domain.Course{}, err
	}
	course.Status = domain.CourseStatusPublished
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

// LatestVersions is LatestVersion batched across several courses in one
// call — used by ListCourses to resolve every catalog entry's
// has_unpublished_changes/latest_published_version without one round-trip
// per course. A course id with no published version is simply absent from
// the result map.
func (s *CourseService) LatestVersions(ctx context.Context, ids []string) (map[string]domain.CourseVersion, error) {
	return s.versions.GetLatestByCourseIDs(ctx, ids)
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
