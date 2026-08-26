// Package domain holds the entities and pure business rules for the Aggregation
// Worker. Per ADR-011, this worker's MVP scope is limited to deriving per-student,
// per-content-node completion status from lesson-family tracking events — it does
// not model exercise scoring or analytics, which remain post-MVP.
package domain

// EventType identifies which tracking event a message represents. The worker
// only acts on the three lesson-family values; all others (exercise.*) are
// accepted without error but produce no state change — see IsLessonEvent.
type EventType string

const (
	EventTypeLessonStarted   EventType = "lesson.started"
	EventTypeLessonResumed   EventType = "lesson.resumed"
	EventTypeLessonCompleted EventType = "lesson.completed"
)

// IsLessonEvent reports whether t is one of the three event types this worker
// derives completion status from.
func IsLessonEvent(t EventType) bool {
	switch t {
	case EventTypeLessonStarted, EventTypeLessonResumed, EventTypeLessonCompleted:
		return true
	default:
		return false
	}
}

// TrackingEvent is the subset of a motifpath.events message this worker needs.
// It is decoded independently of the Event Ingestion Service's own domain
// types — per the monorepo's layering rules, services never share Go packages,
// and this worker only needs three fields regardless of which of the seven
// tracking events was published.
type TrackingEvent struct {
	EventType     EventType
	StudentID     string
	ContentNodeID string
}
