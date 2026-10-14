package domain

import (
	"errors"
	"regexp"
	"strings"
)

// ErrInvalidPracticeItemKey is returned for a key that names no practice item of a
// known kind.
var ErrInvalidPracticeItemKey = errors.New("invalid practice item key")

// PracticeItemKind is the prefix of an item key. The set is open: a new
// kind adds its key scheme here and its grader to the registry.
type PracticeItemKind string

const (
	PracticeItemKindFretboardCell PracticeItemKind = "fretboard_cell"
	PracticeItemKindExercise      PracticeItemKind = "exercise"
	PracticeItemKindPlayAlong     PracticeItemKind = "play_along"
	PracticeItemKindChordChange   PracticeItemKind = "chord_change"
	PracticeItemKindDiagramShape  PracticeItemKind = "diagram_shape"

	PracticeItemKindDiatonicChord    PracticeItemKind = "diatonic_chord"
	PracticeItemKindChordFunction    PracticeItemKind = "chord_function"
	PracticeItemKindChartProgression PracticeItemKind = "chart_progression"
	PracticeItemKindHeardChord       PracticeItemKind = "heard_chord"
	PracticeItemKindHeardProgression PracticeItemKind = "heard_progression"
	PracticeItemKindChordVoicing     PracticeItemKind = "chord_voicing"
	PracticeItemKindRootFifth        PracticeItemKind = "root_fifth"
	PracticeItemKindRootChange       PracticeItemKind = "root_change"
	PracticeItemKindGuideTone        PracticeItemKind = "guide_tone"
	PracticeItemKindFretboardNote    PracticeItemKind = "fretboard_note"
)

const (
	uuidPattern = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`
	// musicKeyPattern is a major key, its root spelled as the chord catalog spells roots.
	musicKeyPattern = `(C|Db|D|Eb|E|F|F#|G|Ab|A|Bb|B)`
	// numeralPattern is a degree of a major key; viio stands for vii°.
	numeralPattern        = `(I|ii|iii|IV|V|vi|viio)`
	progressionKeyPattern = `[0-9A-Za-z][0-9A-Za-z-]*`
	// chordPattern is a root, then minor, diminished, dominant seventh or nothing (major).
	chordPattern = musicKeyPattern + `(m|dim|7)?`
)

// practiceItemKeyPattern matches every known kind's key scheme: the kind prefix,
// then its own segments. It must stay identical to the one ingestion validates with.
var practiceItemKeyPattern = regexp.MustCompile(`^(` +
	`fretboard_cell:` + uuidPattern + `:[1-9][0-9]*:(0|[1-9][0-9]*)` +
	`|exercise:` + uuidPattern +
	`|play_along:` + uuidPattern +
	`|chord_change:` + uuidPattern + `:` + uuidPattern +
	`|diagram_shape:` + uuidPattern +
	`|diatonic_chord:` + musicKeyPattern + `:` + numeralPattern +
	`|chord_function:` + musicKeyPattern + `:` + numeralPattern +
	`|chart_progression:` + progressionKeyPattern +
	`|heard_chord:` + numeralPattern +
	`|heard_progression:` + progressionKeyPattern +
	`|chord_voicing:` + uuidPattern +
	`|root_fifth:` + uuidPattern + `:` + chordPattern +
	`|root_change:` + uuidPattern + `:` + chordPattern + `:` + chordPattern +
	`|guide_tone:` + uuidPattern + `:` + progressionKeyPattern +
	`|fretboard_note:` + uuidPattern + `:(C|C#|D|D#|E|F|F#|G|G#|A|A#|B)` +
	`)$`)

// PracticeItemKey identifies the practice item that evidence is about.
type PracticeItemKey struct {
	Kind PracticeItemKind
	raw  string
	// parts are the key's segments after the kind prefix.
	parts []string
}

// ParsePracticeItemKey reads an item key. Ingestion already validated it; the worker
// checks again so a malformed message can never reach a grader.
func ParsePracticeItemKey(raw string) (PracticeItemKey, error) {
	if !practiceItemKeyPattern.MatchString(raw) {
		return PracticeItemKey{}, ErrInvalidPracticeItemKey
	}
	segments := strings.Split(raw, ":")
	return PracticeItemKey{Kind: PracticeItemKind(segments[0]), raw: raw, parts: segments[1:]}, nil
}

func (k PracticeItemKey) String() string { return k.raw }

// LayoutInstrumentID is the instrument whose fretboard the item is played or
// tapped on: a fretboard cell's, a bass root and fifth's or root change's, a guide
// tone's or a note found everywhere; empty for the other kinds.
func (k PracticeItemKey) LayoutInstrumentID() string {
	if !onALayout[k.Kind] {
		return ""
	}
	return k.parts[0]
}

// onALayout are the kinds whose key starts with the layout instrument they are
// played or tapped on.
var onALayout = map[PracticeItemKind]bool{
	PracticeItemKindFretboardCell: true, PracticeItemKindRootFifth: true, PracticeItemKindRootChange: true,
	PracticeItemKindGuideTone: true, PracticeItemKindFretboardNote: true,
}

// ProgressionKey is the progression of the progression catalog the item is about:
// a chart progression's, a heard progression's or a guide tone's; empty for the
// other kinds.
func (k PracticeItemKey) ProgressionKey() string {
	segment, ok := progressionSegment[k.Kind]
	if !ok {
		return ""
	}
	return k.parts[segment]
}

// progressionSegment is where a progression item's key names its progression.
var progressionSegment = map[PracticeItemKind]int{
	PracticeItemKindChartProgression: 0, PracticeItemKindHeardProgression: 0, PracticeItemKindGuideTone: 1,
}

// ExerciseID is the exercise an exercise item is about; empty for the other kinds.
func (k PracticeItemKey) ExerciseID() string {
	if k.Kind != PracticeItemKindExercise {
		return ""
	}
	return k.parts[0]
}

// DiagramIDs are the diagrams the item points at: one for a play-along or a
// diagram shape, the two chords of a chord change, none for the other kinds.
func (k PracticeItemKey) DiagramIDs() []string {
	switch k.Kind {
	case PracticeItemKindPlayAlong, PracticeItemKindChordChange, PracticeItemKindDiagramShape, PracticeItemKindChordVoicing:
		return k.parts
	case PracticeItemKindFretboardCell, PracticeItemKindExercise, PracticeItemKindDiatonicChord, PracticeItemKindChordFunction,
		PracticeItemKindChartProgression, PracticeItemKindHeardChord, PracticeItemKindHeardProgression,
		PracticeItemKindRootFifth, PracticeItemKindRootChange, PracticeItemKindGuideTone, PracticeItemKindFretboardNote:
		return nil
	}
	return nil
}
