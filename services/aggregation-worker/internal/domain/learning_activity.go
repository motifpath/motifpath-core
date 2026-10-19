package domain

import "time"

// LearningActivity is one completion of a content node, kept raw with when it
// happened: completing a node again is another completion. Learning days, a
// streak or any other measure are evaluated from these when read.
type LearningActivity struct {
	// EventID is the id of the lesson.completed event, so an event counts once.
	EventID       string
	StudentID     string
	ContentNodeID string
	CompletedAt   time.Time
}
