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

func newChallengeService(nodes *fakeContentNodeRepository, challenges *fakeChallengeRepository, exercises *fakeExerciseRepository) *application.ChallengeService {
	return application.NewChallengeService(nodes, challenges, exercises, idSequence(), func() time.Time { return fixedCreatedAt })
}

func TestChallengeService_CreateChallenge(t *testing.T) {
	t.Run("a teacher creates a challenge with a subject tag and pass threshold", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		challenge, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "triad-shapes", 70, nil)

		require.NoError(t, err)
		assert.Equal(t, "node-1", challenge.ContentNodeID)
		assert.Equal(t, 70, challenge.PassThreshold)
	})

	t.Run("a teacher creates a challenge with a remediation target", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())
		target := "node-remediation"

		challenge, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "triad-shapes", 70, &target)

		require.NoError(t, err)
		require.NotNil(t, challenge.RemediationTargetContentNodeID)
		assert.Equal(t, target, *challenge.RemediationTargetContentNodeID)
	})

	t.Run("an admin creates a challenge", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), adminCaller(), "node-1", "chord-theory", 80, nil)

		require.NoError(t, err)
	})

	t.Run("creating a challenge without a subject tag is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "", 70, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "subject_tag")
	})

	t.Run("creating a challenge without a pass threshold is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "triad-shapes", 0, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "pass_threshold")
	})

	t.Run("creating a challenge for a non-existent content node returns not found", func(t *testing.T) {
		svc := newChallengeService(newFakeContentNodeRepository(), newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), teacherCaller(), "missing", "triad-shapes", 70, nil)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot create a challenge", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), studentCaller(), "node-1", "triad-shapes", 70, nil)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestChallengeService_GetChallenge(t *testing.T) {
	t.Run("any authenticated user retrieves a challenge by id", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenge := domain.Challenge{ID: "challenge-1", SubjectTag: "triad-shapes"}
		challenges.put(challenge)
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, newFakeExerciseRepository())

		got, err := svc.GetChallenge(context.Background(), "challenge-1")

		require.NoError(t, err)
		assert.Equal(t, challenge, got)
	})

	t.Run("retrieving a challenge that does not exist returns not found", func(t *testing.T) {
		svc := newChallengeService(newFakeContentNodeRepository(), newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.GetChallenge(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestChallengeService_CreateExercise(t *testing.T) {
	t.Run("a teacher creates a fretboard region exercise", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, newFakeExerciseRepository())

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(), "challenge-1",
			domain.ExerciseTypeFretboardRegion, "Identify the root position of a C major triad")

		require.NoError(t, err)
		assert.Equal(t, "challenge-1", exercise.ChallengeID)
	})

	t.Run("an admin creates an exercise", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), adminCaller(), "challenge-1",
			domain.ExerciseTypeFretboardRegion, "Tap the first inversion of a G major triad")

		require.NoError(t, err)
	})

	t.Run("creating an exercise without an exercise type is rejected", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(), "challenge-1", "", "prompt")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "exercise_type")
	})

	t.Run("creating an exercise without a prompt is rejected", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(), "challenge-1", domain.ExerciseTypeFretboardRegion, "")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "prompt")
	})

	t.Run("creating an exercise with an unrecognised type is rejected", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(), "challenge-1", domain.ExerciseType("multiple_choice"), "prompt")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "exercise_type")
	})

	t.Run("creating an exercise for a non-existent challenge returns not found", func(t *testing.T) {
		svc := newChallengeService(newFakeContentNodeRepository(), newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(), "missing", domain.ExerciseTypeFretboardRegion, "prompt")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot create an exercise", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), studentCaller(), "challenge-1", domain.ExerciseTypeFretboardRegion, "prompt")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestChallengeService_GetExercise(t *testing.T) {
	t.Run("retrieving an exercise that does not exist returns not found", func(t *testing.T) {
		svc := newChallengeService(newFakeContentNodeRepository(), newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.GetExercise(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
