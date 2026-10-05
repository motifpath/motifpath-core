package domain

// ExerciseStartedEvent is emitted when a student begins an exercise attempt. Always
// paired with an ExerciseEndedEvent marking the attempt's conclusion.
type ExerciseStartedEvent struct {
	TrackingEventBase
	ExerciseID     string
	TriggerContext TriggerContext
}

func (e ExerciseStartedEvent) Base() TrackingEventBase { return e.TrackingEventBase }

// ExerciseProgressEvent is emitted periodically during an active exercise attempt to
// signal continued engagement, distinguishing active practice from abandoned sessions.
type ExerciseProgressEvent struct {
	TrackingEventBase
	ExerciseID     string
	TriggerContext TriggerContext

	// ElapsedSeconds is time since the paired ExerciseStartedEvent, client-computed.
	ElapsedSeconds *int
}

func (e ExerciseProgressEvent) Base() TrackingEventBase { return e.TrackingEventBase }

// ExerciseEndedEvent is emitted when an exercise attempt session ends, regardless of
// outcome. Always paired with an ExerciseStartedEvent.
type ExerciseEndedEvent struct {
	TrackingEventBase
	ExerciseID     string
	TriggerContext TriggerContext
	Outcome        ExerciseOutcome

	// FinalScore is present only when Outcome is ExerciseOutcomeCompleted.
	FinalScore *int
}

func (e ExerciseEndedEvent) Base() TrackingEventBase { return e.TrackingEventBase }
