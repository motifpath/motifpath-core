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

// RungLevel is a student's level on one rung of a subject: the highest
// level at least 80% of the subject's items at that rung reach.
type RungLevel struct {
	Rung  PracticeRung
	Level KnowledgeLevel
}

// SubjectProgress is where a student stands on one subject for a session:
// its current rung (empty with no material) and their level on every rung
// with material, lowest first.
type SubjectProgress struct {
	NodeID      string
	CurrentRung PracticeRung
	Rungs       []RungLevel
}

// NewSubjectProgress finds where a student stands on the subject nodeID
// from its material for a session, the keys of its items by the rung they
// train, and their states: the current rung as SubjectRungs finds it, and
// each rung's level rolled up as a node's is.
func NewSubjectProgress(nodeID string, material map[PracticeRung][]string, states map[string]PracticeItemState, now time.Time) SubjectProgress {
	p := SubjectProgress{NodeID: nodeID, Rungs: []RungLevel{}}
	p.CurrentRung, _ = SubjectRungs(material, states, now)
	for _, r := range PracticeRungs {
		if len(material[r]) == 0 {
			continue
		}
		level := KnowledgeLevelNew
		if l := RollUpNode(material[r], states, now).Level; l != nil {
			level = *l
		}
		p.Rungs = append(p.Rungs, RungLevel{Rung: r, Level: level})
	}
	return p
}

// Current is the student's level on the current rung; new with no material.
func (p SubjectProgress) Current() RungLevel {
	for _, r := range p.Rungs {
		if r.Rung == p.CurrentRung {
			return r
		}
	}
	return RungLevel{Rung: p.CurrentRung, Level: KnowledgeLevelNew}
}

// PracticeSessionSubjectProgress is where a student stands on a session's
// subjects now, for its summary. CaughtUp reports whether the session's end
// has been folded: a student's events are folded in order, an answer before
// it is recorded in its session, so from then on the levels hold every
// answer that arrived before the end.
type PracticeSessionSubjectProgress struct {
	CaughtUp bool
	Subjects []SubjectProgress
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
