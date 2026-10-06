package domain

import (
	"fmt"
	"regexp"
	"slices"
)

// DiagramMode is the mode of the key a Diagram's material belongs to. With
// the Diagram's root note it names the key (A + minor is A minor), from
// which a key signature can be derived; the signature itself is never
// stored, so it can't contradict the two.
type DiagramMode string

const (
	DiagramModeMajor      DiagramMode = "major"
	DiagramModeMinor      DiagramMode = "minor"
	DiagramModeDorian     DiagramMode = "dorian"
	DiagramModePhrygian   DiagramMode = "phrygian"
	DiagramModeLydian     DiagramMode = "lydian"
	DiagramModeMixolydian DiagramMode = "mixolydian"
	DiagramModeLocrian    DiagramMode = "locrian"
)

// Valid reports whether m is a mode this service knows.
func (m DiagramMode) Valid() bool {
	switch m {
	case DiagramModeMajor, DiagramModeMinor, DiagramModeDorian, DiagramModePhrygian,
		DiagramModeLydian, DiagramModeMixolydian, DiagramModeLocrian:
		return true
	}
	return false
}

// TimeSignature is a playback's meter as written: Beats per bar of
// BeatValue notes (4/4, 6/8, 7/8, ...). The player derives what one beat of
// the tempo is from it, so only the signature itself is stored.
type TimeSignature struct {
	Beats     int
	BeatValue int
}

// DefaultTimeSignature is the meter of a playback that doesn't give one.
var DefaultTimeSignature = TimeSignature{Beats: 4, BeatValue: 4}

// The bounds of a time signature, a tempo, a playback's steps and name,
// and how many playbacks a diagram has.
const (
	MaxTimeSignatureBeats = 16
	MaxDiagramPlaybacks   = 16
	MaxPlaybackNameLength = 200
	MinTempoBPM           = 20
	MaxTempoBPM           = 300
	MaxSequenceSteps      = 1024
	MaxNoteValueNum       = 64
	MaxNoteValueDen       = 128
)

func (s TimeSignature) valid() bool {
	return s.Beats >= 1 && s.Beats <= MaxTimeSignatureBeats && slices.Contains([]int{1, 2, 4, 8, 16, 32}, s.BeatValue)
}

// NoteValue is a length as a fraction of a whole note: 1/4 is a quarter,
// 3/8 a dotted quarter. Tuplets are plain fractions too (an eighth-note
// triplet is 1/12), so the fraction is kept exactly as the author gave it.
type NoteValue struct {
	Num int
	Den int
}

func (v NoteValue) valid() bool {
	return v.Num >= 1 && v.Num <= MaxNoteValueNum && v.Den >= 1 && v.Den <= MaxNoteValueDen
}

// Strum is how a step's positions start: together, or a few milliseconds
// apart from the lowest pitch (down) or the highest (up).
type Strum string

const (
	StrumNone Strum = "none"
	StrumDown Strum = "down"
	StrumUp   Strum = "up"
)

// Valid reports whether s is a strum this service knows.
func (s Strum) Valid() bool {
	return s == StrumNone || s == StrumDown || s == StrumUp
}

// SequenceStep is one step of a DiagramPlayback: PositionIDs sound
// together (or strummed) for Value, and the next step starts when it ends.
// A step with no positions is a rest. A position may sound in any number of
// steps.
type SequenceStep struct {
	PositionIDs []string
	Value       NoteValue
	// Strum is normalized to StrumNone by NewDiagram when left as the zero
	// value.
	Strum Strum
}

// DiagramPlayback is one named way a Diagram sounds: Steps over the
// diagram's own positions, played at TempoBPM in TimeSignature. A diagram
// may have several, e.g. a strum and an arpeggio of the same chord shape.
type DiagramPlayback struct {
	// ID is stable within its diagram, so a usage that chose this playback
	// still finds it after the playbacks are updated.
	ID string
	// Names is the playback's name in exactly the diagram's languages; no
	// two playbacks of one diagram share a name in the same language.
	Names LocalizedText
	// TempoBPM is the tempo the playback plays at by default.
	TempoBPM int
	// TimeSignature defaults to DefaultTimeSignature when left as the zero
	// value.
	TimeSignature TimeSignature
	// Steps are played in order; there is at least one.
	Steps []SequenceStep
}

// validateMode checks that mode, when set, is a known mode of a diagram
// that records its root note: a mode without a root names no key.
func validateMode(mode *DiagramMode, rootNote *string) error {
	if mode == nil {
		return nil
	}
	if !mode.Valid() {
		return NewValidationError("mode", "must be one of: major, minor, dorian, phrygian, lydian, mixolydian, locrian")
	}
	if rootNote == nil {
		return NewValidationError("mode", "requires a root_note")
	}
	return nil
}

// keyAndPlaybacks validates opts' mode against its root note, and its
// playbacks against positions and languages, the languages the diagram is
// named in. It returns the playbacks normalized and the default playback's
// id: the one opts chose, otherwise the first playback's, and nil exactly
// when there are no playbacks.
func keyAndPlaybacks(opts DiagramOptions, positions []Position, languages []string) ([]DiagramPlayback, *string, error) {
	if err := validateMode(opts.Mode, opts.RootNote); err != nil {
		return nil, nil, err
	}
	playbacks, err := validPlaybacks(opts.Playbacks, positions, languages)
	if err != nil {
		return nil, nil, err
	}
	defaultID, err := defaultPlaybackID(opts.DefaultPlaybackID, playbacks)
	if err != nil {
		return nil, nil, err
	}
	return playbacks, defaultID, nil
}

// defaultPlaybackID returns chosen when it names one of playbacks, the first
// playback's id when nothing is chosen, and nil when there are no playbacks.
func defaultPlaybackID(chosen *string, playbacks []DiagramPlayback) (*string, error) {
	if chosen == nil {
		if len(playbacks) == 0 {
			return nil, nil
		}
		first := playbacks[0].ID
		return &first, nil
	}
	if len(playbacks) == 0 {
		return nil, NewValidationError("default_playback_id", "must be absent when the diagram has no playbacks")
	}
	if !slices.ContainsFunc(playbacks, func(p DiagramPlayback) bool { return p.ID == *chosen }) {
		return nil, NewValidationError("default_playback_id", fmt.Sprintf("names %q, which is not a playback of this diagram", *chosen))
	}
	id := *chosen
	return &id, nil
}

// KeptDefaultPlaybackID is the default playback an update that doesn't
// choose one keeps: current when it is still one of playbacks, otherwise
// nil, so the first of playbacks becomes the default.
func KeptDefaultPlaybackID(current *string, playbacks []DiagramPlayback) *string {
	if current == nil || !slices.ContainsFunc(playbacks, func(p DiagramPlayback) bool { return p.ID == *current }) {
		return nil
	}
	id := *current
	return &id
}

// validPlaybacks returns a copy of playbacks normalized as playback does, or
// an error for the first that is invalid or that clashes with an earlier
// one by id or by name in some language. No playbacks is nil.
func validPlaybacks(playbacks []DiagramPlayback, positions []Position, languages []string) ([]DiagramPlayback, error) {
	if len(playbacks) == 0 {
		return nil, nil
	}
	if len(playbacks) > MaxDiagramPlaybacks {
		return nil, NewValidationError("playbacks", fmt.Sprintf("must have at most %d playbacks", MaxDiagramPlaybacks))
	}
	known := make(map[string]struct{}, len(positions))
	for _, p := range positions {
		known[p.ID] = struct{}{}
	}
	out := make([]DiagramPlayback, len(playbacks))
	ids := make(map[string]struct{}, len(playbacks))
	names := make(map[playbackName]struct{}, len(playbacks)*len(languages))
	for i, p := range playbacks {
		normalized, reason := validPlayback(p, known, languages)
		if reason == "" {
			reason = clashProblem(normalized, ids, names)
		}
		if reason != "" {
			return nil, NewValidationError("playbacks", fmt.Sprintf("playback %d %s", i, reason))
		}
		out[i] = normalized
	}
	return out, nil
}

// playbackName is a playback's name in one language.
type playbackName struct{ language, name string }

// clashProblem returns why p clashes with an earlier playback, whose ids and
// names are in ids and names, or "" if it doesn't, and then records p's.
func clashProblem(p DiagramPlayback, ids map[string]struct{}, names map[playbackName]struct{}) string {
	if _, dup := ids[p.ID]; dup {
		return fmt.Sprintf("repeats playback_id %q", p.ID)
	}
	for _, code := range p.Names.Languages() {
		if _, dup := names[playbackName{code, p.Names[code]}]; dup {
			return fmt.Sprintf("repeats the name %q in %q", p.Names[code], code)
		}
	}
	ids[p.ID] = struct{}{}
	for code, name := range p.Names {
		names[playbackName{code, name}] = struct{}{}
	}
	return ""
}

// validPlayback returns p normalized — names trimmed, the default time
// signature when none is given, StrumNone for every step without a strum —
// or why it is invalid against the known position ids and the diagram's
// languages.
func validPlayback(p DiagramPlayback, known map[string]struct{}, languages []string) (DiagramPlayback, string) {
	if p.ID == "" {
		return DiagramPlayback{}, "has no playback_id"
	}
	names, problem := localizedTextProblem(p.Names, MaxPlaybackNameLength, languages, true)
	if problem != "" {
		return DiagramPlayback{}, "names " + problem
	}
	if p.TempoBPM < MinTempoBPM || p.TempoBPM > MaxTempoBPM {
		return DiagramPlayback{}, fmt.Sprintf("has tempo_bpm %d; it must be between %d and %d", p.TempoBPM, MinTempoBPM, MaxTempoBPM)
	}
	timeSignature := p.TimeSignature
	if timeSignature == (TimeSignature{}) {
		timeSignature = DefaultTimeSignature
	}
	if !timeSignature.valid() {
		return DiagramPlayback{}, fmt.Sprintf("has time_signature %d/%d; beats must be 1-%d and beat_value one of 1, 2, 4, 8, 16, 32", timeSignature.Beats, timeSignature.BeatValue, MaxTimeSignatureBeats)
	}
	steps, problem := validSteps(p.Steps, known)
	if problem != "" {
		return DiagramPlayback{}, problem
	}
	return DiagramPlayback{ID: p.ID, Names: names, TempoBPM: p.TempoBPM, TimeSignature: timeSignature, Steps: steps}, ""
}

// validSteps returns a copy of steps with StrumNone filled in, or why they
// are invalid: none, too many, or one that names a position the diagram
// doesn't have, names one twice, or carries an invalid value or strum.
func validSteps(steps []SequenceStep, known map[string]struct{}) ([]SequenceStep, string) {
	if len(steps) == 0 {
		return nil, "has no steps"
	}
	if len(steps) > MaxSequenceSteps {
		return nil, fmt.Sprintf("must have at most %d steps", MaxSequenceSteps)
	}
	out := make([]SequenceStep, len(steps))
	for i, s := range steps {
		if reason := stepProblem(s, known); reason != "" {
			return nil, fmt.Sprintf("step %d %s", i, reason)
		}
		out[i] = SequenceStep{PositionIDs: slices.Clone(s.PositionIDs), Value: s.Value, Strum: s.Strum}
		if out[i].Strum == "" {
			out[i].Strum = StrumNone
		}
	}
	return out, ""
}

// stepProblem returns why s is invalid against the known position ids, or
// "" if it is valid.
func stepProblem(s SequenceStep, known map[string]struct{}) string {
	if !s.Value.valid() {
		return fmt.Sprintf("has value %d/%d; num must be 1-%d and den 1-%d", s.Value.Num, s.Value.Den, MaxNoteValueNum, MaxNoteValueDen)
	}
	if s.Strum != "" && !s.Strum.Valid() {
		return "has an unrecognised strum (want none, down or up)"
	}
	seen := make(map[string]struct{}, len(s.PositionIDs))
	for _, id := range s.PositionIDs {
		if _, ok := known[id]; !ok {
			return fmt.Sprintf("names position_id %q, which is not a position of this diagram", id)
		}
		if _, dup := seen[id]; dup {
			return fmt.Sprintf("names position_id %q more than once", id)
		}
		seen[id] = struct{}{}
	}
	return ""
}

// pitchPattern matches a pitch in scientific pitch notation: a letter note
// name with up to two sharps or flats, then its octave, -1 to 9.
var pitchPattern = regexp.MustCompile(`^([A-G])(bb|b|##|#)?(-1|[0-9])$`)

// ParsePitch returns pitch's MIDI note number (C4, middle C, is 60), or
// false if pitch isn't in scientific pitch notation or falls outside MIDI's
// 0-127.
func ParsePitch(pitch string) (int, bool) {
	match := pitchPattern.FindStringSubmatch(pitch)
	if match == nil {
		return 0, false
	}
	octave := -1
	if match[3] != "-1" {
		octave = int(match[3][0] - '0')
	}
	midi := (octave+1)*12 + letterSemitones[match[1]] + accidentalSemitones[match[2]]
	if midi < 0 || midi > 127 {
		return 0, false
	}
	return midi, true
}

// Semitones above C of each letter note name, and the shift of each
// accidental.
var (
	letterSemitones     = map[string]int{"C": 0, "D": 2, "E": 4, "F": 5, "G": 7, "A": 9, "B": 11}
	accidentalSemitones = map[string]int{"": 0, "#": 1, "##": 2, "b": -1, "bb": -2}
)
