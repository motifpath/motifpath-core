package domain

import (
	"slices"
	"time"
)

// SubjectRungs finds where a student is on the ladder of one subject, from
// its material for a session, the keys of its items by the rung they train.
// Its current rung is the lowest rung with material whose items aren't yet
// accurate, as a node's level is rolled up; with every rung accurate, the
// top one. Its new items come from the current rung and the next rung with
// material, so a session never asks more than two adjacent steps of it.
// With no material, there is no rung.
func SubjectRungs(material map[PracticeRung][]string, states map[string]PracticeItemState, now time.Time) (PracticeRung, []PracticeRung) {
	var rungs []PracticeRung
	for _, r := range PracticeRungs {
		if len(material[r]) > 0 {
			rungs = append(rungs, r)
		}
	}
	if len(rungs) == 0 {
		return "", nil
	}
	current := len(rungs) - 1
	for i, r := range rungs {
		if level := RollUpNode(material[r], states, now).Level; level == nil || level.Rank() < KnowledgeLevelAccurate.Rank() {
			current = i
			break
		}
	}
	return rungs[current], slices.Clone(rungs[current:min(current+2, len(rungs))])
}

// PracticeSubjectList is what a student can choose to practise with one
// instrument in hand, or in the head when InstrumentID is nil.
type PracticeSubjectList struct {
	InstrumentID *string
	// Groups holds one group per area with a subject, in catalog order.
	Groups []PracticeSubjectGroup
}

// PracticeSubjectGroup is the subjects of one area, a top-level node, in
// catalog order.
type PracticeSubjectGroup struct {
	Area     KnowledgeNode
	Subjects []PracticeSubject
}

// PracticeSubject is a node the student can choose to practise, with where
// they stand on it.
type PracticeSubject struct {
	Node KnowledgeNode
	// OnPath reports whether it is a skill of one of the student's paths.
	OnPath bool
	// ItemCount counts the items of its subtree a session could pick.
	ItemCount int
	// Level is the student's level over those items: the best of their
	// instruments in the head; nil for a wide node, one with child nodes.
	Level       *KnowledgeLevel
	CurrentRung PracticeRung
}
