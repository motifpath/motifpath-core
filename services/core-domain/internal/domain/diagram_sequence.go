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

// TimeSignature is a Diagram's meter as written: Beats per bar of
// BeatValue notes (4/4, 6/8, 7/8, ...). The player derives what one beat of
// the tempo is from it, so only the signature itself is stored.
type TimeSignature struct {
	Beats     int
	BeatValue int
}

// DefaultTimeSignature is the meter of a Diagram that doesn't give one.
var DefaultTimeSignature = TimeSignature{Beats: 4, BeatValue: 4}

// The bounds of a time signature, a tempo and a sequence.
const (
	MaxTimeSignatureBeats = 16
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

// SequenceStep is one step of a Diagram's playback: PositionIDs sound
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

// keyAndPlayback validates opts' mode against its root note, and its time
// signature, sequence and tempo against positions, returning the time
// signature and sequence normalized as playback does.
func keyAndPlayback(opts DiagramOptions, positions []Position) (TimeSignature, []SequenceStep, error) {
	if err := validateMode(opts.Mode, opts.RootNote); err != nil {
		return TimeSignature{}, nil, err
	}
	return playback(opts.TimeSignature, opts.Sequence, opts.TempoBPM, positions)
}

// playback validates a Diagram's time signature, sequence and tempo against
// its positions and returns them normalized: the default time signature
// when none is given, StrumNone for every step without a strum. The tempo
// is set exactly when the sequence has steps, since it is what they are
// played at.
func playback(timeSignature TimeSignature, sequence []SequenceStep, tempoBPM *int, positions []Position) (TimeSignature, []SequenceStep, error) {
	if timeSignature == (TimeSignature{}) {
		timeSignature = DefaultTimeSignature
	}
	if !timeSignature.valid() {
		return TimeSignature{}, nil, NewValidationError("time_signature", fmt.Sprintf("beats must be 1-%d and beat_value one of 1, 2, 4, 8, 16, 32", MaxTimeSignatureBeats))
	}
	steps, err := validSequence(sequence, positions)
	if err != nil {
		return TimeSignature{}, nil, err
	}
	switch {
	case len(steps) > 0 && tempoBPM == nil:
		return TimeSignature{}, nil, NewValidationError("tempo_bpm", "is required when the sequence has steps")
	case len(steps) == 0 && tempoBPM != nil:
		return TimeSignature{}, nil, NewValidationError("tempo_bpm", "must be absent when the sequence is empty")
	case tempoBPM != nil && (*tempoBPM < MinTempoBPM || *tempoBPM > MaxTempoBPM):
		return TimeSignature{}, nil, NewValidationError("tempo_bpm", fmt.Sprintf("must be between %d and %d", MinTempoBPM, MaxTempoBPM))
	}
	return timeSignature, steps, nil
}

// validSequence returns a copy of sequence with StrumNone filled in, or an
// error for the first step that names a position positions doesn't have,
// names one twice, or carries an invalid value or strum. No steps is nil.
func validSequence(sequence []SequenceStep, positions []Position) ([]SequenceStep, error) {
	if len(sequence) == 0 {
		return nil, nil
	}
	if len(sequence) > MaxSequenceSteps {
		return nil, NewValidationError("sequence", fmt.Sprintf("must have at most %d steps", MaxSequenceSteps))
	}
	known := make(map[string]struct{}, len(positions))
	for _, p := range positions {
		known[p.ID] = struct{}{}
	}
	out := make([]SequenceStep, len(sequence))
	for i, s := range sequence {
		if reason := stepProblem(s, known); reason != "" {
			return nil, NewValidationError("sequence", fmt.Sprintf("step %d %s", i, reason))
		}
		out[i] = SequenceStep{PositionIDs: slices.Clone(s.PositionIDs), Value: s.Value, Strum: s.Strum}
		if out[i].Strum == "" {
			out[i].Strum = StrumNone
		}
	}
	return out, nil
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
