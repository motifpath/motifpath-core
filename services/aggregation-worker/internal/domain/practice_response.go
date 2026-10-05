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
	Type      PracticeResponseType
	NoteName  string
	String    *int
	Fret      *int
	OptionIDs []string
	LatencyMs *int
	// AudioMs (option_choice) is the audio the student hears once before
	// answering an exercise with sound; nil without audio.
	AudioMs          *int
	Rating           SelfRating
	TempoBPM         *int
	ChangesPerMinute *int
}

// TriggerContext is where an exercise answered outside a practice session was
// answered, such as a node's challenge.
type TriggerContext struct {
	Source        string
	ContentNodeID string
	ChallengeID   string
}

// PracticeAnswer is a practice.item_answered event: one answer to one item.
type PracticeAnswer struct {
	EventID    string
	StudentID  string
	OccurredAt time.Time
	// Exactly one context is set: the practice session, or the trigger context
	// of an exercise answered elsewhere on the platform.
	PracticeSessionID string
	TriggerContext    *TriggerContext
	ItemKey           string
	Response          PracticeResponse
	// TapMs is the tap time ingestion stamped on a timed answer; nil otherwise.
	TapMs *int
}
