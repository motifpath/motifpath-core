package application

import (
	"context"

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
