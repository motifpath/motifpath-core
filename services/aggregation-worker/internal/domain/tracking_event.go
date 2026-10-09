// Package domain holds the entities and pure business rules for the Aggregation
// Worker: per-student, per-content-node completion status from lesson events, the
// knowledge state graded and folded from practice answers, and the raw record of
// practice sessions and completed content nodes.
package domain

import "time"

// EventType identifies which tracking event a message represents. The worker
// acts on the lesson and practice events it names; any other is accepted without
// error and changes nothing.
type EventType string

const (
	EventTypeLessonStarted   EventType = "lesson.started"
	EventTypeLessonResumed   EventType = "lesson.resumed"
	EventTypeLessonCompleted EventType = "lesson.completed"
)

// TrackingEvent is the subset of a motifpath.events message this worker needs.
// It is decoded independently of the Event Ingestion Service's own domain
// types — per the monorepo's layering rules, services never share Go packages.
// Only the fields of the event type it carries are set.
type TrackingEvent struct {
	EventType     EventType
	EventID       string
	StudentID     string
	OccurredAt    time.Time
	ContentNodeID string

	// PracticeSessionID is set on practice events given in a practice session.
	PracticeSessionID string
	// PracticeAnswer is set on a practice.item_answered event.
	PracticeAnswer *PracticeAnswer
	// SessionStart is set on a practice.session_started event.
	SessionStart *PracticeSessionStart
	// SessionEnd is set on a practice.session_ended event.
	SessionEnd *PracticeSessionEnd
	// TapCheck is set on a practice.tap_check_completed event.
	TapCheck *TapCheck
	// SongChartCompletion is set on a song_chart.completed event.
	SongChartCompletion *SongChartCompletion
}
