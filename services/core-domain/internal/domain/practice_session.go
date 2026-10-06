package domain

import (
	"math"
	"time"
)

// PracticeRulesVersion is the version of the mastery rules this service
// reads item states under, and boxWaitDays below their review waits. Both
// copy the Aggregation Worker's, which folds the states: a rule change
// bumps the worker's version first, and this one with it once this
// service reads the new rules. A state folded under an older version is
// stale until its item is next answered and rebuilt, so it is offered as
// due; one folded under a newer version is read as it is, so the worker
// shipping first never makes every item due.
const PracticeRulesVersion = 1

// KnowledgeLevel is how well a student knows a practice item.
type KnowledgeLevel string

const (
	KnowledgeLevelNew      KnowledgeLevel = "new"
	KnowledgeLevelLearning KnowledgeLevel = "learning"
	KnowledgeLevelAccurate KnowledgeLevel = "accurate"
	KnowledgeLevelFluent   KnowledgeLevel = "fluent"
	KnowledgeLevelRetained KnowledgeLevel = "retained"
)

// lapsed is the level one step below l. A practised item never falls back
// to new: new means never practised.
func (l KnowledgeLevel) lapsed() KnowledgeLevel {
	switch l {
	case KnowledgeLevelRetained:
		return KnowledgeLevelFluent
	case KnowledgeLevelFluent:
		return KnowledgeLevelAccurate
	case KnowledgeLevelAccurate:
		return KnowledgeLevelLearning
	case KnowledgeLevelLearning, KnowledgeLevelNew:
		return l
	default:
		return l
	}
}

// boxWaitDays is the wait before the next review, by Leitner box: the
// waits the item states are folded with, kept in step with the worker's
// under PracticeRulesVersion. Box 0 is unseen.
var boxWaitDays = [...]int{0, 1, 2, 4, 8, 16, 32}

// PracticeItemState is what the evidence says about one student's item, as
// the Aggregation Worker folded it. Level is the earned level, before any
// lapse for an overdue review.
type PracticeItemState struct {
	ItemKey      string
	RulesVersion int
	Level        KnowledgeLevel
	Counted      int
	Box          int
	DueAt        *time.Time
	LastAt       *time.Time
	// Accuracy and Fluency are the shares, from 0 to 1, of the item's
	// counted answers that were correct and that were fluent.
	Accuracy float64
	Fluency  float64
	// BestCleanBPM is the best tempo rated clean since the latest teacher
	// review; nil with no clean take.
	BestCleanBPM *int
}

// Stale reports whether s was folded under mastery rules older than these.
func (s PracticeItemState) Stale() bool {
	return s.RulesVersion < PracticeRulesVersion
}

// Due reports whether s should be offered for review at now: its review is
// due, or it is stale, since answering it rebuilds it under the current
// rules.
func (s PracticeItemState) Due(now time.Time) bool {
	return s.Stale() || s.ReviewDue(now)
}

// ReviewDue reports whether s's review date has come at now, whatever rules
// it was folded under.
func (s PracticeItemState) ReviewDue(now time.Time) bool {
	return s.DueAt != nil && !s.DueAt.After(now)
}

// Weak reports whether s is weak at now: practised, not due, and shown
// below fluent.
func (s PracticeItemState) Weak(now time.Time) bool {
	return s.Counted > 0 && !s.Due(now) && s.ShownLevel(now).Rank() < KnowledgeLevelFluent.Rank()
}

// ShownLevel is the level shown at now: the earned level, one step lower
// once the review is overdue by more than the box's wait.
func (s PracticeItemState) ShownLevel(now time.Time) KnowledgeLevel {
	if s.DueAt == nil || s.Box < 0 || s.Box >= len(boxWaitDays) {
		return s.Level
	}
	if now.After(s.DueAt.AddDate(0, 0, boxWaitDays[s.Box])) {
		return s.Level.lapsed()
	}
	return s.Level
}

// PracticeItemKind is the kind of a practice item, its item key's prefix.
type PracticeItemKind string

const (
	PracticeItemKindPlayAlong     PracticeItemKind = "play_along"
	PracticeItemKindExercise      PracticeItemKind = "exercise"
	PracticeItemKindFretboardCell PracticeItemKind = "fretboard_cell"
)

// PlayAlongItemKey is the item key of playing diagramID along with its
// playback.
func PlayAlongItemKey(diagramID string) string {
	return string(PracticeItemKindPlayAlong) + ":" + diagramID
}

// ExerciseItemKey is the item key of answering the authored exercise
// exerciseID.
func ExerciseItemKey(exerciseID string) string {
	return string(PracticeItemKindExercise) + ":" + exerciseID
}

// PracticePickReason is why the composer put an item in a session.
type PracticePickReason string

const (
	PracticePickDue         PracticePickReason = "due"
	PracticePickWeak        PracticePickReason = "weak"
	PracticePickNew         PracticePickReason = "new"
	PracticePickWarmUp      PracticePickReason = "warm_up"
	PracticePickApplication PracticePickReason = "application"
	PracticePickReviewAhead PracticePickReason = "review_ahead"
	PracticePickStretch     PracticePickReason = "stretch"
)

// PlannedPlayAlong is how a play-along is played in a session.
type PlannedPlayAlong struct {
	DiagramID         string
	StartTempoBPM     int
	TargetTempoBPM    int
	BestCleanTempoBPM *int
}

// PracticeSessionItem is one pick of a composed session.
type PracticeSessionItem struct {
	ItemKey string
	Kind    PracticeItemKind
	Reason  PracticePickReason
	// NodeID is the knowledge node the pick serves; nil when none applies.
	NodeID           *string
	Level            KnowledgeLevel
	EstimatedSeconds int
	// Exactly one of PlayAlong and Exercise is set, matching Kind.
	PlayAlong *PlannedPlayAlong
	Exercise  *Exercise
}

// PracticeSessionPlan is a composed session. It is never stored: the client
// starts the session with it.
type PracticeSessionPlan struct {
	ID string
	// InstrumentID is the instrument in hand; nil for a session in the head.
	InstrumentID *string
	Minutes      int
	Items        []PracticeSessionItem
}

// The bounds of a session's length, in minutes.
const (
	MinPracticeMinutes = 1
	MaxPracticeMinutes = 60
)

// ValidatePracticeMinutes checks a requested session length.
func ValidatePracticeMinutes(minutes int) error {
	if minutes < MinPracticeMinutes || minutes > MaxPracticeMinutes {
		return NewValidationError("minutes", "must be between 1 and 60")
	}
	return nil
}

// Play-along tempo and timing rules.
const (
	// A play-along with no clean take yet starts at this share of its
	// target, rounded down to a tempoStepBPM step.
	firstStartTempoShare = 0.6
	// A warm-up starts at this share of the best clean tempo, outside the
	// tempo ladder.
	warmUpTempoShare = 0.8
	tempoStepBPM     = 5
	// A focus play-along is played over this many takes, a warm-up over
	// warmUpTakes; rating a take takes about rateSeconds.
	focusTakes  = 4
	warmUpTakes = 2
	rateSeconds = 10
)

// PlayAlongStartTempo is the tempo a play-along's ladder starts at: the best
// clean tempo, or 60% of the target rounded down to 5 BPM with no clean
// take yet, kept between the slowest playable tempo and the target.
func PlayAlongStartTempo(target int, bestClean *int) int {
	start := roundDownToStep(float64(target) * firstStartTempoShare)
	if bestClean != nil {
		start = *bestClean
	}
	return clampTempo(start, target)
}

// WarmUpTempo is a warm-up's tempo: about 80% of the best clean tempo,
// rounded down to 5 BPM, kept between the slowest playable tempo and the
// target.
func WarmUpTempo(target, bestClean int) int {
	return clampTempo(roundDownToStep(float64(bestClean)*warmUpTempoShare), target)
}

func roundDownToStep(bpm float64) int {
	return int(math.Floor(bpm/tempoStepBPM)) * tempoStepBPM
}

func clampTempo(bpm, target int) int {
	return max(MinTempoBPM, min(bpm, target))
}

// PlayAlongSeconds estimates how long playing d at tempo takes in a
// session: takes of a count-in bar plus the sequence, each followed by its
// rating.
func PlayAlongSeconds(d Diagram, tempo int, warmUp bool) int {
	takes := focusTakes
	if warmUp {
		takes = warmUpTakes
	}
	signature := d.TimeSignature
	if signature.Beats == 0 {
		signature = DefaultTimeSignature
	}
	beats := float64(signature.Beats)
	for _, step := range d.Sequence {
		beats += float64(step.Value.Num) / float64(step.Value.Den) * float64(signature.BeatValue)
	}
	take := beats * 60 / float64(tempo)
	return int(math.Ceil(float64(takes) * (take + rateSeconds)))
}

// defaultExerciseSeconds is how long an exercise with no authored estimate
// is taken to need in a session.
const defaultExerciseSeconds = 30

// ExerciseSeconds estimates how long answering e once takes in a session:
// its authored estimate, or 30 seconds without one.
func ExerciseSeconds(e Exercise) int {
	if e.EstimatedDurationSeconds != nil {
		return *e.EstimatedDurationSeconds
	}
	return defaultExerciseSeconds
}
