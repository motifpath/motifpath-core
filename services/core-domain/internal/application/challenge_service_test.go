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

		challenge, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "triad-shapes", 70, nil, false, false)

		require.NoError(t, err)
		assert.Equal(t, "node-1", challenge.ContentNodeID)
		assert.Equal(t, 70, challenge.PassThreshold)
	})

	t.Run("a teacher creates a challenge with an explicit time threshold", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		challenge, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "triad-shapes", 70, intPtr(120000), false, false)

		require.NoError(t, err)
		require.NotNil(t, challenge.TimeThresholdMS)
		assert.Equal(t, 120000, *challenge.TimeThresholdMS)
	})

	t.Run("an admin creates a challenge", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), adminCaller(), "node-1", "chord-theory", 80, nil, false, false)

		require.NoError(t, err)
	})

	t.Run("creating a challenge without a subject tag is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "", 70, nil, false, false)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "subject_tag")
	})

	t.Run("creating a challenge without a pass threshold is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "triad-shapes", 0, nil, false, false)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "pass_threshold")
	})

	t.Run("creating a challenge for a non-existent content node returns not found", func(t *testing.T) {
		svc := newChallengeService(newFakeContentNodeRepository(), newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), teacherCaller(), "missing", "triad-shapes", 70, nil, false, false)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot create a challenge", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateChallenge(context.Background(), studentCaller(), "node-1", "triad-shapes", 70, nil, false, false)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher creates a challenge with shuffled exercises and options", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		challenge, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "triad-shapes", 70, nil, true, true)

		require.NoError(t, err)
		assert.True(t, challenge.ShuffleExercises)
		assert.True(t, challenge.ShuffleOptions)
	})

	t.Run("a teacher creates a challenge without specifying shuffling", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		challenge, err := svc.CreateChallenge(context.Background(), teacherCaller(), "node-1", "triad-shapes", 70, nil, false, false)

		require.NoError(t, err)
		assert.False(t, challenge.ShuffleExercises)
		assert.False(t, challenge.ShuffleOptions)
	})
}

func TestChallengeService_ListChallengesForContentNode(t *testing.T) {
	t.Run("a student lists the challenges for a node that has one", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1"})
		svc := newChallengeService(nodes, challenges, newFakeExerciseRepository())

		got, err := svc.ListChallengesForContentNode(context.Background(), "node-1")

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "challenge-1", got[0].ID)
	})

	t.Run("a student lists the challenges for a node that has none", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("silent-node"))
		svc := newChallengeService(nodes, newFakeChallengeRepository(), newFakeExerciseRepository())

		got, err := svc.ListChallengesForContentNode(context.Background(), "silent-node")

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("listing challenges for a content node that does not exist returns not found", func(t *testing.T) {
		svc := newChallengeService(newFakeContentNodeRepository(), newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.ListChallengesForContentNode(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("each listed challenge resolves its own time threshold independently", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "overridden", ContentNodeID: "node-1", TimeThresholdMS: intPtr(90000)})
		challenges.put(domain.Challenge{ID: "derived", ContentNodeID: "node-1"})
		challenges.put(domain.Challenge{ID: "no-exercises", ContentNodeID: "node-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ChallengeIDs: []string{"derived"}, EstimatedDurationSeconds: intPtr(30)})
		exercises.put(domain.Exercise{ID: "ex-2", ChallengeIDs: []string{"derived"}, EstimatedDurationSeconds: intPtr(45)})
		svc := newChallengeService(nodes, challenges, exercises)

		got, err := svc.ListChallengesForContentNode(context.Background(), "node-1")

		require.NoError(t, err)
		byID := map[string]domain.Challenge{}
		for _, c := range got {
			byID[c.ID] = c
		}
		require.NotNil(t, byID["overridden"].TimeThresholdMS)
		assert.Equal(t, 90000, *byID["overridden"].TimeThresholdMS)
		require.NotNil(t, byID["derived"].TimeThresholdMS)
		assert.Equal(t, 75000, *byID["derived"].TimeThresholdMS)
		assert.Nil(t, byID["no-exercises"].TimeThresholdMS)
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

func TestChallengeService_TimeThresholdResolution(t *testing.T) {
	t.Run("a challenge's time threshold defaults to the sum of its linked exercises' estimated durations", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ChallengeIDs: []string{"challenge-1"}, EstimatedDurationSeconds: intPtr(30)})
		exercises.put(domain.Exercise{ID: "ex-2", ChallengeIDs: []string{"challenge-1"}, EstimatedDurationSeconds: intPtr(45)})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, exercises)

		got, err := svc.GetChallenge(context.Background(), "challenge-1")

		require.NoError(t, err)
		require.NotNil(t, got.TimeThresholdMS)
		assert.Equal(t, 75000, *got.TimeThresholdMS)
	})

	t.Run("a linked exercise with no estimate contributes zero to the computed threshold", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ChallengeIDs: []string{"challenge-1"}, EstimatedDurationSeconds: intPtr(30)})
		exercises.put(domain.Exercise{ID: "ex-2", ChallengeIDs: []string{"challenge-1"}})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, exercises)

		got, err := svc.GetChallenge(context.Background(), "challenge-1")

		require.NoError(t, err)
		require.NotNil(t, got.TimeThresholdMS)
		assert.Equal(t, 30000, *got.TimeThresholdMS)
	})

	t.Run("the computed time threshold is absent when the challenge has no linked exercises", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, newFakeExerciseRepository())

		got, err := svc.GetChallenge(context.Background(), "challenge-1")

		require.NoError(t, err)
		assert.Nil(t, got.TimeThresholdMS)
	})

	t.Run("an explicit override takes precedence over the computed value", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", TimeThresholdMS: intPtr(90000)})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ChallengeIDs: []string{"challenge-1"}, EstimatedDurationSeconds: intPtr(30)})
		svc := newChallengeService(newFakeContentNodeRepository(), challenges, exercises)

		got, err := svc.GetChallenge(context.Background(), "challenge-1")

		require.NoError(t, err)
		require.NotNil(t, got.TimeThresholdMS)
		assert.Equal(t, 90000, *got.TimeThresholdMS)
	})
}

func TestChallengeService_UpdateChallenge(t *testing.T) {
	t.Run("a teacher updates a challenge's subject tag and pass threshold", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1", SubjectTag: "triad-shapes", PassThreshold: 70})
		svc := newChallengeService(nodes, challenges, newFakeExerciseRepository())

		got, err := svc.UpdateChallenge(context.Background(), teacherCaller(), "challenge-1", "triad-shapes-revised", 85, nil, false, false)

		require.NoError(t, err)
		assert.Equal(t, "triad-shapes-revised", got.SubjectTag)
		assert.Equal(t, 85, got.PassThreshold)
	})

	t.Run("a teacher sets an explicit time threshold on an existing challenge", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1", SubjectTag: "triad-shapes", PassThreshold: 70})
		svc := newChallengeService(nodes, challenges, newFakeExerciseRepository())

		got, err := svc.UpdateChallenge(context.Background(), teacherCaller(), "challenge-1", "triad-shapes", 70, intPtr(90000), false, false)

		require.NoError(t, err)
		require.NotNil(t, got.TimeThresholdMS)
		assert.Equal(t, 90000, *got.TimeThresholdMS)
	})

	t.Run("a teacher clears a challenge's explicit time threshold, falling back to the computed value", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1", SubjectTag: "triad-shapes", PassThreshold: 70, TimeThresholdMS: intPtr(90000)})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ChallengeIDs: []string{"challenge-1"}, EstimatedDurationSeconds: intPtr(30)})
		svc := newChallengeService(nodes, challenges, exercises)

		got, err := svc.UpdateChallenge(context.Background(), teacherCaller(), "challenge-1", "triad-shapes", 70, nil, false, false)

		require.NoError(t, err)
		require.NotNil(t, got.TimeThresholdMS)
		assert.Equal(t, 30000, *got.TimeThresholdMS)
	})

	t.Run("updating does not change linked exercises", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1", SubjectTag: "triad-shapes", PassThreshold: 70})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ChallengeIDs: []string{"challenge-1"}})
		svc := newChallengeService(nodes, challenges, exercises)

		_, err := svc.UpdateChallenge(context.Background(), teacherCaller(), "challenge-1", "triad-shapes-revised", 70, nil, false, false)
		require.NoError(t, err)

		linked, err := exercises.ListByChallengeID(context.Background(), "challenge-1")
		require.NoError(t, err)
		require.Len(t, linked, 1)
		assert.Equal(t, "ex-1", linked[0].ID)
	})

	t.Run("updating without a subject tag is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1", SubjectTag: "triad-shapes", PassThreshold: 70})
		svc := newChallengeService(nodes, challenges, newFakeExerciseRepository())

		_, err := svc.UpdateChallenge(context.Background(), teacherCaller(), "challenge-1", "", 70, nil, false, false)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "subject_tag")
	})

	t.Run("updating with a pass threshold above 100 is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1", SubjectTag: "triad-shapes", PassThreshold: 70})
		svc := newChallengeService(nodes, challenges, newFakeExerciseRepository())

		_, err := svc.UpdateChallenge(context.Background(), teacherCaller(), "challenge-1", "triad-shapes", 101, nil, false, false)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "pass_threshold")
	})

	t.Run("a student cannot update a challenge", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1", SubjectTag: "triad-shapes", PassThreshold: 70})
		svc := newChallengeService(nodes, challenges, newFakeExerciseRepository())

		_, err := svc.UpdateChallenge(context.Background(), studentCaller(), "challenge-1", "hijacked-tag", 70, nil, false, false)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher cannot update a challenge on another teacher's content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1")) // owned by teacher-1
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1", SubjectTag: "triad-shapes", PassThreshold: 70})
		svc := newChallengeService(nodes, challenges, newFakeExerciseRepository())

		_, err := svc.UpdateChallenge(context.Background(), otherTeacherCaller(), "challenge-1", "hijacked-tag", 70, nil, false, false)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an admin can update a challenge on any teacher's content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1")) // owned by teacher-1
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1", ContentNodeID: "node-1", SubjectTag: "triad-shapes", PassThreshold: 70})
		svc := newChallengeService(nodes, challenges, newFakeExerciseRepository())

		got, err := svc.UpdateChallenge(context.Background(), adminCaller(), "challenge-1", "revised-by-admin", 70, nil, false, false)

		require.NoError(t, err)
		assert.Equal(t, "revised-by-admin", got.SubjectTag)
	})

	t.Run("updating a challenge that does not exist returns not found", func(t *testing.T) {
		svc := newChallengeService(newFakeContentNodeRepository(), newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.UpdateChallenge(context.Background(), teacherCaller(), "missing", "triad-shapes", 70, nil, false, false)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

// Exercise creation/retrieval/linking now lives on ExerciseService — see
// exercise_service_test.go.
