package domain

import "slices"

// DiagramReference is what the practice graders may know about a Diagram
// (ADR-047): that it exists, the instruments it suits, and its playback
// tempo. Core keeps it in the read-only practice reference snapshot; it
// carries nothing shown to users.
type DiagramReference struct {
	ID string
	// InstrumentIDs lists every instrument the diagram suits, layout first.
	InstrumentIDs []string
	// TempoBPM is the playback tempo; nil when the diagram has no sequence.
	TempoBPM *int
}

// NewDiagramReference returns d's reference.
func NewDiagramReference(d Diagram) DiagramReference {
	return DiagramReference{ID: d.ID, InstrumentIDs: slices.Clone(d.InstrumentIDs), TempoBPM: d.TempoBPM}
}

// ExerciseReference is what the exercise_option grader may know about an
// Exercise (ADR-047): its type, which picks its fluent time, every option
// and the correct ones, and the instruments it's for (empty for every
// instrument).
type ExerciseReference struct {
	ID               string
	ExerciseType     ExerciseType
	OptionIDs        []string
	CorrectOptionIDs []string
	InstrumentIDs    []string
}

// NewExerciseReference returns e's reference.
func NewExerciseReference(e Exercise) ExerciseReference {
	ref := ExerciseReference{
		ID:               e.ID,
		ExerciseType:     e.ExerciseType,
		OptionIDs:        make([]string, 0, len(e.Options)),
		CorrectOptionIDs: []string{},
		InstrumentIDs:    append([]string{}, e.InstrumentIDs...),
	}
	for _, o := range e.Options {
		ref.OptionIDs = append(ref.OptionIDs, o.ID)
		if o.IsCorrect {
			ref.CorrectOptionIDs = append(ref.CorrectOptionIDs, o.ID)
		}
	}
	return ref
}
