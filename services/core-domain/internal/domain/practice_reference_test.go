package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestNewDiagramReference(t *testing.T) {
	t.Run("carries the id, every instrument and the playback tempo", func(t *testing.T) {
		tempo := 90
		d := domain.Diagram{ID: "d-1", InstrumentID: "guitar", InstrumentIDs: []string{"guitar", "electric"}, TempoBPM: &tempo, Names: domain.LocalizedText{"en": "Lick"}}

		got := domain.NewDiagramReference(d)

		assert.Equal(t, domain.DiagramReference{ID: "d-1", InstrumentIDs: []string{"guitar", "electric"}, TempoBPM: &tempo}, got)
	})

	t.Run("a diagram without playback has no tempo", func(t *testing.T) {
		d := domain.Diagram{ID: "d-2", InstrumentID: "guitar", InstrumentIDs: []string{"guitar"}}

		got := domain.NewDiagramReference(d)

		assert.Nil(t, got.TempoBPM)
	})

	t.Run("the instrument list is a copy", func(t *testing.T) {
		d := domain.Diagram{ID: "d-3", InstrumentIDs: []string{"guitar"}}

		got := domain.NewDiagramReference(d)
		d.InstrumentIDs[0] = "piano"

		assert.Equal(t, []string{"guitar"}, got.InstrumentIDs)
	})
}

func TestNewExerciseReference(t *testing.T) {
	label := "x"
	options := []domain.Option{
		{ID: "o-1", IsCorrect: false, Label: &label},
		{ID: "o-2", IsCorrect: true, Label: &label},
		{ID: "o-3", IsCorrect: true, Label: &label},
	}

	t.Run("carries the type, every option, the correct ones and the instruments", func(t *testing.T) {
		e := domain.Exercise{ID: "e-1", Title: "Triad", ExerciseType: domain.ExerciseTypeTextResponse, Options: options, InstrumentIDs: []string{"guitar"}}

		got := domain.NewExerciseReference(e)

		assert.Equal(t, domain.ExerciseReference{
			ID:               "e-1",
			ExerciseType:     domain.ExerciseTypeTextResponse,
			OptionIDs:        []string{"o-1", "o-2", "o-3"},
			CorrectOptionIDs: []string{"o-2", "o-3"},
			InstrumentIDs:    []string{"guitar"},
		}, got)
	})

	t.Run("an exercise for every instrument has an empty instrument list, never nil", func(t *testing.T) {
		got := domain.NewExerciseReference(domain.Exercise{ID: "e-2", ExerciseType: domain.ExerciseTypeAudioSelection, Options: options})

		assert.NotNil(t, got.InstrumentIDs)
		assert.Empty(t, got.InstrumentIDs)
	})

	t.Run("the instrument list is a copy", func(t *testing.T) {
		e := domain.Exercise{ID: "e-3", ExerciseType: domain.ExerciseTypeTextResponse, Options: options, InstrumentIDs: []string{"guitar"}}

		got := domain.NewExerciseReference(e)
		e.InstrumentIDs[0] = "piano"

		assert.Equal(t, []string{"guitar"}, got.InstrumentIDs)
	})
}
