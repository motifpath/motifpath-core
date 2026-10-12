//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// soundRefOf is a diagram_ref that plays diagramID's playbackID once.
func soundRefOf(diagramID, playbackID string) *domain.DiagramRef {
	return &domain.DiagramRef{DiagramID: diagramID, Playback: &domain.DiagramRefPlayback{PlaybackID: &playbackID, Direction: domain.DiagramPlaybackDirectionAsAuthored}}
}

func TestEntExerciseSoundUses(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	exercises := NewEntExerciseRepository(client)
	skill := seedSkill(t, ctx, client, "ear-training-"+uuid.NewString())
	diagramID, otherDiagramID := uuid.NewString(), uuid.NewString()

	recognition := instrumentExercise("Major or minor?", skill)
	recognition.ExerciseType = domain.ExerciseTypeAudioRecognition
	recognition.DiagramRef = soundRefOf(diagramID, "pb-strum")

	selection := instrumentExercise("Which one is minor?", skill)
	selection.ExerciseType = domain.ExerciseTypeAudioSelection
	selection.Options = []domain.Option{
		{ID: uuid.NewString(), DiagramRef: soundRefOf(diagramID, "pb-arp")},
		{ID: uuid.NewString(), IsCorrect: true, DiagramRef: soundRefOf(otherDiagramID, "pb-strum")},
		{ID: uuid.NewString(), AudioURL: strPtr("https://cdn.example.com/c.mp3")},
	}

	clipOnly := instrumentExercise("Interval by ear", skill)
	clipOnly.ExerciseType = domain.ExerciseTypeAudioRecognition
	clipOnly.AudioURL = strPtr("https://cdn.example.com/fifth.mp3")

	// An image_choice thumbnail of the same diagram is not a sound.
	thumbnail := instrumentExercise("Which box is A minor?", skill)
	thumbnail.ExerciseType = domain.ExerciseTypeImageChoice
	thumbnail.Options = []domain.Option{{ID: uuid.NewString(), IsCorrect: true, DiagramRef: soundRefOf(diagramID, "pb-strum")}}

	for _, e := range []domain.Exercise{recognition, selection, clipOnly, thumbnail} {
		require.NoError(t, exercises.Create(ctx, e))
	}

	t.Run("lists the stimulus and option sounds that play the diagram", func(t *testing.T) {
		got, err := exercises.ListSoundUses(ctx, diagramID)

		require.NoError(t, err)
		assert.ElementsMatch(t, []domain.ExerciseSoundUse{
			{ExerciseID: recognition.ID, Title: "Major or minor?", PlaybackID: "pb-strum"},
			{ExerciseID: selection.ID, Title: "Which one is minor?", PlaybackID: "pb-arp"},
		}, got)
	})

	t.Run("lists only the options that play the diagram asked for", func(t *testing.T) {
		got, err := exercises.ListSoundUses(ctx, otherDiagramID)

		require.NoError(t, err)
		assert.Equal(t, []domain.ExerciseSoundUse{{ExerciseID: selection.ID, Title: "Which one is minor?", PlaybackID: "pb-strum"}}, got)
	})

	t.Run("is empty for a diagram no exercise plays", func(t *testing.T) {
		got, err := exercises.ListSoundUses(ctx, uuid.NewString())

		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
