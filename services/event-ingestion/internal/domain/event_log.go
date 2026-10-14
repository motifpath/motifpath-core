package domain

import (
	"fmt"
	"time"
)

const (
	defaultEventLogLimit = 20
	maxEventLogLimit     = 100
)

// eventTypeGroups are the groups of event types the event log can be
// filtered by.
var eventTypeGroups = map[string][]EventType{
	"practice_sessions": {EventTypePracticeSessionStarted, EventTypePracticeSessionEnded},
	"answers":           {EventTypePracticeItemAnswered, EventTypePracticeTapCheckCompleted},
	"lessons":           {EventTypeLessonStarted, EventTypeLessonResumed, EventTypeLessonCompleted},
	"exercises":         {EventTypeExerciseStarted, EventTypeExerciseProgress, EventTypeExerciseEnded},
	"song_charts":       {EventTypeSongChartOpened, EventTypeSongChartChordViewed, EventTypeSongChartCompleted},
}

// LoggedEvent is a tracking event as it was received, with when it was.
type LoggedEvent struct {
	ReceivedAt time.Time
	Event      TrackingEvent
}

// EventLogQuery is one page of the event log to read, newest received
// first.
type EventLogQuery struct {
	// StudentID is empty for every student's events.
	StudentID string
	// EventTypes is empty for events of every type.
	EventTypes []EventType
	Limit      int
	Offset     int
}

// EventLogPage is one page of the event log, with the count of all events
// the query matches.
type EventLogPage struct {
	Items  []LoggedEvent
	Total  int
	Limit  int
	Offset int
}

// InvalidQueryError names the query parameter a read can't be made with.
type InvalidQueryError struct {
	Field  string
	Reason string
}

func (e *InvalidQueryError) Error() string {
	return fmt.Sprintf("%s %s", e.Field, e.Reason)
}

// NewEventLogQuery builds a query from the optional request parameters:
// typeGroup names a group of event types, limit is 1 to 100 (default 20)
// and offset is 0 or more (default 0).
func NewEventLogQuery(studentID, typeGroup *string, limit, offset *int) (EventLogQuery, error) {
	q := EventLogQuery{Limit: defaultEventLogLimit}
	if studentID != nil {
		q.StudentID = *studentID
	}
	if typeGroup != nil {
		types, ok := eventTypeGroups[*typeGroup]
		if !ok {
			return EventLogQuery{}, &InvalidQueryError{Field: "type_group", Reason: "must be one of practice_sessions, answers, lessons, exercises, song_charts"}
		}
		q.EventTypes = types
	}
	if limit != nil {
		if *limit < 1 || *limit > maxEventLogLimit {
			return EventLogQuery{}, &InvalidQueryError{Field: "limit", Reason: "must be between 1 and 100"}
		}
		q.Limit = *limit
	}
	if offset != nil {
		if *offset < 0 {
			return EventLogQuery{}, &InvalidQueryError{Field: "offset", Reason: "must not be negative"}
		}
		q.Offset = *offset
	}
	return q, nil
}
