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
)

const uuidPattern = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

// practiceItemKeyPattern matches every known kind's key scheme: the kind prefix,
// then its own segments. It must stay identical to the one ingestion validates with.
var practiceItemKeyPattern = regexp.MustCompile(`^(` +
	`fretboard_cell:` + uuidPattern + `:[1-9][0-9]*:(0|[1-9][0-9]*)` +
	`|exercise:` + uuidPattern +
	`|play_along:` + uuidPattern +
	`|chord_change:` + uuidPattern + `:` + uuidPattern +
	`|diagram_shape:` + uuidPattern +
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

// LayoutInstrumentID is the instrument whose fretboard a fretboard cell is on;
// empty for the other kinds.
func (k PracticeItemKey) LayoutInstrumentID() string {
	if k.Kind != PracticeItemKindFretboardCell {
		return ""
	}
	return k.parts[0]
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
	case PracticeItemKindPlayAlong, PracticeItemKindChordChange, PracticeItemKindDiagramShape:
		return k.parts
	case PracticeItemKindFretboardCell, PracticeItemKindExercise:
		return nil
	}
	return nil
}
