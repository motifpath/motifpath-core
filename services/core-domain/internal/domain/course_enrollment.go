package domain

import "time"

// CourseEnrollmentStatus tracks a CourseEnrollment's lifecycle: active (in
// progress), completed (every checkpoint finished), or abandoned (the
// student left it).
type CourseEnrollmentStatus string

const (
	CourseEnrollmentStatusActive    CourseEnrollmentStatus = "active"
	CourseEnrollmentStatusCompleted CourseEnrollmentStatus = "completed"
	CourseEnrollmentStatusAbandoned CourseEnrollmentStatus = "abandoned"
)

// CourseEnrollment is a student's self-enrollment in one course, pinned to
// the CourseVersion that was published at enrollment time, tracking their
// active checkpoint independently of whichever course or path is currently
// focused (StudentLearningState).
type CourseEnrollment struct {
	ID                  string
	StudentID           string
	CourseID            string
	CourseTitle         string
	CourseVersionNumber int
	Status              CourseEnrollmentStatus
	// ActiveCheckpointStudentPathID is the StudentPath for the checkpoint
	// the student is currently working through. Nil once the enrollment is
	// completed or abandoned.
	ActiveCheckpointStudentPathID *string
	// ActiveCheckpointPosition is the 1-based position of the active
	// checkpoint within the course. Nil once the enrollment is completed
	// or abandoned. Stored rather than derived: it is the enrollment's own
	// progress state, not something resolvable from the pinned
	// CourseVersion's checkpoint list alone (checkpoint advancement is a
	// later capability this type only carries the field for).
	ActiveCheckpointPosition *int
	EnrolledAt               time.Time
}

// IsActive reports whether the enrollment's status is active.
func (e CourseEnrollment) IsActive() bool {
	return e.Status == CourseEnrollmentStatusActive
}

// NewCourseEnrollment creates a new active CourseEnrollment for studentID,
// pinned to version, starting at checkpoint 1. studentPathID is the id of
// checkpoint 1's StudentPath — the application layer copies checkpoint 1's
// LearningPath template into a new StudentPath before calling this and
// passes its id in, the same copy-on-assign flow AssignLearningPath already
// performs for standalone paths.
func NewCourseEnrollment(id, studentID string, version CourseVersion, studentPathID string, enrolledAt time.Time) (CourseEnrollment, error) {
	if len(version.Checkpoints) == 0 {
		return CourseEnrollment{}, NewValidationError("course_id", "the course's published version has no checkpoints")
	}

	position := version.Checkpoints[0].Position
	return CourseEnrollment{
		ID:                            id,
		StudentID:                     studentID,
		CourseID:                      version.CourseID,
		CourseTitle:                   version.TitleSnapshot,
		CourseVersionNumber:           version.VersionNumber,
		Status:                        CourseEnrollmentStatusActive,
		ActiveCheckpointStudentPathID: &studentPathID,
		ActiveCheckpointPosition:      &position,
		EnrolledAt:                    enrolledAt,
	}, nil
}
