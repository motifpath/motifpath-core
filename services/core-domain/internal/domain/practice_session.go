package domain

import (
	"math"
	"slices"
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
	// RightByResponse counts the right answers by the way the item was
	// asked, such as name_the_note; nil before any.
	RightByResponse map[string]int
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
	// Exactly one of PlayAlong, Exercise and FretboardCell is set, matching
	// Kind.
	PlayAlong     *PlannedPlayAlong
	Exercise      *Exercise
	FretboardCell *PlannedFretboardCell
}

// DrillTemplateKey is the timed drill template answering the item
// practises: a fretboard cell's way of being asked, or an exercise's type.
// A play-along is rated, not timed, so it practises none ("").
func (i PracticeSessionItem) DrillTemplateKey() string {
	switch {
	case i.FretboardCell != nil:
		return string(PracticeItemKindFretboardCell) + ":" + string(i.FretboardCell.Drill)
	case i.Exercise != nil:
		return string(PracticeItemKindExercise) + ":" + string(i.Exercise.ExerciseType)
	}
	return ""
}

// PlanDrillTemplates lists the timed drill templates items practise, each
// once, in plan order.
func PlanDrillTemplates(items []PracticeSessionItem) []string {
	var templates []string
	for _, item := range items {
		if key := item.DrillTemplateKey(); key != "" && !slices.Contains(templates, key) {
			templates = append(templates, key)
		}
	}
	return templates
}

// MaxFeltQuestions is the most "How did it feel?" questions a session asks,
// so the end of a session stays short.
const MaxFeltQuestions = 2

// FeltQuestions picks the timed drill templates of items to ask "How did it
// feel?" about: those with the fewest felt-rated sessions so far, fewest
// first, MaxFeltQuestions at most. Felt ratings calibrate a template's fluent
// time, so the least calibrated gain the most from one more. A tie keeps the
// plan's order. A plan with no timed drill asks none.
func FeltQuestions(items []PracticeSessionItem, feltRatedSessions map[string]int) []string {
	templates := PlanDrillTemplates(items)
	slices.SortStableFunc(templates, func(a, b string) int {
		return feltRatedSessions[a] - feltRatedSessions[b]
	})
	if len(templates) > MaxFeltQuestions {
		templates = templates[:MaxFeltQuestions]
	}
	if templates == nil {
		return []string{}
	}
	return templates
}

// PlannedFretboardCell is a fretboard cell picked for a session and the way
// it is asked.
type PlannedFretboardCell struct {
	FretboardCell
	Drill FretboardDrill
}

// PracticeSessionPlan is a composed session. It is never stored: the client
// starts the session with it.
type PracticeSessionPlan struct {
	ID string
	// InstrumentID is the instrument in hand; nil for a session in the head.
	InstrumentID *string
	Minutes      int
	Items        []PracticeSessionItem
	// FeltQuestions are the timed drill templates the client asks "How did
	// it feel?" about at the end, if the student practised them: see
	// FeltQuestions.
	FeltQuestions []string
	// TapCheckDue is true when the client should offer a tap check before
	// the first item: see TapCheckDue.
	TapCheckDue bool
}

// TapCheckValidity is how long a tap check stands for the student's tap
// time: a student's device or habits change, so a plan asks for another
// once the newest is older.
const TapCheckValidity = 30 * 24 * time.Hour

// TapCheckDue reports whether a plan of items asks for a tap check: when
// it has a fretboard cell and the student's newest tap check, lastDone, is
// older than TapCheckValidity, or there is none.
func TapCheckDue(items []PracticeSessionItem, lastDone *time.Time, now time.Time) bool {
	if !slices.ContainsFunc(items, func(item PracticeSessionItem) bool { return item.Kind == PracticeItemKindFretboardCell }) {
		return false
	}
	return lastDone == nil || lastDone.Before(now.Add(-TapCheckValidity))
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
// take yet. It is kept between the slowest playable tempo and the target,
// unless the student has played it clean past the target: that tempo is
// theirs, and kept.
func PlayAlongStartTempo(target int, bestClean *int) int {
	if bestClean != nil {
		return clampTempo(*bestClean, max(target, *bestClean))
	}
	return clampTempo(roundDownToStep(float64(target)*firstStartTempoShare), target)
}

// WarmUpTempo is a warm-up's tempo: about 80% of the best clean tempo,
// rounded down to 5 BPM, kept between the slowest playable tempo and the
// target, or the best clean tempo when the student has played it past the
// target.
func WarmUpTempo(target, bestClean int) int {
	return clampTempo(roundDownToStep(float64(bestClean)*warmUpTempoShare), max(target, bestClean))
}

func roundDownToStep(bpm float64) int {
	return int(math.Floor(bpm/tempoStepBPM)) * tempoStepBPM
}

func clampTempo(bpm, ceiling int) int {
	return max(MinTempoBPM, min(bpm, ceiling))
}

// PlayAlongSeconds estimates how long playing d's default playback at tempo
// takes in a session: takes of a count-in bar plus the playback's steps,
// each followed by its rating.
func PlayAlongSeconds(d Diagram, tempo int, warmUp bool) int {
	takes := focusTakes
	if warmUp {
		takes = warmUpTakes
	}
	playback, _ := d.DefaultPlayback()
	signature := playback.TimeSignature
	if signature.Beats == 0 {
		signature = DefaultTimeSignature
	}
	beats := float64(signature.Beats)
	for _, step := range playback.Steps {
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
