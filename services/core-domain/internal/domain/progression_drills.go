package domain

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// ProgressionNotAddableReason says why a progression can't enter the drills.
type ProgressionNotAddableReason string

const (
	// ProgressionNotAddableUnsupportedBarCount refuses a single bar, which is a
	// chord, not a progression.
	ProgressionNotAddableUnsupportedBarCount ProgressionNotAddableReason = "unsupported_bar_count"
	ProgressionNotAddableAlreadyInDrills     ProgressionNotAddableReason = "already_in_drills"
)

// ProgressionNotAddableError refuses a progression for the drills, with every
// reason.
type ProgressionNotAddableError struct {
	Reasons []ProgressionNotAddableReason
}

func (e *ProgressionNotAddableError) Error() string {
	return fmt.Sprintf("the progression can't be added to the drills: %v", e.Reasons)
}

// minDrillBars is the fewest bars a drill progression has: one bar is a chord,
// which the diatonic chord drills already ask.
const minDrillBars = 2

// AddedToDrills is p as a drill progression practising nodeIDs, with wrong answers
// chosen among drills, the progressions already there. It may have none: drills
// then make up its wrong answers each time they ask it.
func (p Progression) AddedToDrills(nodeIDs []string, drills []Progression) (Progression, error) {
	var reasons []ProgressionNotAddableReason
	if p.InDrills {
		reasons = append(reasons, ProgressionNotAddableAlreadyInDrills)
	}
	if len(p.Bars) < minDrillBars {
		reasons = append(reasons, ProgressionNotAddableUnsupportedBarCount)
	}
	if len(reasons) > 0 {
		return Progression{}, &ProgressionNotAddableError{Reasons: reasons}
	}
	p.InDrills, p.NodeIDs, p.Confusable = true, nodeIDs, rankedWrongAnswers(p, drills)
	return p, nil
}

// progressionOptions is how many options a question about a progression of
// bars offers: four up to four bars, two for a longer one, whose charts take
// longer to compare.
func progressionOptions(bars int) int {
	if bars > HeardProgressionBars {
		return longProgressionOptions
	}
	return harmonyOptions
}

const longProgressionOptions = 2

// rankedWrongAnswers are the keys of the progressions p is best mistaken for, as
// many as a question offers: the same bar count, the same mode and another chart,
// those sharing the most chords bar by bar first, then by key so the choice stays
// put. Another mode is ruled out by its first chord, and a progression with p's
// chart can't be told from it by reading or by ear, however it is spelled, so
// neither is ever one.
func rankedWrongAnswers(p Progression, drills []Progression) []string {
	var candidates []wrongAnswerCandidate
	chart := ProgressionChart(p.Bars, drillComparisonKey)
	for _, other := range drills {
		if other.Key == p.Key || other.Mode != p.Mode || len(other.Bars) != len(p.Bars) {
			continue
		}
		otherChart := ProgressionChart(other.Bars, drillComparisonKey)
		if slices.Equal(chart, otherChart) {
			continue
		}
		candidates = append(candidates, wrongAnswerCandidate{other.Key, sharedBars(chart, otherChart)})
	}
	slices.SortFunc(candidates, wrongAnswerCandidate.compare)
	most := progressionOptions(len(p.Bars)) - 1
	keys := make([]string, 0, most)
	for _, c := range candidates[:min(len(candidates), most)] {
		keys = append(keys, c.key)
	}
	return keys
}

// wrongAnswerCandidate is a drill progression weighed as a wrong answer.
type wrongAnswerCandidate struct {
	key    string
	shared int
}

func (c wrongAnswerCandidate) compare(other wrongAnswerCandidate) int {
	return cmp.Or(cmp.Compare(other.shared, c.shared), strings.Compare(c.key, other.key))
}

// sharedBars counts the bars two charts of one length have in common.
func sharedBars(a, b []string) int {
	shared := 0
	for i := range a {
		if a[i] == b[i] {
			shared++
		}
	}
	return shared
}

// drillComparisonKey is the key charts are compared in: two progressions share a
// chord in every key or in none, so any key gives the same answer.
const drillComparisonKey = "C"
