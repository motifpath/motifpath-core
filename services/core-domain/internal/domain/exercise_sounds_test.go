package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// soundRef is a diagram_ref that plays its diagram's playbackID once.
func soundRef(playbackID string) *domain.DiagramRef {
	return &domain.DiagramRef{DiagramID: "diagram-1", Playback: &domain.DiagramRefPlayback{PlaybackID: &playbackID, Direction: domain.DiagramPlaybackDirectionAsAuthored}}
}

func TestNewExercise_AudioRecognitionSound(t *testing.T) {
	audioURL := "https://cdn.example.com/a.mp3"
	looping := soundRef("pb-1")
	looping.Playback.Loop = true
	noPlayback := &domain.DiagramRef{DiagramID: "diagram-1"}
	defaultPlayback := &domain.DiagramRef{DiagramID: "diagram-1", Playback: &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored}}

	tests := []struct {
		name       string
		audioURL   *string
		diagramRef *domain.DiagramRef
		wantField  string // "" = valid
	}{
		{name: "an uploaded clip alone is valid", audioURL: &audioURL},
		{name: "a diagram playback alone is valid", diagramRef: soundRef("pb-1")},
		{name: "both an uploaded clip and a diagram is invalid", audioURL: &audioURL, diagramRef: soundRef("pb-1"), wantField: "diagram_ref"},
		{name: "no sound at all is invalid", wantField: "audio_url"},
		{name: "a diagram with no playback is invalid", diagramRef: noPlayback, wantField: "diagram_ref"},
		{name: "a diagram playing its default playback is invalid", diagramRef: defaultPlayback, wantField: "diagram_ref"},
		{name: "a looping diagram sound is invalid", diagramRef: looping, wantField: "diagram_ref"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label := "Minor"
			options := []domain.Option{{ID: "opt-1", IsCorrect: true, Label: &label}}
			exercise, err := domain.NewExercise("ex-1", "Title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeAudioRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, tt.audioURL, tt.diagramRef, nil, options, nil, nil, []string{"en"}, time.Now())
			if tt.wantField == "" {
				require.NoError(t, err)
				assert.Equal(t, tt.diagramRef, exercise.DiagramRef)
				assert.Equal(t, tt.audioURL, exercise.AudioURL)
				return
			}
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}
}

func TestNewExercise_AudioSelectionOptionSound(t *testing.T) {
	audioURL := "https://cdn.example.com/a.mp3"
	looping := soundRef("pb-1")
	looping.Playback.Loop = true

	tests := []struct {
		name      string
		option    domain.Option
		wantValid bool
	}{
		{name: "an uploaded clip alone is valid", option: domain.Option{ID: "opt-1", IsCorrect: true, AudioURL: &audioURL}, wantValid: true},
		{name: "a diagram playback alone is valid", option: domain.Option{ID: "opt-1", IsCorrect: true, DiagramRef: soundRef("pb-1")}, wantValid: true},
		{name: "both is invalid", option: domain.Option{ID: "opt-1", IsCorrect: true, AudioURL: &audioURL, DiagramRef: soundRef("pb-1")}},
		{name: "neither is invalid", option: domain.Option{ID: "opt-1", IsCorrect: true}},
		{name: "a diagram with no playback is invalid", option: domain.Option{ID: "opt-1", IsCorrect: true, DiagramRef: &domain.DiagramRef{DiagramID: "diagram-1"}}},
		{name: "a looping diagram sound is invalid", option: domain.Option{ID: "opt-1", IsCorrect: true, DiagramRef: looping}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewExercise("ex-1", "Title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeAudioSelection, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, []domain.Option{tt.option}, nil, nil, []string{"en"}, time.Now())
			if tt.wantValid {
				require.NoError(t, err)
				return
			}
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "options", valErr.Fields[0].Field)
		})
	}
}

func TestNewExercise_DiagramRefStaysOffOtherTypes(t *testing.T) {
	for _, exerciseType := range []domain.ExerciseType{domain.ExerciseTypeTextResponse, domain.ExerciseTypeImageChoice, domain.ExerciseTypeAudioSelection} {
		t.Run(string(exerciseType), func(t *testing.T) {
			audioURL := "https://cdn.example.com/a.mp3"
			label := "A"
			option := domain.Option{ID: "opt-1", IsCorrect: true, Label: &label, AudioURL: &audioURL, ImageURL: &audioURL}
			_, err := domain.NewExercise("ex-1", "Title", domain.NewPlainTextPrompt("prompt"), exerciseType, []string{"skill-1"}, []string{"concept-1"}, nil, nil, soundRef("pb-1"), nil, []domain.Option{option}, nil, nil, []string{"en"}, time.Now())
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Contains(t, fieldsOf(valErr), "diagram_ref")
		})
	}
}

func fieldsOf(err *domain.ValidationError) []string {
	fields := make([]string, len(err.Fields))
	for i, f := range err.Fields {
		fields[i] = f.Field
	}
	return fields
}
