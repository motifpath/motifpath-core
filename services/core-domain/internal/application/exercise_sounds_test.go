package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// soundOf is a diagram_ref that plays diagramID's playbackID once.
func soundOf(diagramID, playbackID string) *domain.DiagramRef {
	return &domain.DiagramRef{DiagramID: diagramID, Playback: &domain.DiagramRefPlayback{PlaybackID: &playbackID, Direction: domain.DiagramPlaybackDirectionAsAuthored}}
}

// soundExerciseService is an exercise service whose diagram "a-minor" has
// the playbacks "pb-strum" and "pb-arp".
func soundExerciseService(t *testing.T) (*application.ExerciseService, *fakeExerciseRepository) {
	t.Helper()
	exercises, diagrams := newFakeExerciseRepository(), newFakeDiagramRepository()
	seedDiagram(t, diagrams, "a-minor", "guitar", nil)
	givePlaybacks(t, diagrams, "a-minor", "pb-strum", "pb-arp")
	return newExerciseServiceWithDiagrams(newFakeChallengeRepository(), exercises, newFakeContentNodeRepository(), diagrams), exercises
}

func majorMinorOptions() []domain.Option {
	major, minor := "Major", "Minor"
	return []domain.Option{{ID: "opt-major", Label: &major}, {ID: "opt-minor", IsCorrect: true, Label: &minor}}
}

func TestExerciseService_DiagramSounds(t *testing.T) {
	ctx := context.Background()
	createRecognition := func(svc *application.ExerciseService, audioURL *string, sound *domain.DiagramRef) (domain.Exercise, error) {
		return svc.CreateExercise(ctx, teacherCaller(), "Major or minor?", domain.NewPlainTextPrompt("Is this chord major or minor?"),
			domain.ExerciseTypeAudioRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, audioURL, sound, nil, majorMinorOptions(), nil, nil, []string{"en"}, nil)
	}
	createSelection := func(svc *application.ExerciseService, sounds ...*domain.DiagramRef) (domain.Exercise, error) {
		options := make([]domain.Option, len(sounds))
		for i, sound := range sounds {
			options[i] = domain.Option{ID: "opt-" + string(rune('1'+i)), IsCorrect: i == len(sounds)-1, DiagramRef: sound}
		}
		return svc.CreateExercise(ctx, teacherCaller(), "Which one is minor?", domain.NewPlainTextPrompt("Which of these is minor?"),
			domain.ExerciseTypeAudioSelection, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, options, nil, nil, []string{"en"}, nil)
	}
	requireField := func(t *testing.T, err error, field string) {
		t.Helper()
		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, field, valErr.Fields[0].Field)
	}

	tests := []struct {
		name      string
		create    func(svc *application.ExerciseService) (domain.Exercise, error)
		wantField string // "" = created
	}{
		{name: "an audio_recognition exercise plays one of its diagram's playbacks",
			create: func(svc *application.ExerciseService) (domain.Exercise, error) {
				return createRecognition(svc, nil, soundOf("a-minor", "pb-strum"))
			}},
		{name: "an audio_recognition sound's diagram must exist",
			create: func(svc *application.ExerciseService) (domain.Exercise, error) {
				return createRecognition(svc, nil, soundOf("no-such-diagram", "pb-strum"))
			}, wantField: "diagram_ref"},
		{name: "an audio_recognition sound must name one of its diagram's playbacks",
			create: func(svc *application.ExerciseService) (domain.Exercise, error) {
				return createRecognition(svc, nil, soundOf("a-minor", "pb-fingerstyle"))
			}, wantField: "diagram_ref"},
		{name: "audio_selection options play their diagrams' playbacks",
			create: func(svc *application.ExerciseService) (domain.Exercise, error) {
				return createSelection(svc, soundOf("a-minor", "pb-strum"), soundOf("a-minor", "pb-arp"))
			}},
		{name: "an audio_selection option's sound must name one of its diagram's playbacks",
			create: func(svc *application.ExerciseService) (domain.Exercise, error) {
				return createSelection(svc, soundOf("a-minor", "pb-strum"), soundOf("a-minor", "pb-fingerstyle"))
			}, wantField: "options"},
		{name: "an audio_selection option's sound's diagram must exist",
			create: func(svc *application.ExerciseService) (domain.Exercise, error) {
				return createSelection(svc, soundOf("a-minor", "pb-strum"), soundOf("no-such-diagram", "pb-strum"))
			}, wantField: "options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := soundExerciseService(t)
			exercise, err := tt.create(svc)
			if tt.wantField != "" {
				requireField(t, err, tt.wantField)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, exercise.ID)
		})
	}

	t.Run("an audio_recognition exercise keeps its diagram sound", func(t *testing.T) {
		svc, _ := soundExerciseService(t)
		exercise, err := createRecognition(svc, nil, soundOf("a-minor", "pb-arp"))
		require.NoError(t, err)
		require.NotNil(t, exercise.DiagramRef)
		assert.Equal(t, "a-minor", exercise.DiagramRef.DiagramID)
		assert.Equal(t, "pb-arp", *exercise.DiagramRef.Playback.PlaybackID)
		assert.Nil(t, exercise.AudioURL)
	})

	t.Run("an exercise's sound switches from an uploaded clip to a diagram", func(t *testing.T) {
		svc, _ := soundExerciseService(t)
		clip := "https://cdn.example.com/a-minor.mp3"
		created, err := createRecognition(svc, &clip, nil)
		require.NoError(t, err)

		updated, err := svc.UpdateExercise(ctx, teacherCaller(), created.ID, created.Title, created.Prompt, []string{"skill-1"}, []string{"concept-1"},
			nil, nil, soundOf("a-minor", "pb-strum"), nil, created.Options, nil, nil, []string{"en"}, nil)
		require.NoError(t, err)
		assert.Nil(t, updated.AudioURL)
		assert.Equal(t, "pb-strum", *updated.DiagramRef.Playback.PlaybackID)
	})
}
