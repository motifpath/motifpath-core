package domain

import "time"

// PathAssignment records a learning path assigned to a student. Only one
// active assignment exists per student at MVP; assigning a new path
// replaces it entirely rather than mutating it in place, which is why this
// type carries no update semantics — every assignment is created once and
// never changed. See PathAssignmentRepository.ReplaceActive.
type PathAssignment struct {
	ID             string
	StudentID      string
	LearningPathID string
	AssignedBy     string
	AssignedAt     time.Time
}
