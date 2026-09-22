package application

import (
	"context"
	"errors"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// CourseEnrollmentService manages a student's self-enrollment in a course:
// creating a CourseEnrollment pinned to the course's latest published
// version (copying checkpoint 1's LearningPath template into a new
// StudentPath, the same copy-on-assign flow AssignLearningPath performs for
// staff-assigned paths), listing a student's enrollments, and abandoning
// one. Switching the caller's current course or path — which touches both
// CourseEnrollment and standalone StudentPath — lives on StudentPathService
// instead (SetCurrentPath), since it composes the same StudentPathView
// GetMyPath already builds.
type CourseEnrollmentService struct {
	courses        ports.CourseRepository
	courseVersions ports.CourseVersionRepository
	paths          ports.LearningPathRepository
	studentPaths   ports.StudentPathRepository
	enrollments    ports.CourseEnrollmentRepository
	studentPathSvc *StudentPathService
	state          ports.StudentLearningStateRepository
	newID          func() string
	now            func() time.Time
}

func NewCourseEnrollmentService(
	courses ports.CourseRepository,
	courseVersions ports.CourseVersionRepository,
	paths ports.LearningPathRepository,
	studentPaths ports.StudentPathRepository,
	enrollments ports.CourseEnrollmentRepository,
	studentPathSvc *StudentPathService,
	state ports.StudentLearningStateRepository,
	newID func() string,
	now func() time.Time,
) *CourseEnrollmentService {
	return &CourseEnrollmentService{
		courses:        courses,
		courseVersions: courseVersions,
		paths:          paths,
		studentPaths:   studentPaths,
		enrollments:    enrollments,
		studentPathSvc: studentPathSvc,
		state:          state,
		newID:          newID,
		now:            now,
	}
}

// CreateCourseEnrollment self-enrolls caller in courseID: pins to the
// course's latest published CourseVersion, copies checkpoint 1's
// LearningPath template into a new StudentPath, and sets the new enrollment
// as caller's current course only if nothing is currently set — a student
// actively running another course or a standalone path is never silently
// switched away from it. Only students may self-enroll. Refused with
// domain.ErrNotFound if courseID does not exist, the course is draft or
// retired, or its latest published version is not currently available for
// new enrollments. Refused with domain.ErrConflict if caller already holds
// an active CourseEnrollment for this course — a prior enrollment that was
// later abandoned or completed does not count, so re-enrolling after
// leaving a course is allowed.
func (s *CourseEnrollmentService) CreateCourseEnrollment(ctx context.Context, caller domain.User, courseID string) (domain.CourseEnrollment, error) {
	if caller.Role != domain.RoleStudent {
		return domain.CourseEnrollment{}, domain.ErrForbidden
	}

	latest, err := s.resolveEnrollableVersion(ctx, courseID)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}

	if err := s.ensureNoActiveEnrollment(ctx, caller.ID, courseID); err != nil {
		return domain.CourseEnrollment{}, err
	}

	checkpoint1 := latest.Checkpoints[0]
	template, err := s.paths.GetByID(ctx, checkpoint1.LearningPathID)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}

	enrollmentID := s.newID()
	sp, err := s.studentPathSvc.CopyTemplateForCheckpoint(ctx, caller.ID, template, caller.ID, enrollmentID, checkpoint1.Position)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}

	enrollment, err := domain.NewCourseEnrollment(enrollmentID, caller.ID, latest, sp.ID, s.now())
	if err != nil {
		return domain.CourseEnrollment{}, err
	}
	if err := s.enrollments.Create(ctx, enrollment); err != nil {
		return domain.CourseEnrollment{}, err
	}

	if err := s.setCurrentIfNothingSet(ctx, caller.ID, enrollment.ID); err != nil {
		return domain.CourseEnrollment{}, err
	}

	return enrollment, nil
}

// resolveEnrollableVersion returns courseID's latest published
// CourseVersion, or domain.ErrNotFound if courseID does not exist, the
// course is draft or retired, or its latest published version is not
// currently available for new enrollments.
func (s *CourseEnrollmentService) resolveEnrollableVersion(ctx context.Context, courseID string) (domain.CourseVersion, error) {
	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return domain.CourseVersion{}, err
	}
	if course.Status != domain.CourseStatusPublished {
		return domain.CourseVersion{}, domain.ErrNotFound
	}

	latest, err := s.courseVersions.GetLatestByCourseID(ctx, courseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.CourseVersion{}, domain.ErrNotFound
		}
		return domain.CourseVersion{}, err
	}
	if !latest.AvailableForNewEnrollments {
		return domain.CourseVersion{}, domain.ErrNotFound
	}

	return latest, nil
}

// ensureNoActiveEnrollment returns domain.ErrConflict if studentID already
// holds an active CourseEnrollment for courseID.
func (s *CourseEnrollmentService) ensureNoActiveEnrollment(ctx context.Context, studentID, courseID string) error {
	if _, err := s.enrollments.GetActiveByCourseID(ctx, studentID, courseID); err == nil {
		return domain.ErrConflict
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	return nil
}

// setCurrentIfNothingSet points studentID's current pointer at
// enrollmentID, but only if nothing is currently set — a student actively
// running another course or a standalone path is never silently switched
// away from it.
func (s *CourseEnrollmentService) setCurrentIfNothingSet(ctx context.Context, studentID, enrollmentID string) error {
	existingState, err := s.state.GetByStudentID(ctx, studentID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	existingState.StudentID = studentID
	if existingState.HasCurrent() {
		return nil
	}
	return s.state.Upsert(ctx, existingState.WithCurrentCourseEnrollment(enrollmentID))
}

// ListMyCourseEnrollments returns every CourseEnrollment caller has ever
// held — active, completed, and abandoned. Only students hold course
// enrollments.
func (s *CourseEnrollmentService) ListMyCourseEnrollments(ctx context.Context, caller domain.User) ([]domain.CourseEnrollment, error) {
	if caller.Role != domain.RoleStudent {
		return nil, domain.ErrForbidden
	}
	return s.enrollments.ListByStudentID(ctx, caller.ID)
}

// AbandonCourseEnrollment sets the CourseEnrollment with the given id to
// abandoned. All of its checkpoints' StudentPaths are implicitly left
// behind — per-checkpoint partial archiving does not exist, since
// checkpoints are sequential and owned by one enrollment. Only students
// hold course enrollments. Refused with domain.ErrNotFound if no active
// enrollment with this id belongs to caller. If it is caller's current
// course or path, refused with domain.ErrConflict when another eligible
// course enrollment or standalone path exists — the student must switch to
// it first via SetCurrentPath — and allowed, clearing the current pointer,
// only when nothing else exists to become current.
func (s *CourseEnrollmentService) AbandonCourseEnrollment(ctx context.Context, caller domain.User, enrollmentID string) (domain.CourseEnrollment, error) {
	if caller.Role != domain.RoleStudent {
		return domain.CourseEnrollment{}, domain.ErrForbidden
	}

	enrollment, err := s.enrollments.GetByID(ctx, enrollmentID)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}
	if enrollment.StudentID != caller.ID || !enrollment.IsActive() {
		return domain.CourseEnrollment{}, domain.ErrNotFound
	}

	state, isCurrent, err := s.checkCanLeaveCurrent(ctx, caller.ID, "", enrollment.ID)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}

	if err := s.enrollments.Abandon(ctx, enrollment.ID); err != nil {
		return domain.CourseEnrollment{}, err
	}
	enrollment.Status = domain.CourseEnrollmentStatusAbandoned
	enrollment.ActiveCheckpointStudentPathID = nil
	enrollment.ActiveCheckpointPosition = nil

	if isCurrent {
		if err := s.state.Upsert(ctx, state.Cleared()); err != nil {
			return domain.CourseEnrollment{}, err
		}
	}

	return enrollment, nil
}

// checkCanLeaveCurrent loads studentID's StudentLearningState and reports
// whether the standalone path (excludeStandalonePathID) or course
// enrollment (excludeCourseEnrollmentID) being left — pass "" for whichever
// doesn't apply — is currently set. Returns domain.ErrConflict if it is
// current and another eligible course enrollment or standalone path
// exists, per the shared "switch before leaving your current thing unless
// it's the only thing you have" rule AbandonCourseEnrollment and
// ArchiveStandaloneStudentPath both enforce.
func (s *CourseEnrollmentService) checkCanLeaveCurrent(ctx context.Context, studentID, excludeStandalonePathID, excludeCourseEnrollmentID string) (domain.StudentLearningState, bool, error) {
	state, err := s.state.GetByStudentID(ctx, studentID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.StudentLearningState{}, false, err
	}
	isCurrent := state.CurrentCourseEnrollmentID != nil && *state.CurrentCourseEnrollmentID == excludeCourseEnrollmentID
	if !isCurrent {
		return state, false, nil
	}

	hasAlternative, err := hasEligibleAlternative(ctx, s.studentPaths, s.enrollments, studentID, excludeStandalonePathID, excludeCourseEnrollmentID)
	if err != nil {
		return domain.StudentLearningState{}, false, err
	}
	if hasAlternative {
		return domain.StudentLearningState{}, false, domain.ErrConflict
	}
	return state, true, nil
}
