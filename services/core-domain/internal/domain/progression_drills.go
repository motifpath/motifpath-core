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
	ProgressionNotAddableUnsupportedBarCount     ProgressionNotAddableReason = "unsupported_bar_count"
	ProgressionNotAddableUnsupportedMode         ProgressionNotAddableReason = "unsupported_mode"
	ProgressionNotAddableNoOtherDrillProgression ProgressionNotAddableReason = "no_other_drill_progression"
	ProgressionNotAddableAlreadyInDrills         ProgressionNotAddableReason = "already_in_drills"
)

// ProgressionNotAddableError refuses a progression for the drills, with every
// reason.
type ProgressionNotAddableError struct {
	Reasons []ProgressionNotAddableReason
}

func (e *ProgressionNotAddableError) Error() string {
	return fmt.Sprintf("the progression can't be added to the drills: %v", e.Reasons)
}

// drillBarCounts are the lengths drill families ask.
var drillBarCounts = []int{HeardProgressionBars, 12}

// AddedToDrills is p as a drill progression practising nodeIDs, with wrong answers
// chosen among drills, the progressions already there.
func (p Progression) AddedToDrills(nodeIDs []string, drills []Progression) (Progression, error) {
	wrongAnswers := rankedWrongAnswers(p, drills)
	var reasons []ProgressionNotAddableReason
	if p.InDrills {
		reasons = append(reasons, ProgressionNotAddableAlreadyInDrills)
	}
	if !slices.Contains(drillBarCounts, len(p.Bars)) {
		reasons = append(reasons, ProgressionNotAddableUnsupportedBarCount)
	}
	if p.Mode != string(ProgressionModeMajor) {
		reasons = append(reasons, ProgressionNotAddableUnsupportedMode)
	}
	if len(wrongAnswers) == 0 {
		reasons = append(reasons, ProgressionNotAddableNoOtherDrillProgression)
	}
	if len(reasons) > 0 {
		return Progression{}, &ProgressionNotAddableError{Reasons: reasons}
	}
	p.InDrills, p.NodeIDs, p.Confusable = true, nodeIDs, wrongAnswers
	return p, nil
}

// rankedWrongAnswers are the keys of the progressions p is best mistaken for, as
// many as a question offers: the same bar count and another chart, those in p's
// mode first, then those sharing the most chords bar by bar, then by key so the
// choice stays put. A progression with p's chart can't be told from it by reading
// or by ear, however it is spelled, so it is never one.
func rankedWrongAnswers(p Progression, drills []Progression) []string {
	var candidates []wrongAnswerCandidate
	chart := ProgressionChart(p.Bars, drillComparisonKey)
	for _, other := range drills {
		if other.Key == p.Key || len(other.Bars) != len(p.Bars) {
			continue
		}
		otherChart := ProgressionChart(other.Bars, drillComparisonKey)
		if slices.Equal(chart, otherChart) {
			continue
		}
		candidates = append(candidates, wrongAnswerCandidate{other.Key, other.Mode == p.Mode, sharedBars(chart, otherChart)})
	}
	slices.SortFunc(candidates, wrongAnswerCandidate.compare)
	keys := make([]string, 0, harmonyOptions-1)
	for _, c := range candidates[:min(len(candidates), harmonyOptions-1)] {
		keys = append(keys, c.key)
	}
	return keys
}

// wrongAnswerCandidate is a drill progression weighed as a wrong answer.
type wrongAnswerCandidate struct {
	key      string
	sameMode bool
	shared   int
}

func (c wrongAnswerCandidate) compare(other wrongAnswerCandidate) int {
	if c.sameMode != other.sameMode {
		if c.sameMode {
			return -1
		}
		return 1
	}
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
