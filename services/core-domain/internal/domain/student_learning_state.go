package domain

// StudentLearningState tracks which single course enrollment or standalone
// path a student is currently working through. At most one of
// CurrentCourseEnrollmentID and CurrentStandalonePathID may be set at a
// time — a student who wants to switch to something else they already hold
// must explicitly leave whichever is current first.
type StudentLearningState struct {
	StudentID                 string
	CurrentCourseEnrollmentID *string
	CurrentStandalonePathID   *string
}

// Validate reports a validation error if both pointers are set — the
// invariant every persisted StudentLearningState must satisfy.
func (s StudentLearningState) Validate() error {
	if s.CurrentCourseEnrollmentID != nil && s.CurrentStandalonePathID != nil {
		return NewValidationError("current_path", "at most one of current_course_enrollment_id or current_standalone_path_id may be set at a time")
	}
	return nil
}

// HasCurrent reports whether the student currently has a course enrollment
// or standalone path set.
func (s StudentLearningState) HasCurrent() bool {
	return s.CurrentCourseEnrollmentID != nil || s.CurrentStandalonePathID != nil
}

// WithCurrentStandalonePath returns a copy of s pointed at the standalone
// path pathID, clearing any current course enrollment. Every state produced
// this way satisfies Validate by construction.
func (s StudentLearningState) WithCurrentStandalonePath(pathID string) StudentLearningState {
	s.CurrentStandalonePathID = &pathID
	s.CurrentCourseEnrollmentID = nil
	return s
}

// WithCurrentCourseEnrollment returns a copy of s pointed at the course
// enrollment enrollmentID, clearing any current standalone path.
func (s StudentLearningState) WithCurrentCourseEnrollment(enrollmentID string) StudentLearningState {
	s.CurrentCourseEnrollmentID = &enrollmentID
	s.CurrentStandalonePathID = nil
	return s
}

// Cleared returns a copy of s with both pointers unset.
func (s StudentLearningState) Cleared() StudentLearningState {
	s.CurrentCourseEnrollmentID = nil
	s.CurrentStandalonePathID = nil
	return s
}
