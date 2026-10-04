package domain

import "time"

// EventTypePracticeItemAnswered is the practice event this worker grades into evidence.
const EventTypePracticeItemAnswered EventType = "practice.item_answered"

// PracticeResponseType discriminates the shapes of a PracticeResponse.
type PracticeResponseType string

const (
	PracticeResponseNameTheNote  PracticeResponseType = "name_the_note"
	PracticeResponseFindTheNote  PracticeResponseType = "find_the_note"
	PracticeResponseOptionChoice PracticeResponseType = "option_choice"
	PracticeResponseSelfRating   PracticeResponseType = "self_rating"
)

// SelfRating is the student's own judgement of a take.
type SelfRating string

const (
	SelfRatingStruggled SelfRating = "struggled"
	SelfRatingAlmost    SelfRating = "almost"
	SelfRatingClean     SelfRating = "clean"
)

func (r SelfRating) valid() bool {
	switch r {
	case SelfRatingStruggled, SelfRatingAlmost, SelfRatingClean:
		return true
	}
	return false
}

// PracticeResponse is the student's raw answer, exactly as given. The answer takes
// one of several shapes; they are flattened into one struct tagged by Type, as
// ingestion publishes them, and only the fields of Type's shape are set.
type PracticeResponse struct {
	Type             PracticeResponseType
	NoteName         string
	String           *int
	Fret             *int
	OptionIDs        []string
	LatencyMs        *int
	Rating           SelfRating
	TempoBPM         *int
	ChangesPerMinute *int
}

// PracticeAnswer is a practice.item_answered event: one answer to one item.
type PracticeAnswer struct {
	EventID           string
	StudentID         string
	OccurredAt        time.Time
	PracticeSessionID string
	ItemKey           string
	Response          PracticeResponse
	// TapMs is the tap time ingestion stamped on a timed answer; nil otherwise.
	TapMs *int
}
