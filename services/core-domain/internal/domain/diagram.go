package domain

import (
	"fmt"
	"regexp"
	"slices"
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
	// CustomLabel is shown inside the marker instead of the interval or note
	// name the Diagram's LabelDisplay picks, at most MaxCustomLabelLength
	// characters per language; nil means none.
	CustomLabel LocalizedText
	// Note explains this position to a reader, at most MaxPositionNoteLength
	// characters per language; nil means none.
	Note LocalizedText
}

// Region is a highlighted area of a Diagram, drawn as a band behind its
// markers and captioned by Description. Like a Position's, its coordinates
// follow the parent Diagram's Instrument.Family: FretStart/FretEnd, and
// optionally StringStart/StringEnd, for a fretted instrument; KeyStart/KeyEnd
// for a keyboard one. Every range is inclusive, and regions may overlap.
type Region struct {
	ID        string
	FretStart *int
	FretEnd   *int
	// StringStart and StringEnd are set together or not at all; neither
	// means the band covers every string.
	StringStart *int
	StringEnd   *int
	KeyStart    *string
	KeyEnd      *string
	// Description is the band's caption, at most
	// MaxRegionDescriptionLength characters per language.
	Description LocalizedText
	// Color is the band's tint as #RRGGBB; nil means the default tint.
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
	// Names is the diagram's name per language: every offered language for
	// a basic diagram, at least one for a custom one.
	Names LocalizedText
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
	// Regions are drawn in order, later ones on top; empty when there are
	// none.
	Regions   []Region
	Skills    []Skill
	Concepts  []Concept
	CreatedAt time.Time
}

// IntervalCodes is every interval a Position may carry: canonical codes
// (R is the root), not display text — clients show each in the viewer's
// language. Enharmonic codes stay distinct (#4 vs b5) because the author's
// spelling carries musical meaning.
var IntervalCodes = []string{
	"R", "b2", "2", "#2", "b3", "3", "4", "#4", "b5", "5", "#5", "b6", "6",
	"bb7", "b7", "7", "b9", "9", "#9", "11", "#11", "b13", "13",
}

// noteNamePattern matches a letter note name in every locale: A-G with up to
// two sharps or flats.
var noteNamePattern = regexp.MustCompile(`^[A-G](bb|b|##|#)?$`)

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
	// Regions are the diagram's highlighted areas, in drawing order; nil
	// means none.
	Regions []Region
}

// The longest each piece of diagram text may be, in characters, in any one
// language. A custom label has to fit inside a marker.
const (
	MaxDiagramNameLength       = 200
	MaxCustomLabelLength       = 2
	MaxPositionNoteLength      = 280
	MaxRegionDescriptionLength = 60
)

// diagramNames validates names against languages, the languages MotifPath
// offers: a basic diagram is shared with every teacher, so it needs a name in
// every one of them; a custom diagram needs at least one.
func diagramNames(names map[string]string, languages []string, kind DiagramKind) (LocalizedText, error) {
	if kind == DiagramKindBasic {
		return NewLocalizedText("names", names, MaxDiagramNameLength, languages)
	}
	return NewLocalizedTextFrom("names", names, MaxDiagramNameLength, languages)
}

// NewDiagram validates and constructs a Diagram against instrument, owned
// by createdBy, stopping at the first violated invariant. languages are the
// languages MotifPath offers, which names are checked against. Every position
// must use the coordinate shape of instrument's family — a rule no database
// constraint expresses — and, for a fretted instrument, sit on a string it
// has. opts.LabelDisplay, opts.Kind and each position's Shape default
// (LabelDisplayInterval, DiagramKindCustom, PositionShapeDot) when left as
// their zero value — a caller that doesn't care about one may omit it.
// opts.Color, when set, must be #RRGGBB. Every other piece of text — a
// position's CustomLabel and Note, a region's Description — must be written
// in exactly the languages names is, so the diagram reads completely in each
// of them. Regions follow the instrument's family like positions do. Whether skillIDs/conceptIDs
// reference existing rows needs a repository round trip, so that stays an
// application-layer concern.
func NewDiagram(id, createdBy string, instrument Instrument, names map[string]string, languages []string, positions []Position, skillIDs, conceptIDs []string, opts DiagramOptions, now time.Time) (Diagram, error) {
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
	localizedNames, err := diagramNames(names, languages, kind)
	if err != nil {
		return Diagram{}, err
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
	positions, regions, err := annotations(instrument, positions, opts.Regions, localizedNames.Languages())
	if err != nil {
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
		Names:        localizedNames,
		Kind:         kind,
		CreatedBy:    createdBy,
		RootNote:     rootNote,
		LabelDisplay: labelDisplay,
		Color:        color,
		Positions:    positions,
		Regions:      regions,
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
	if !slices.Contains(IntervalCodes, p.Interval) {
		return fmt.Sprintf("has interval %q, which is not one of the canonical interval codes", p.Interval)
	}
	if p.NoteName == "" {
		return "has no note_name"
	}
	if !noteNamePattern.MatchString(p.NoteName) {
		return fmt.Sprintf("has note_name %q, which is not a letter name (A-G with up to two sharps or flats)", p.NoteName)
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

// annotations validates the diagram's text beyond its names — positions'
// custom labels and notes, regions and their descriptions — against
// instrument and languages, the languages the diagram is named in, and
// returns positions and regions with that text trimmed.
func annotations(instrument Instrument, positions []Position, regions []Region, languages []string) ([]Position, []Region, error) {
	positions, err := positionsWithText(positions, languages)
	if err != nil {
		return nil, nil, err
	}
	regions, err = validRegions(instrument, regions, languages)
	if err != nil {
		return nil, nil, err
	}
	return positions, regions, nil
}

// positionsWithText returns positions with every custom label and note
// trimmed, or an error for the first one that isn't written in exactly
// languages or doesn't fit its length limit. positions is already a copy.
func positionsWithText(positions []Position, languages []string) ([]Position, error) {
	for i := range positions {
		label, problem := optionalText(positions[i].CustomLabel, MaxCustomLabelLength, languages)
		if problem != "" {
			return nil, NewValidationError("positions", fmt.Sprintf("position %d custom_label %s", i, problem))
		}
		note, problem := optionalText(positions[i].Note, MaxPositionNoteLength, languages)
		if problem != "" {
			return nil, NewValidationError("positions", fmt.Sprintf("position %d note %s", i, problem))
		}
		positions[i].CustomLabel, positions[i].Note = label, note
	}
	return positions, nil
}

// optionalText is text trimmed and checked against exactly languages, or
// nil for absent text; problem is why it can't be accepted.
func optionalText(text LocalizedText, maxLength int, languages []string) (_ LocalizedText, problem string) {
	if text == nil {
		return nil, ""
	}
	return localizedTextProblem(text, maxLength, languages, true)
}

// validRegions returns a copy of regions with every description trimmed, or
// an error for the first region that is invalid on instrument or whose
// description isn't written in exactly languages. No regions is nil.
func validRegions(instrument Instrument, regions []Region, languages []string) ([]Region, error) {
	if len(regions) == 0 {
		return nil, nil
	}
	out := make([]Region, len(regions))
	seen := make(map[string]struct{}, len(regions))
	for i, r := range regions {
		if reason := regionProblem(instrument, r); reason != "" {
			return nil, NewValidationError("regions", fmt.Sprintf("region %d %s", i, reason))
		}
		if _, dup := seen[r.ID]; dup {
			return nil, NewValidationError("regions", fmt.Sprintf("region %d repeats region_id %q", i, r.ID))
		}
		seen[r.ID] = struct{}{}
		if r.Description == nil {
			return nil, NewValidationError("regions", fmt.Sprintf("region %d has no description", i))
		}
		description, problem := optionalText(r.Description, MaxRegionDescriptionLength, languages)
		if problem != "" {
			return nil, NewValidationError("regions", fmt.Sprintf("region %d description %s", i, problem))
		}
		r.Description = description
		out[i] = r
	}
	return out, nil
}

// regionProblem returns why r's coordinates or color are invalid for
// instrument, or "" if they are valid.
func regionProblem(instrument Instrument, r Region) string {
	if r.Color != nil && !validHexColor(*r.Color) {
		return "has a malformed color (want #RRGGBB)"
	}
	switch instrument.Family {
	case InstrumentFamilyFretted:
		return frettedRegionProblem(instrument, r)
	case InstrumentFamilyKeyboard:
		return keyboardRegionProblem(r)
	default:
		return "belongs to an instrument with an unrecognised family"
	}
}

func frettedRegionProblem(instrument Instrument, r Region) string {
	if r.KeyStart != nil || r.KeyEnd != nil {
		return "uses a key range on a fretted instrument"
	}
	if r.FretStart == nil || r.FretEnd == nil {
		return "needs both fret_start and fret_end on a fretted instrument"
	}
	if *r.FretStart < 0 {
		return "has a negative fret"
	}
	if *r.FretStart > *r.FretEnd {
		return "runs backwards: fret_start is after fret_end"
	}
	if (r.StringStart == nil) != (r.StringEnd == nil) {
		return "needs both string_start and string_end, or neither"
	}
	if r.StringStart == nil {
		return ""
	}
	if *r.StringStart < 1 || (instrument.StringCount != nil && *r.StringEnd > *instrument.StringCount) {
		return "covers a string this instrument does not have"
	}
	if *r.StringStart > *r.StringEnd {
		return "runs backwards: string_start is after string_end"
	}
	return ""
}

func keyboardRegionProblem(r Region) string {
	if r.FretStart != nil || r.FretEnd != nil || r.StringStart != nil || r.StringEnd != nil {
		return "uses frets or strings on a keyboard instrument"
	}
	if r.KeyStart == nil || r.KeyEnd == nil {
		return "needs both key_start and key_end on a keyboard instrument"
	}
	start, ok := keyPitch(*r.KeyStart)
	if !ok {
		return fmt.Sprintf("has key_start %q, which is not a note name and octave (e.g. C4)", *r.KeyStart)
	}
	end, ok := keyPitch(*r.KeyEnd)
	if !ok {
		return fmt.Sprintf("has key_end %q, which is not a note name and octave (e.g. B4)", *r.KeyEnd)
	}
	if start > end {
		return "runs backwards: key_start is above key_end"
	}
	return ""
}

// keyPattern matches a keyboard key: a letter note name with up to two
// sharps or flats, then a single-digit octave (e.g. "C4", "Bb3").
var keyPattern = regexp.MustCompile(`^([A-G])(bb|b|##|#)?([0-9])$`)

// keyPitch returns key's pitch as semitones above C0, so two keys compare
// by how high they sound, whatever their spelling.
func keyPitch(key string) (int, bool) {
	match := keyPattern.FindStringSubmatch(key)
	if match == nil {
		return 0, false
	}
	letterSemitones := map[string]int{"C": 0, "D": 2, "E": 4, "F": 5, "G": 7, "A": 9, "B": 11}
	accidentalSemitones := map[string]int{"": 0, "#": 1, "##": 2, "b": -1, "bb": -2}
	octave := int(match[3][0] - '0')
	return octave*12 + letterSemitones[match[1]] + accidentalSemitones[match[2]], true
}
