package domain

import "time"

// EventTypePracticeItemAnswered is the practice event this worker grades into evidence.
const EventTypePracticeItemAnswered EventType = "practice.item_answered"

// PracticeResponseType discriminates the shapes of a PracticeResponse.
type PracticeResponseType string

const (
	PracticeResponseNameTheNote     PracticeResponseType = "name_the_note"
	PracticeResponseFindTheNote     PracticeResponseType = "find_the_note"
	PracticeResponseOptionChoice    PracticeResponseType = "option_choice"
	PracticeResponseSelfRating      PracticeResponseType = "self_rating"
	PracticeResponseNameTheShape    PracticeResponseType = "name_the_shape"
	PracticeResponseFindTheDegree   PracticeResponseType = "find_the_degree"
	PracticeResponseGeneratedChoice PracticeResponseType = "generated_choice"
	PracticeResponseTimedTaps       PracticeResponseType = "timed_taps"
	PracticeResponseMultiTap        PracticeResponseType = "multi_tap"
	// PracticeResponseNotAnswered is an item skipped because it couldn't be shown. It
	// fits every kind and is never graded.
	PracticeResponseNotAnswered PracticeResponseType = "not_answered"
)

// NotAnsweredReason is why an item was skipped without an answer.
type NotAnsweredReason string

// TimedTap is one tap on the fretboard during a take, milliseconds after the
// downbeat of its first bar.
type TimedTap struct {
	AtMs   int
	String int
	Fret   int
}

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
	Type     PracticeResponseType
	NoteName string
	// Choices (name_the_note) are the four notes it was named among, as shown; nil
	// for an answer given on the earlier twelve-note keypad.
	Choices []string
	// Reason (not_answered) is why the item was skipped.
	Reason NotAnsweredReason
	// Way, Options and Choice (generated_choice) are how the question was asked, its
	// options as shown and the one picked.
	Way     string
	Options []string
	Choice  string
	// MusicKey (generated_choice, timed_taps) is the major key the question was
	// asked in, when its item doesn't fix one.
	MusicKey string
	// Tone and Taps (timed_taps) are the chord tone asked and every tap of the take;
	// TempoBPM is the tempo it played at.
	Tone string
	Taps []TimedTap
	// FretWindowFrom and Cells (multi_tap) are the window's lowest fret and every
	// place tapped.
	FretWindowFrom *int
	Cells          []AnswerCell
	// Shape (name_the_shape) is the member of its family the student named.
	Shape string
	// Interval (find_the_degree) is the degree the student was asked to find.
	Interval  string
	String    *int
	Fret      *int
	OptionIDs []string
	LatencyMs *int
	// AudioMs (option_choice, generated_choice) is the audio the student hears once
	// before answering; nil without audio.
	AudioMs  *int
	Rating   SelfRating
	TempoBPM *int
	// NoteValue (self_rating) is the note value a play-along take was played at, as
	// given; nil when the client sent none.
	NoteValue        *NoteValue
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
