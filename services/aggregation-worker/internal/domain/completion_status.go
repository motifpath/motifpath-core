package domain

// CompletionStatus is a student's progress state on a single content node, as
// derived from lesson-family tracking events. It is the value ADR-011 writes
// to the aggregates collection and Phase 4's Core Domain Service reads back for
// GET /students/me/path.
type CompletionStatus string

const (
	CompletionStatusNotStarted CompletionStatus = "not_started"
	CompletionStatusInProgress CompletionStatus = "in_progress"
	CompletionStatusCompleted  CompletionStatus = "completed"
)

// NextStatus applies ADR-011's transition rule: lesson.started or
// lesson.resumed advance an untouched node to in_progress; lesson.completed
// marks it completed. completed is terminal — a student revisiting an
// already-finished node emits another lesson.started/resumed, but that must
// never regress the stored status, so any event type is a no-op once current
// is completed.
func NextStatus(current CompletionStatus, eventType EventType) CompletionStatus {
	if current == CompletionStatusCompleted {
		return CompletionStatusCompleted
	}

	switch eventType {
	case EventTypeLessonStarted, EventTypeLessonResumed:
		return CompletionStatusInProgress
	case EventTypeLessonCompleted:
		return CompletionStatusCompleted
	default:
		return current
	}
}
