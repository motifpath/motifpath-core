package domain

import (
	"fmt"
	"time"
)

// Position is one marked location in a Diagram: a note at a physical spot on
// the diagram's instrument. Interval, NoteName and SequenceIndex apply to
// every instrument family. String and Fret (fretted) and Key (keyboard) are
// mutually exclusive: which group is populated follows the parent Diagram's
// Instrument.Family, and a Diagram never mixes the two.
type Position struct {
	ID       string
	Interval string
	NoteName string
	// SequenceIndex is this position's place in an authored playback
	// sequence; nil means it is not part of any sequence.
	SequenceIndex *int
	String        *int
	Fret          *int
	Key           *string
}

// Diagram is a prebuilt, reusable set of positions for a scale, chord or
// similar pattern on one instrument. It stores structured positions only,
// never a rendered image: how it looks and plays back is decided by
// whatever references it.
//
// Skills/Concepts carry only ID until this Diagram is read back from the
// repository with its Skill/Concept rows joined in — the same
// construct-then-refetch convention ContentNode's Classification follows.
type Diagram struct {
	ID           string
	InstrumentID string
	Name         string
	Positions    []Position
	Skills       []Skill
	Concepts     []Concept
	CreatedAt    time.Time
}

// SkillIDs returns the ids of d.Skills, in order.
func (d Diagram) SkillIDs() []string {
	ids := make([]string, len(d.Skills))
	for i, s := range d.Skills {
		ids[i] = s.ID
	}
	return ids
}

// ConceptIDs returns the ids of d.Concepts, in order.
func (d Diagram) ConceptIDs() []string {
	ids := make([]string, len(d.Concepts))
	for i, c := range d.Concepts {
		ids[i] = c.ID
	}
	return ids
}

// NewDiagram validates and constructs a Diagram against instrument,
// stopping at the first violated invariant. Every position must use the
// coordinate shape of instrument's family — a rule no database constraint
// expresses — and, for a fretted instrument, sit on a string it has.
// Whether skillIDs/conceptIDs reference existing rows needs a repository
// round trip, so that stays an application-layer concern.
func NewDiagram(id string, instrument Instrument, name string, positions []Position, skillIDs, conceptIDs []string, now time.Time) (Diagram, error) {
	if name == "" {
		return Diagram{}, NewValidationError("name", "must not be empty")
	}
	if err := validatePositions(instrument, positions); err != nil {
		return Diagram{}, err
	}
	if len(skillIDs) == 0 {
		return Diagram{}, NewValidationError("skill_ids", "must contain at least one skill")
	}
	if len(conceptIDs) == 0 {
		return Diagram{}, NewValidationError("concept_ids", "must contain at least one concept")
	}

	skills := make([]Skill, len(skillIDs))
	for i, sid := range skillIDs {
		skills[i] = Skill{ID: sid}
	}
	concepts := make([]Concept, len(conceptIDs))
	for i, cid := range conceptIDs {
		concepts[i] = Concept{ID: cid}
	}

	return Diagram{
		ID:           id,
		InstrumentID: instrument.ID,
		Name:         name,
		Positions:    positions,
		Skills:       skills,
		Concepts:     concepts,
		CreatedAt:    now,
	}, nil
}

func validatePositions(instrument Instrument, positions []Position) error {
	if len(positions) == 0 {
		return NewValidationError("positions", "must contain at least one position")
	}
	seen := make(map[string]struct{}, len(positions))
	for i, p := range positions {
		if reason := positionProblem(instrument, p); reason != "" {
			return NewValidationError("positions", fmt.Sprintf("position %d %s", i, reason))
		}
		if _, dup := seen[p.ID]; dup {
			return NewValidationError("positions", fmt.Sprintf("position %d repeats position_id %q", i, p.ID))
		}
		seen[p.ID] = struct{}{}
	}
	return nil
}

// positionProblem returns why p is invalid for instrument, or "" if it is
// valid.
func positionProblem(instrument Instrument, p Position) string {
	if p.Interval == "" {
		return "has no interval"
	}
	if p.NoteName == "" {
		return "has no note_name"
	}
	if p.SequenceIndex != nil && *p.SequenceIndex < 0 {
		return "has a negative sequence_index"
	}

	switch instrument.Family {
	case InstrumentFamilyFretted:
		return frettedPositionProblem(instrument, p)
	case InstrumentFamilyKeyboard:
		return keyboardPositionProblem(p)
	default:
		return "belongs to an instrument with an unrecognised family"
	}
}

func frettedPositionProblem(instrument Instrument, p Position) string {
	if p.Key != nil {
		return "uses a keyboard key on a fretted instrument"
	}
	if p.String == nil || p.Fret == nil {
		return "needs both string and fret on a fretted instrument"
	}
	if *p.String < 1 || (instrument.StringCount != nil && *p.String > *instrument.StringCount) {
		return "is on a string this instrument does not have"
	}
	if *p.Fret < 0 {
		return "has a negative fret"
	}
	return ""
}

func keyboardPositionProblem(p Position) string {
	if p.String != nil || p.Fret != nil {
		return "uses string/fret on a keyboard instrument"
	}
	if p.Key == nil || *p.Key == "" {
		return "needs a key on a keyboard instrument"
	}
	return ""
}
