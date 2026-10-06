package domain

import "time"

// EventTypePracticeTapCheckCompleted is sent when a student completes a tap check.
const EventTypePracticeTapCheckCompleted EventType = "practice.tap_check_completed"

// TapCheck is one tap check a student completed, kept raw with when it was done:
// doing it again is another tap check. Whether a session should ask for one is
// evaluated from these when read.
type TapCheck struct {
	// EventID is the id of the practice.tap_check_completed event, so an event
	// counts once.
	EventID   string
	StudentID string
	// DoneAt is when the student did it, on their own clock.
	DoneAt time.Time
	// MedianTapMs is the median time from a fret lighting up to the student's tap.
	MedianTapMs int
	// TapCount is how many taps the median was taken over.
	TapCount int
}
