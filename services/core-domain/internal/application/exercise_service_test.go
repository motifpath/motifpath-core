package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func newExerciseService(challenges *fakeChallengeRepository, exercises *fakeExerciseRepository) *application.ExerciseService {
	return application.NewExerciseService(challenges, exercises, idSequence(), func() time.Time { return fixedCreatedAt })
}

func imageRecognitionOptions() []domain.Option {
	return []domain.Option{
		{ID: "opt-1", IsCorrect: true, Region: &domain.OptionRegion{X: 0.2, Y: 0.3, Width: 0.1, Height: 0.1, Shape: domain.OptionRegionShapeRectangle}},
		{ID: "opt-2", IsCorrect: false, Region: &domain.OptionRegion{X: 0.5, Y: 0.3, Width: 0.1, Height: 0.1, Shape: domain.OptionRegionShapeRectangle}},
	}
}

func textResponseOptions() []domain.Option {
	label1, label2 := "A major", "A minor"
	return []domain.Option{
		{ID: "opt-1", IsCorrect: true, Label: &label1},
		{ID: "opt-2", IsCorrect: false, Label: &label2},
	}
}

func TestExerciseService_CreateExercise(t *testing.T) {
	t.Run("a teacher creates a standalone image_recognition exercise", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		imageURL := "https://cdn.example.com/fretboard/c-major-triad.png"

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Root position of a C major triad", "Identify the root position of a C major triad",
			domain.ExerciseTypeImageRecognition, nil, &imageURL, nil, imageRecognitionOptions())

		require.NoError(t, err)
		assert.Equal(t, "Root position of a C major triad", exercise.Title)
		assert.Empty(t, exercise.ChallengeIDs)
	})

	t.Run("an admin creates an exercise", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), adminCaller(),
			"Name the interval", "Name the interval between the open low E and the 5th fret",
			domain.ExerciseTypeTextResponse, nil, nil, nil, textResponseOptions())

		require.NoError(t, err)
	})

	t.Run("a teacher creates an exercise with skill tags", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		imageURL := "https://cdn.example.com/fretboard/descending-run.png"

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Alternate picking — descending run", "Play the descending run cleanly",
			domain.ExerciseTypeImageRecognition, []string{"alternate_picking", "technique"}, &imageURL, nil, imageRecognitionOptions())

		require.NoError(t, err)
		assert.Equal(t, []string{"alternate_picking", "technique"}, exercise.SkillTags)
	})

	t.Run("creating an exercise without a title is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"", "prompt", domain.ExerciseTypeTextResponse, nil, nil, nil, textResponseOptions())

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "title")
	})

	t.Run("creating an exercise without a prompt is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", "", domain.ExerciseTypeTextResponse, nil, nil, nil, textResponseOptions())

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "prompt")
	})

	t.Run("creating an exercise without an exercise type is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", "prompt", "", nil, nil, nil, textResponseOptions())

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "exercise_type")
	})

	t.Run("creating an exercise with an unrecognised type is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", "prompt", domain.ExerciseType("multiple_choice"), nil, nil, nil, textResponseOptions())

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "exercise_type")
	})

	t.Run("creating an exercise with zero correct options is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		label := "A major"

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", "prompt", domain.ExerciseTypeTextResponse, nil, nil, nil,
			[]domain.Option{{ID: "opt-1", IsCorrect: false, Label: &label}})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "options")
	})

	t.Run("creating an exercise with an empty-string skill tag is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", "prompt", domain.ExerciseTypeTextResponse, []string{"technique", ""}, nil, nil, textResponseOptions())

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "skill_tags")
	})

	t.Run("a student cannot create an exercise", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), studentCaller(),
			"title", "prompt", domain.ExerciseTypeTextResponse, nil, nil, nil, textResponseOptions())

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_GetExercise(t *testing.T) {
	t.Run("retrieving an exercise that does not exist returns not found", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.GetExercise(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestExerciseService_LinkExerciseToChallenge(t *testing.T) {
	t.Run("a teacher links an existing exercise into a challenge", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(challenges, exercises)

		exercise, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")

		require.NoError(t, err)
		assert.Equal(t, []string{"challenge-1"}, exercise.ChallengeIDs)
	})

	t.Run("the same exercise is linked into a second challenge without duplication", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		challenges.put(domain.Challenge{ID: "challenge-2"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(challenges, exercises)
		_, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")
		require.NoError(t, err)

		exercise, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-2", "exercise-1")

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"challenge-1", "challenge-2"}, exercise.ChallengeIDs)
		assert.Equal(t, 1, exercises.count())
	})

	t.Run("linking a non-existent exercise to a challenge returns not found", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newExerciseService(challenges, newFakeExerciseRepository())

		_, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("linking an exercise to a non-existent challenge returns not found", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		_, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "missing", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("linking an exercise that is already linked to the challenge is rejected", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{"challenge-1"}})
		svc := newExerciseService(challenges, exercises)

		_, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("a student cannot link an exercise to a challenge", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(challenges, exercises)

		_, err := svc.LinkExerciseToChallenge(context.Background(), studentCaller(), "challenge-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_UnlinkExerciseFromChallenge(t *testing.T) {
	t.Run("a teacher unlinks an exercise from a challenge", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{"challenge-1"}})
		svc := newExerciseService(challenges, exercises)

		err := svc.UnlinkExerciseFromChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")
		require.NoError(t, err)

		exercise, err := svc.GetExercise(context.Background(), "exercise-1")
		require.NoError(t, err)
		assert.Empty(t, exercise.ChallengeIDs)
	})

	t.Run("unlinking an exercise that is not linked to the challenge returns not found", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(challenges, exercises)

		err := svc.UnlinkExerciseFromChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot unlink an exercise from a challenge", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{"challenge-1"}})
		svc := newExerciseService(challenges, exercises)

		err := svc.UnlinkExerciseFromChallenge(context.Background(), studentCaller(), "challenge-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}
