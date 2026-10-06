package domain

import "slices"

// DiagramReference is what the practice graders may know about a Diagram
// that it exists, the instruments it suits, and its playback
// tempo. Core keeps it in the read-only practice reference snapshot; it
// carries nothing shown to users.
type DiagramReference struct {
	ID string
	// InstrumentIDs lists every instrument the diagram suits, layout first.
	InstrumentIDs []string
	// TempoBPM is the default playback's tempo; nil when the diagram has no
	// playbacks.
	TempoBPM *int
}

// NewDiagramReference returns d's reference.
func NewDiagramReference(d Diagram) DiagramReference {
	ref := DiagramReference{ID: d.ID, InstrumentIDs: slices.Clone(d.InstrumentIDs)}
	if playback, ok := d.DefaultPlayback(); ok {
		tempo := playback.TempoBPM
		ref.TempoBPM = &tempo
	}
	return ref
}

// ExerciseReference is what the exercise_option grader may know about an
// Exercise: its type, which picks its fluent time, every option and the
// correct ones, and the instruments it's for (empty for every instrument).
// Options keeps each option as it is shown, so an answer's evidence can keep
// what the student saw even after the exercise is edited.
type ExerciseReference struct {
	ID               string
	ExerciseType     ExerciseType
	OptionIDs        []string
	CorrectOptionIDs []string
	InstrumentIDs    []string
	Options          []Option
}

// NewExerciseReference returns e's reference.
func NewExerciseReference(e Exercise) ExerciseReference {
	ref := ExerciseReference{
		ID:               e.ID,
		ExerciseType:     e.ExerciseType,
		OptionIDs:        make([]string, 0, len(e.Options)),
		CorrectOptionIDs: []string{},
		InstrumentIDs:    append([]string{}, e.InstrumentIDs...),
		Options:          slices.Clone(e.Options),
	}
	for _, o := range e.Options {
		ref.OptionIDs = append(ref.OptionIDs, o.ID)
		if o.IsCorrect {
			ref.CorrectOptionIDs = append(ref.CorrectOptionIDs, o.ID)
		}
	}
	return ref
}

// InstrumentReference is what the fretboard grader may know about an
// Instrument: its family, and the strings and tuning that give each fretboard
// cell its pitch. Strings and tuning never change after creation.
type InstrumentReference struct {
	ID          string
	Family      InstrumentFamily
	StringCount *int
	// Tuning is each string's open pitch, lowest string first.
	Tuning []string
}

// NewInstrumentReference returns i's reference.
func NewInstrumentReference(i Instrument) InstrumentReference {
	return InstrumentReference{ID: i.ID, Family: i.Family, StringCount: i.StringCount, Tuning: slices.Clone(i.Tuning)}
}
