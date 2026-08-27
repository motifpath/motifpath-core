package domain

import "time"

// ExerciseType identifies the kind of practice interaction. For MVP the
// only supported type is fretboard_region.
type ExerciseType string

const (
	ExerciseTypeFretboardRegion ExerciseType = "fretboard_region"
)

// Exercise is a pre-defined practice item within a challenge. For MVP all
// exercises are fretboard region interactions with a binary correct/
// incorrect outcome.
type Exercise struct {
	ID           string
	ChallengeID  string
	ExerciseType ExerciseType
	Prompt       string
	CreatedAt    time.Time
}

// NewExercise validates and constructs an Exercise. Whether ChallengeID
// refers to a challenge that actually exists is an application-layer
// concern.
func NewExercise(id, challengeID string, exerciseType ExerciseType, prompt string, createdAt time.Time) (Exercise, error) {
	var errs []FieldError

	switch exerciseType {
	case ExerciseTypeFretboardRegion:
	default:
		errs = append(errs, FieldError{Field: "exercise_type", Reason: "must be fretboard_region"})
	}
	if prompt == "" {
		errs = append(errs, FieldError{Field: "prompt", Reason: "must not be empty"})
	}

	if len(errs) > 0 {
		return Exercise{}, &ValidationError{Fields: errs}
	}

	return Exercise{
		ID:           id,
		ChallengeID:  challengeID,
		ExerciseType: exerciseType,
		Prompt:       prompt,
		CreatedAt:    createdAt,
	}, nil
}
