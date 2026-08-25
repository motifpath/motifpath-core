package domain

import "time"

// EventType identifies which of the seven tracking events a payload represents.
// The set is closed — adding a value requires a spec change and a new ADR if it
// introduces a new consumer concern.
type EventType string

const (
	EventTypeLessonStarted      EventType = "lesson.started"
	EventTypeLessonResumed      EventType = "lesson.resumed"
	EventTypeLessonCompleted    EventType = "lesson.completed"
	EventTypeExerciseStarted    EventType = "exercise.started"
	EventTypeExerciseProgress   EventType = "exercise.progress"
	EventTypeExerciseAnswerSent EventType = "exercise.answer_sent"
	EventTypeExerciseEnded      EventType = "exercise.ended"
)

// TrackingEventBase is the common envelope carried by every student tracking event.
// EventID enables idempotent processing under at-least-once Kafka delivery; StudentID
// is used as the Kafka partition key.
type TrackingEventBase struct {
	EventID    string
	EventType  EventType
	StudentID  string
	SessionID  string
	OccurredAt time.Time
}

// TrackingEvent is implemented by all seven event-specific structs. It stands in for
// a discriminated union: Go has no native sum type, and interface{}/any would erase
// the compile-time guarantee that only the seven known event shapes satisfy it.
type TrackingEvent interface {
	Base() TrackingEventBase
}

// ContentType is the media format of a ContentNode, mirrored onto lesson events.
type ContentType string

const (
	ContentTypeVideo   ContentType = "video"
	ContentTypeArticle ContentType = "article"
)

// ContentContext describes the ContentNode a student is engaging with. Required on
// every lesson event.
type ContentContext struct {
	ContentNodeID string
	ContentType   ContentType
	TeacherID     string
}

// TriggerSource describes what initiated an exercise attempt.
type TriggerSource string

const (
	TriggerSourceChallengeSequence TriggerSource = "challenge_sequence"
	TriggerSourceFreePractice      TriggerSource = "free_practice"
	TriggerSourceRemediation       TriggerSource = "remediation"
)

// TriggerContext describes the context in which an exercise was initiated. Required
// on every exercise event. ContentNodeID and ChallengeID are populated only when
// Source is ChallengeSequence or Remediation, per the spec.
type TriggerContext struct {
	Source        TriggerSource
	ContentNodeID string
	ChallengeID   string
}

// ExerciseOutcome describes how an exercise attempt concluded.
type ExerciseOutcome string

const (
	ExerciseOutcomeCompleted ExerciseOutcome = "completed"
	ExerciseOutcomeAbandoned ExerciseOutcome = "abandoned"
)
