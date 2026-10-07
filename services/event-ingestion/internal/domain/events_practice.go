package domain

import "regexp"

// practiceItemKeyPattern is PracticeItemKey's pattern from practice.yaml: the item kind
// prefix, then that kind's key scheme. Item kinds are an open set — a new kind adds
// its alternative here when the spec adds it.
var practiceItemKeyPattern = regexp.MustCompile(`^(` +
	`fretboard_cell:` + uuidPattern + `:[1-9][0-9]*:(0|[1-9][0-9]*)` +
	`|exercise:` + uuidPattern +
	`|play_along:` + uuidPattern +
	`|chord_change:` + uuidPattern + `:` + uuidPattern +
	`|diagram_shape:` + uuidPattern +
	`)$`)

const uuidPattern = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

// ValidPracticeItemKey reports whether key names a practice item of a known kind.
func ValidPracticeItemKey(key string) bool {
	return practiceItemKeyPattern.MatchString(key)
}

// PracticeResponseType discriminates the shapes of a PracticeResponse.
type PracticeResponseType string

const (
	PracticeResponseNameTheNote   PracticeResponseType = "name_the_note"
	PracticeResponseFindTheNote   PracticeResponseType = "find_the_note"
	PracticeResponseOptionChoice  PracticeResponseType = "option_choice"
	PracticeResponseSelfRating    PracticeResponseType = "self_rating"
	PracticeResponseNameTheShape  PracticeResponseType = "name_the_shape"
	PracticeResponseFindTheDegree PracticeResponseType = "find_the_degree"
)

// SelfRating is the student's own judgement of a take.
type SelfRating string

const (
	SelfRatingStruggled SelfRating = "struggled"
	SelfRatingAlmost    SelfRating = "almost"
	SelfRatingClean     SelfRating = "clean"
)

// PracticeResponse is the student's raw answer to one practice item, exactly as given:
// it never says whether the answer was right — the aggregation worker grades it. It
// stands in for practice.yaml's discriminated union as one flat struct, tagged by Type,
// because the events document and the Kafka message store it flat. Only the fields of
// Type's shape are set; the HTTP mapping guarantees that.
type PracticeResponse struct {
	Type PracticeResponseType

	// NoteName (name_the_note) is a letter with an optional # or b, without an octave.
	NoteName string

	// Shape (name_the_shape) is the member of its family the student named, by its
	// key in the practice drill catalog.
	Shape string

	// Interval (find_the_degree) is the degree the student was asked to find.
	Interval string

	// String and Fret (find_the_note, find_the_degree) are the cell tapped; strings
	// count from 1, the highest-pitched, and fret 0 is the open string.
	String *int
	Fret   *int

	// OptionIDs (option_choice) are the options selected, in any order.
	OptionIDs []string

	// LatencyMs is set on every shape but self_rating: milliseconds from the moment the
	// item was asked to the answer. Its presence is what makes an answer timed.
	LatencyMs *int

	// AudioMs (option_choice) is the audio the exercise asks the student to hear once
	// before answering; nil for an exercise without audio. Replays aren't counted.
	AudioMs *int

	// Rating (self_rating) is always set; TempoBPM for a play-along take,
	// ChangesPerMinute for a chord-change minute.
	Rating           SelfRating
	TempoBPM         *int
	ChangesPerMinute *int
}

// IsTimed reports whether the answer was timed, which is when the server stamps the
// student's tap time on it.
func (r PracticeResponse) IsTimed() bool {
	return r.LatencyMs != nil
}

// PracticePickReason is why the session composer put an item in a session.
type PracticePickReason string

const (
	PracticePickReasonTeacherSuggested PracticePickReason = "teacher_suggested"
	PracticePickReasonDue              PracticePickReason = "due"
	PracticePickReasonWeak             PracticePickReason = "weak"
	PracticePickReasonNew              PracticePickReason = "new"
	PracticePickReasonWarmUp           PracticePickReason = "warm_up"
	PracticePickReasonApplication      PracticePickReason = "application"
	PracticePickReasonReviewAhead      PracticePickReason = "review_ahead"
	PracticePickReasonStretch          PracticePickReason = "stretch"
)

// PlannedPracticeItem is one item of a composed session, with why it was picked.
type PlannedPracticeItem struct {
	ItemKey string
	Reason  PracticePickReason
}

// Felt is the student's answer to "How did it feel?" about a timed drill.
type Felt string

const (
	FeltEasy       Felt = "easy"
	FeltAboutRight Felt = "about_right"
	FeltHard       Felt = "hard"
)

// FeltRating is how one timed drill felt in a session. It only calibrates the drill's
// thresholds and never counts toward the student's mastery.
type FeltRating struct {
	DrillTemplateKey string
	Felt             Felt
}

// PracticeSessionStartedEvent is emitted when a student starts a composed practice
// session. It carries the plan as offered; the plan is stored nowhere else.
type PracticeSessionStartedEvent struct {
	TrackingEventBase
	PracticeSessionID string

	// InstrumentID is the instrument in the student's hands; empty when the session is
	// practised in the head.
	InstrumentID string

	Minutes      int
	PlannedItems []PlannedPracticeItem
}

func (e PracticeSessionStartedEvent) Base() TrackingEventBase { return e.TrackingEventBase }

// PracticeItemAnsweredEvent is emitted when a student answers one practice item.
type PracticeItemAnsweredEvent struct {
	TrackingEventBase
	// Exactly one context is set: the practice session the answer was given in, or,
	// for an exercise answered elsewhere such as a node's challenge, its trigger context.
	PracticeSessionID string
	TriggerContext    *TriggerContext
	ItemKey           string
	Response          PracticeResponse

	// TapMs is the student's tap time from their latest tap check before the answer.
	// Server-set on timed answers only; a client-sent value is never carried over.
	TapMs *int
}

func (e PracticeItemAnsweredEvent) Base() TrackingEventBase { return e.TrackingEventBase }

// PracticeSessionEndedEvent is emitted when a practice session ends, finished or not.
type PracticeSessionEndedEvent struct {
	TrackingEventBase
	PracticeSessionID string
	AnsweredCount     int
	LeftEarly         bool
	FeltRatings       []FeltRating
}

func (e PracticeSessionEndedEvent) Base() TrackingEventBase { return e.TrackingEventBase }

// PracticeTapCheckCompletedEvent is emitted when a student completes the tap check,
// which measures how long a tap takes them when there is nothing to work out.
type PracticeTapCheckCompletedEvent struct {
	TrackingEventBase
	MedianTapMs int
	TapCount    int
}

func (e PracticeTapCheckCompletedEvent) Base() TrackingEventBase { return e.TrackingEventBase }
