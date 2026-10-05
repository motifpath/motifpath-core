package domain

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"time"
)

// PracticeRulesVersion versions the mastery rules below. A rule change bumps
// it. A state folded under another version is rebuilt from its evidence, which is
// never modified, the next time its item is answered; until then its stored
// version tells readers it is stale.
const PracticeRulesVersion = 1

// ErrUnsupportedEvidenceSource is returned for evidence whose source the fold has no
// rules for yet: teacher reviews need teacher notes. Failing loudly keeps such
// evidence from being folded wrong.
var ErrUnsupportedEvidenceSource = errors.New("no fold rules for this evidence source yet")

// PracticeEvidence is one observation of what a student knows about one item: the
// only stored learning state. Everything else about the item is derived from it.
type PracticeEvidence struct {
	// EvidenceID is the id of the event that produced it, so an event counts once.
	EvidenceID        string
	StudentID         string
	ItemKey           string
	Source            EvidenceSource
	OccurredAt        time.Time
	PracticeSessionID string
	TriggerContext    *TriggerContext
	GraderID          string
	Response          PracticeResponse
	Correct           *bool
	LatencyMs         *int
	AudioMs           *int
	TapMs             *int
	Rating            SelfRating
	TempoBPM          *int
	ChangesPerMinute  *int
}

// ItemGoal is what fluency is measured against: the tempo a play-along is played
// at in the music, a chord change's target rate, or a timed item's fluent times. A
// nil target counts any clean take as fully fluent.
type ItemGoal struct {
	TargetTempoBPM         *int
	TargetChangesPerMinute *int
	// FluentTimes are every version of the item's drill template fluent time.
	FluentTimes []FluentTime
}

// FluentTime is one version of a drill template's fluent time: the time a fluent
// student spends knowing the answer, net of tap time and audio, in force from
// EffectiveFrom until a later version starts.
type FluentTime struct {
	Version       int
	EffectiveFrom time.Time
	FluentNetMs   int
}

// FluentTimeAt is the version in force at t, and false when t is before the first.
func (g ItemGoal) FluentTimeAt(t time.Time) (FluentTime, bool) {
	var inForce FluentTime
	found := false
	for _, v := range g.FluentTimes {
		if !v.EffectiveFrom.After(t) && (!found || v.EffectiveFrom.After(inForce.EffectiveFrom)) {
			inForce, found = v, true
		}
	}
	return inForce, found
}

// TimedJudgement is how a timed answer's time compares with the fluent time in
// force when it was given.
type TimedJudgement struct {
	// NetMs is the time spent knowing the answer: the latency less the student's
	// tap time and the audio heard once, never below zero.
	NetMs            int
	FluentTime       FluentTime
	WithinFluentTime bool
}

// JudgeTimed judges e's time, and reports false when e has no latency or no fluent
// time was in force when it was given: such an answer never counts for fluency,
// so a new version can never take back a level.
func JudgeTimed(e PracticeEvidence, goal ItemGoal) (TimedJudgement, bool) {
	if e.LatencyMs == nil {
		return TimedJudgement{}, false
	}
	fluent, ok := goal.FluentTimeAt(e.OccurredAt)
	if !ok {
		return TimedJudgement{}, false
	}
	net := *e.LatencyMs
	for _, off := range []*int{e.TapMs, e.AudioMs} {
		if off != nil {
			net -= *off
		}
	}
	net = max(net, 0)
	return TimedJudgement{NetMs: net, FluentTime: fluent, WithinFluentTime: net <= fluent.FluentNetMs}, true
}

// KnowledgeLevel is how well a student knows an item, derived from its evidence.
type KnowledgeLevel string

const (
	KnowledgeLevelNew      KnowledgeLevel = "new"
	KnowledgeLevelLearning KnowledgeLevel = "learning"
	KnowledgeLevelAccurate KnowledgeLevel = "accurate"
	KnowledgeLevelFluent   KnowledgeLevel = "fluent"
	KnowledgeLevelRetained KnowledgeLevel = "retained"
)

// boxWaitDays is the wait before the next review, by Leitner box. Box 0 is unseen.
var boxWaitDays = [...]int{0, 1, 2, 4, 8, 16, 32}

const maxBox = len(boxWaitDays) - 1

// sourceWeight is how strongly one piece of evidence pulls the weighted averages.
var sourceWeight = map[EvidenceSource]float64{
	EvidenceSourceAutoGraded:      0.3,
	EvidenceSourceSelfAssessed:    0.3,
	EvidenceSourceTeacherReviewed: 0.6,
}

// ItemFold is everything the knowledge state needs from an item's evidence so far,
// folded one piece at a time in time order. Nothing in it depends on the clock;
// what the student is shown "now" (fading, a lapsed level) is a view over it.
type ItemFold struct {
	// Attempts counts every piece of evidence; Counted only those that moved the
	// averages (exploration takes don't). Levels need counted attempts.
	Attempts int
	Counted  int
	Accuracy float64
	Fluency  float64
	// Box is the Leitner box, 0 until the first counted evidence.
	Box   int
	DueAt *time.Time
	// LastAt is the latest evidence folded: anything earlier arrives late and
	// rebuilds the item.
	LastAt *time.Time
	// BestCleanBPM and BestChangesPerMinute are the best clean measures since the
	// last teacher review: the edge above which a take that isn't clean explores.
	BestCleanBPM         *int
	BestChangesPerMinute *int
}

// Level is the level the evidence has earned, before any lapse is applied.
func (f ItemFold) Level() KnowledgeLevel {
	switch {
	case f.Counted == 0:
		return KnowledgeLevelNew
	case f.Counted >= 5 && f.Accuracy >= 0.9 && f.Fluency >= 0.8:
		if f.Box >= 5 {
			return KnowledgeLevelRetained
		}
		return KnowledgeLevelFluent
	case f.Counted >= 3 && f.Accuracy >= 0.8:
		return KnowledgeLevelAccurate
	default:
		return KnowledgeLevelLearning
	}
}

type outcome int

const (
	outcomeHit outcome = iota
	outcomeHold
	outcomeMiss
)

// reading is what one counted piece of evidence says: a hit, hold or miss, and
// its accuracy and fluency values for the weighted averages. A reading that
// doesn't judge fluency leaves the fluency average as it was.
type reading struct {
	outcome       outcome
	accuracy      float64
	fluency       float64
	judgesFluency bool
}

// FoldEvidence folds one more piece of evidence, which must not be older than
// f.LastAt (use RebuildFold for that).
func FoldEvidence(f ItemFold, e PracticeEvidence, goal ItemGoal) (ItemFold, error) {
	switch e.Source {
	case EvidenceSourceSelfAssessed:
		return foldTake(f, e, goal), nil
	case EvidenceSourceAutoGraded:
		return foldAnswer(f, e, goal)
	case EvidenceSourceTeacherReviewed:
	}
	return f, fmt.Errorf("%w: %s", ErrUnsupportedEvidenceSource, e.Source)
}

// foldAnswer folds an auto-graded answer: a right one is a hit whose fluency is the
// fluent time over the net time (capped at 1), a wrong one a miss.
func foldAnswer(f ItemFold, e PracticeEvidence, goal ItemGoal) (ItemFold, error) {
	if e.Correct == nil {
		return f, fmt.Errorf("auto-graded evidence %s has no verdict", e.EvidenceID)
	}
	f.Attempts++
	at := e.OccurredAt
	f.LastAt = &at

	judged, timed := JudgeTimed(e, goal)
	r := reading{outcome: outcomeMiss, judgesFluency: timed}
	if *e.Correct {
		r.outcome, r.accuracy, r.fluency = outcomeHit, 1, 1
		if timed && judged.NetMs > 0 {
			r.fluency = math.Min(1, float64(judged.FluentTime.FluentNetMs)/float64(judged.NetMs))
		}
	}
	f.count(r, sourceWeight[e.Source])
	f.schedule(at, r.outcome)
	return f, nil
}

// foldTake folds a self-rated take.
func foldTake(f ItemFold, e PracticeEvidence, goal ItemGoal) ItemFold {
	f.Attempts++
	at := e.OccurredAt
	f.LastAt = &at

	measure, best, target := e.TempoBPM, &f.BestCleanBPM, goal.TargetTempoBPM
	if e.ChangesPerMinute != nil {
		measure, best, target = e.ChangesPerMinute, &f.BestChangesPerMinute, goal.TargetChangesPerMinute
	}
	if exploresAboveTheEdge(e.Rating, measure, *best) {
		return f
	}
	if e.Rating == SelfRatingClean {
		*best = higher(*best, measure)
	}

	r := readRating(e.Rating, goalRatio(measure, target))
	f.count(r, sourceWeight[e.Source])
	f.schedule(at, r.outcome)
	return f
}

// exploresAboveTheEdge reports a take that isn't clean above the best clean
// measure. The tempo ladder pushes every session to the student's edge, so such
// a take is exploring, not forgetting, and doesn't count against them.
func exploresAboveTheEdge(rating SelfRating, measure, best *int) bool {
	return rating != SelfRatingClean && measure != nil && best != nil && *measure > *best
}

func higher(best, measure *int) *int {
	if measure == nil || (best != nil && *best >= *measure) {
		return best
	}
	v := *measure
	return &v
}

// goalRatio is how far a take got toward the item's goal, capped at 1.
func goalRatio(measure, target *int) float64 {
	if measure == nil || target == nil || *target <= 0 {
		return 1
	}
	return math.Min(1, float64(*measure)/float64(*target))
}

// readRating maps a rating onto the three readings: clean is a hit, almost
// a hold at half value, struggled a miss.
func readRating(rating SelfRating, ratio float64) reading {
	switch rating {
	case SelfRatingClean:
		return reading{outcome: outcomeHit, accuracy: 1, fluency: ratio, judgesFluency: true}
	case SelfRatingAlmost:
		return reading{outcome: outcomeHold, accuracy: 0.5, fluency: ratio * 0.5, judgesFluency: true}
	case SelfRatingStruggled:
		return reading{outcome: outcomeMiss, judgesFluency: true}
	}
	return reading{outcome: outcomeMiss, judgesFluency: true}
}

// count moves the weighted averages toward r; the first counted evidence sets them.
// A reading that doesn't judge fluency moves accuracy only.
func (f *ItemFold) count(r reading, weight float64) {
	if f.Counted == 0 {
		f.Accuracy = r.accuracy
		if r.judgesFluency {
			f.Fluency = r.fluency
		}
	} else {
		f.Accuracy += weight * (r.accuracy - f.Accuracy)
		if r.judgesFluency {
			f.Fluency += weight * (r.fluency - f.Fluency)
		}
	}
	f.Counted++
}

// schedule applies the Leitner move: a miss goes back to box 1, a first sighting
// enters box 1, a hit on a due item moves up one box. A hit before it's due, or a
// hold, leaves the schedule as it was.
func (f *ItemFold) schedule(at time.Time, o outcome) {
	switch {
	case o == outcomeMiss:
		f.Box = 1
	case f.Box == 0:
		f.Box = 1
	case o == outcomeHit && f.DueAt != nil && !at.Before(*f.DueAt):
		f.Box = min(maxBox, f.Box+1)
	default:
		return
	}
	due := at.AddDate(0, 0, boxWaitDays[f.Box])
	f.DueAt = &due
}

// RebuildFold folds an item's whole history from scratch, in time order. Evidence
// sharing a timestamp folds in the order given, which callers keep as arrival order.
func RebuildFold(history []PracticeEvidence, goal ItemGoal) (ItemFold, error) {
	snapshots, err := DailySnapshots(history, goal)
	if err != nil || len(snapshots) == 0 {
		return ItemFold{}, err
	}
	return snapshots[len(snapshots)-1].Fold, nil
}

// ItemSnapshot is an item's state at the end of a UTC day on which it was
// practised: the record progress over a week is measured against.
type ItemSnapshot struct {
	Day  time.Time
	Fold ItemFold
}

// SnapshotDay is the UTC day an answer given at t belongs to, as its midnight.
func SnapshotDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DailySnapshots folds an item's whole history like RebuildFold, keeping the state
// at the end of each day that has evidence, in day order.
func DailySnapshots(history []PracticeEvidence, goal ItemGoal) ([]ItemSnapshot, error) {
	ordered := slices.Clone(history)
	slices.SortStableFunc(ordered, func(a, b PracticeEvidence) int { return a.OccurredAt.Compare(b.OccurredAt) })

	var (
		f         ItemFold
		snapshots []ItemSnapshot
	)
	for _, e := range ordered {
		var err error
		if f, err = FoldEvidence(f, e, goal); err != nil {
			return nil, err
		}
		day := SnapshotDay(e.OccurredAt)
		if n := len(snapshots); n > 0 && snapshots[n-1].Day.Equal(day) {
			snapshots[n-1].Fold = f
		} else {
			snapshots = append(snapshots, ItemSnapshot{Day: day, Fold: f})
		}
	}
	return snapshots, nil
}
