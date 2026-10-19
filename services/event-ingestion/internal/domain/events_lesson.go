package domain

// LessonStartedEvent is emitted when a student begins engaging with a ContentNode
// for the first time in a session.
type LessonStartedEvent struct {
	TrackingEventBase
	ContentContext ContentContext
}

func (e LessonStartedEvent) Base() TrackingEventBase { return e.TrackingEventBase }

// LessonResumedEvent is emitted when a student returns to a ContentNode they
// previously started but did not complete.
type LessonResumedEvent struct {
	TrackingEventBase
	ContentContext ContentContext
}

func (e LessonResumedEvent) Base() TrackingEventBase { return e.TrackingEventBase }

// LessonCompletedEvent is emitted when a student completes a ContentNode. Triggers
// prerequisite checking for subsequent nodes in the learning path.
type LessonCompletedEvent struct {
	TrackingEventBase
	ContentContext ContentContext

	// DurationSeconds is nil when the client could not reliably determine session
	// duration (e.g. the tab was backgrounded for an unknown period).
	DurationSeconds *int
}

func (e LessonCompletedEvent) Base() TrackingEventBase { return e.TrackingEventBase }
