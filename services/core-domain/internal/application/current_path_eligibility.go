package application

import (
	"context"
	"errors"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// hasEligibleAlternative reports whether studentID holds any other active
// CourseEnrollment or non-archived standalone StudentPath, excluding the
// one identified by excludeStandalonePathID/excludeCourseEnrollmentID (pass
// "" for whichever one doesn't apply). Shared by AbandonCourseEnrollment and
// ArchiveStandaloneStudentPath to decide whether leaving the caller's
// current course/path is refused (something else exists the student must
// explicitly switch to first, via PUT /students/me/current-path) or allowed
// outright (nothing else exists, so the current pointer simply clears).
func hasEligibleAlternative(ctx context.Context, studentPaths ports.StudentPathRepository, enrollments ports.CourseEnrollmentRepository, studentID, excludeStandalonePathID, excludeCourseEnrollmentID string) (bool, error) {
	paths, err := studentPaths.ListActiveStandaloneByStudentID(ctx, studentID)
	if err != nil {
		return false, err
	}
	for _, p := range paths {
		if p.ID != excludeStandalonePathID {
			return true, nil
		}
	}

	active, err := enrollments.ListActiveByStudentID(ctx, studentID)
	if err != nil {
		return false, err
	}
	for _, e := range active {
		if e.ID != excludeCourseEnrollmentID {
			return true, nil
		}
	}

	return false, nil
}

// checkCanLeaveCurrent loads studentID's StudentLearningState and reports
// whether the standalone path (excludeStandalonePathID) or course
// enrollment (excludeCourseEnrollmentID) being left — pass "" for whichever
// doesn't apply — is currently set. Returns domain.ErrConflict if it is
// current and another eligible course enrollment or standalone path
// exists, per the shared "switch before leaving your current thing unless
// it's the only thing you have" rule AbandonCourseEnrollment and
// ArchiveStandaloneStudentPath both enforce.
func checkCanLeaveCurrent(ctx context.Context, state ports.StudentLearningStateRepository, studentPaths ports.StudentPathRepository, enrollments ports.CourseEnrollmentRepository, studentID, excludeStandalonePathID, excludeCourseEnrollmentID string) (domain.StudentLearningState, bool, error) {
	s, err := state.GetByStudentID(ctx, studentID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.StudentLearningState{}, false, err
	}

	isCurrent := (excludeStandalonePathID != "" && s.CurrentStandalonePathID != nil && *s.CurrentStandalonePathID == excludeStandalonePathID) ||
		(excludeCourseEnrollmentID != "" && s.CurrentCourseEnrollmentID != nil && *s.CurrentCourseEnrollmentID == excludeCourseEnrollmentID)
	if !isCurrent {
		return s, false, nil
	}

	hasAlternative, err := hasEligibleAlternative(ctx, studentPaths, enrollments, studentID, excludeStandalonePathID, excludeCourseEnrollmentID)
	if err != nil {
		return domain.StudentLearningState{}, false, err
	}
	if hasAlternative {
		return domain.StudentLearningState{}, false, domain.ErrConflict
	}
	return s, true, nil
}
