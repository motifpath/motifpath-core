package domain

import (
	"fmt"
	"regexp"
	"time"
)

// PositionShape decides which marker shape a Position renders as. A round
// "dot" (the default) fits typical use; square/star let an author visually
// distinguish a subset of positions (e.g. every root) without relying on
// color alone.
type PositionShape string

const (
	PositionShapeDot    PositionShape = "dot"
	PositionShapeSquare PositionShape = "square"
	PositionShapeStar   PositionShape = "star"
)

// Valid reports whether s is a shape this service knows how to draw.
func (s PositionShape) Valid() bool {
	return s == PositionShapeDot || s == PositionShapeSquare || s == PositionShapeStar
}

// Position is one marked location in a Diagram: a note at a physical spot on
// the diagram's instrument. Interval, NoteName, SequenceIndex and Shape
// apply to every instrument family. String and Fret (fretted) and Key
// (keyboard) are mutually exclusive: which group is populated follows the
// parent Diagram's Instrument.Family, and a Diagram never mixes the two.
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
	// Shape is normalized to PositionShapeDot by NewDiagram when left as
	// the zero value, so a caller that doesn't care about it may omit it.
	Shape PositionShape
	// Color overrides the parent Diagram's general Color for this marker
	// only, as #RRGGBB; nil means the position uses the general color.
	Color *string
}

// LabelDisplay decides which of a Position's Interval or NoteName its
// marker shows by default when the Diagram is reopened for authoring;
// hidden shows neither. An authoring-time display preference, independent
// of a DiagramRef's own layer visibility toggle for one particular
// embedding.
type LabelDisplay string

const (
	LabelDisplayInterval LabelDisplay = "interval"
	LabelDisplayNote     LabelDisplay = "note"
	LabelDisplayHidden   LabelDisplay = "hidden"
)

// Valid reports whether d is a label display mode this service knows.
func (d LabelDisplay) Valid() bool {
	return d == LabelDisplayInterval || d == LabelDisplayNote || d == LabelDisplayHidden
}

// DiagramKind decides who may find and change a Diagram. basic diagrams
// are curated templates every teacher can use, and only an admin may create
// or update one; custom diagrams belong to their creator, who (with admins)
// alone can find them in the library and update them.
type DiagramKind string

const (
	DiagramKindBasic  DiagramKind = "basic"
	DiagramKindCustom DiagramKind = "custom"
)

// Valid reports whether k is a diagram kind this service knows.
func (k DiagramKind) Valid() bool {
	return k == DiagramKindBasic || k == DiagramKindCustom
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
	// Kind and CreatedBy are fixed at creation: an update never changes
	// them, and a copy under another kind or owner is a new Diagram.
	Kind      DiagramKind
	CreatedBy string
	// RootNote is the note this Diagram's positions are authored relative
	// to (e.g. "A"); nil means none is recorded.
	RootNote *string
	// LabelDisplay is normalized to LabelDisplayInterval by NewDiagram when
	// left as the zero value, so a caller that doesn't care about it may
	// omit it.
	LabelDisplay LabelDisplay
	// Color is the general marker color as #RRGGBB, used by every position
	// without a Color of its own; nil means none is recorded.
	Color     *string
	Positions []Position
	Skills    []Skill
	Concepts  []Concept
	CreatedAt time.Time
}

// hexColorPattern matches the #RRGGBB form the API accepts for colors.
var hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func validHexColor(c string) bool { return hexColorPattern.MatchString(c) }

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

// DiagramOptions carries NewDiagram's optional settings, named so two
// optional strings (RootNote, Color) can never be swapped by position.
// The zero value means: no recorded root note, LabelDisplayInterval, no
// general color, DiagramKindCustom.
type DiagramOptions struct {
	// RootNote is the note the positions are authored relative to; nil
	// means none is recorded.
	RootNote *string
	// LabelDisplay defaults to LabelDisplayInterval when left as the zero
	// value.
	LabelDisplay LabelDisplay
	// Color is the general marker color as #RRGGBB; nil means none is
	// recorded.
	Color *string
	// Kind defaults to DiagramKindCustom when left as the zero value.
	Kind DiagramKind
}

// NewDiagram validates and constructs a Diagram against instrument, owned
// by createdBy, stopping at the first violated invariant. Every position
// must use the coordinate shape of instrument's family — a rule no database
// constraint expresses — and, for a fretted instrument, sit on a string it
// has. opts.LabelDisplay, opts.Kind and each position's Shape default
// (LabelDisplayInterval, DiagramKindCustom, PositionShapeDot) when left as
// their zero value — a caller that doesn't care about one may omit it.
// opts.Color, when set, must be #RRGGBB. Whether skillIDs/conceptIDs
// reference existing rows needs a repository round trip, so that stays an
// application-layer concern.
func NewDiagram(id, createdBy string, instrument Instrument, name string, positions []Position, skillIDs, conceptIDs []string, opts DiagramOptions, now time.Time) (Diagram, error) {
	rootNote, labelDisplay, color, kind := opts.RootNote, opts.LabelDisplay, opts.Color, opts.Kind
	if createdBy == "" {
		return Diagram{}, NewValidationError("created_by", "must not be empty")
	}
	if kind == "" {
		kind = DiagramKindCustom
	}
	if !kind.Valid() {
		return Diagram{}, NewValidationError("kind", "must be one of: basic, custom")
	}
	if name == "" {
		return Diagram{}, NewValidationError("name", "must not be empty")
	}
	if labelDisplay == "" {
		labelDisplay = LabelDisplayInterval
	}
	if !labelDisplay.Valid() {
		return Diagram{}, NewValidationError("label_display", "must be one of: interval, note, hidden")
	}
	if color != nil && !validHexColor(*color) {
		return Diagram{}, NewValidationError("color", "must be a #RRGGBB hex color")
	}
	positions = normalizePositionShapes(positions)
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
		Kind:         kind,
		CreatedBy:    createdBy,
		RootNote:     rootNote,
		LabelDisplay: labelDisplay,
		Color:        color,
		Positions:    positions,
		Skills:       skills,
		Concepts:     concepts,
		CreatedAt:    now,
	}, nil
}

// normalizePositionShapes returns a copy of positions with PositionShapeDot
// filled in for every position left at the zero value, leaving the caller's
// slice untouched.
func normalizePositionShapes(positions []Position) []Position {
	out := make([]Position, len(positions))
	copy(out, positions)
	for i := range out {
		if out[i].Shape == "" {
			out[i].Shape = PositionShapeDot
		}
	}
	return out
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
	if !p.Shape.Valid() {
		return "has an unrecognised shape"
	}
	if p.Color != nil && !validHexColor(*p.Color) {
		return "has a malformed color (want #RRGGBB)"
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
