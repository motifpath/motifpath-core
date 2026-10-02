package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestExerciseService_Instruments(t *testing.T) {
	ctx := context.Background()
	create := func(t *testing.T, instrumentIDs *[]string) (domain.Exercise, error) {
		t.Helper()
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		return svc.CreateExercise(ctx, teacherCaller(), "Root notes", domain.NewPlainTextPrompt("Name the root"), domain.ExerciseTypeTextResponse,
			[]string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, instrumentIDs)
	}

	t.Run("an exercise created without instruments is for every instrument", func(t *testing.T) {
		got, err := create(t, nil)

		require.NoError(t, err)
		assert.Empty(t, got.InstrumentIDs)
	})

	t.Run("an exercise created for an instrument keeps it", func(t *testing.T) {
		got, err := create(t, &[]string{"guitar"})

		require.NoError(t, err)
		assert.Equal(t, []string{"guitar"}, got.InstrumentIDs)
	})

	for _, tt := range []struct {
		name string
		ids  []string
	}{
		{"an instrument that does not exist", []string{"banjo"}},
		{"a repeated instrument", []string{"guitar", "guitar"}},
	} {
		t.Run(tt.name+" is rejected", func(t *testing.T) {
			_, err := create(t, &tt.ids)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "instrument_ids", valErr.Fields[0].Field)
		})
	}

	update := func(t *testing.T, instrumentIDs *[]string) domain.Exercise {
		t.Helper()
		exercises := newFakeExerciseRepository()
		svc := newExerciseService(newFakeChallengeRepository(), exercises)
		ex, err := svc.CreateExercise(ctx, teacherCaller(), "Root notes", domain.NewPlainTextPrompt("Name the root"), domain.ExerciseTypeTextResponse,
			[]string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, &[]string{"guitar"})
		require.NoError(t, err)

		got, err := svc.UpdateExercise(ctx, teacherCaller(), ex.ID, "Roots", domain.NewPlainTextPrompt("Name the root"),
			[]string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, instrumentIDs)
		require.NoError(t, err)
		stored, err := exercises.GetByID(ctx, ex.ID)
		require.NoError(t, err)
		assert.Equal(t, got.InstrumentIDs, stored.InstrumentIDs)
		return got
	}

	t.Run("updating without instruments keeps them", func(t *testing.T) {
		assert.Equal(t, []string{"guitar"}, update(t, nil).InstrumentIDs)
	})

	t.Run("updating with instruments replaces them", func(t *testing.T) {
		assert.Equal(t, []string{"piano"}, update(t, &[]string{"piano"}).InstrumentIDs)
	})

	t.Run("updating with no instruments makes the exercise for every instrument", func(t *testing.T) {
		assert.Empty(t, update(t, &[]string{}).InstrumentIDs)
	})
}
