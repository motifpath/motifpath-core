package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// A playback an exercise plays as its sound can't be removed from its
// diagram: dropping it would silently change the exercise's answer.
func TestDiagramService_PlaybackInUseAsSound(t *testing.T) {
	ctx := context.Background()
	quarter := domain.NoteValue{Num: 1, Den: 4}
	playback := func(id, name string, tempo int) domain.DiagramPlayback {
		return domain.DiagramPlayback{ID: id, Names: names(name), TempoBPM: tempo, Steps: []domain.SequenceStep{{PositionIDs: []string{"p1"}, Value: quarter}}}
	}
	// setup creates a diagram with "Strum" and "Arpeggio", and an exercise
	// of exerciseType titled title whose sound plays playbackName.
	setup := func(t *testing.T, exerciseType domain.ExerciseType, title, playbackName string) (diagramFixture, domain.Diagram) {
		t.Helper()
		f := newDiagramFixture()
		p := frettedPos(5, 0)
		p.ID = "p1"
		opts := domain.DiagramOptions{Playbacks: []domain.DiagramPlayback{playback("", "Strum", 90), playback("", "Arpeggio", 70)}}
		d, err := f.svc.CreateDiagram(ctx, teacherCaller(), "guitar", names("A minor"), []domain.Position{p}, []string{"skill-1"}, []string{"concept-1"}, opts)
		require.NoError(t, err)
		sound := soundOf(d.ID, playbackIDNamed(t, d, playbackName))
		exercise := domain.Exercise{ID: "ex-" + playbackName, Title: title, ExerciseType: exerciseType}
		if exerciseType == domain.ExerciseTypeAudioRecognition {
			exercise.DiagramRef = sound
		} else {
			exercise.Options = []domain.Option{{ID: "opt-1", IsCorrect: true, DiagramRef: sound}}
		}
		require.NoError(t, f.exercises.Create(ctx, exercise))
		return f, d
	}
	keeping := func(d domain.Diagram, names ...string) []domain.DiagramPlayback {
		var kept []domain.DiagramPlayback
		for _, p := range d.Playbacks {
			for _, name := range names {
				if p.Names["en"] == name {
					kept = append(kept, p)
				}
			}
		}
		return kept
	}

	t.Run("removing the playback an audio_recognition exercise plays is refused", func(t *testing.T) {
		f, d := setup(t, domain.ExerciseTypeAudioRecognition, "Major or minor?", "Strum")

		_, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Playbacks: keeping(d, "Arpeggio")})

		require.ErrorIs(t, err, domain.ErrConflict)
		assert.Contains(t, err.Error(), `"Strum"`)
		assert.Contains(t, err.Error(), `"Major or minor?"`)
		stored, err := f.diagrams.GetByID(ctx, d.ID)
		require.NoError(t, err)
		assert.Len(t, stored.Playbacks, 2)
	})

	t.Run("removing the playback an audio_selection option plays is refused", func(t *testing.T) {
		f, d := setup(t, domain.ExerciseTypeAudioSelection, "Which one is minor?", "Arpeggio")

		_, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Playbacks: keeping(d, "Strum")})

		require.ErrorIs(t, err, domain.ErrConflict)
		assert.Contains(t, err.Error(), `"Arpeggio"`)
		assert.Contains(t, err.Error(), `"Which one is minor?"`)
	})

	t.Run("removing every playback is refused while one is in use", func(t *testing.T) {
		f, d := setup(t, domain.ExerciseTypeAudioRecognition, "Major or minor?", "Strum")

		_, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Playbacks: []domain.DiagramPlayback{}})

		require.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("a playback in use can still be changed", func(t *testing.T) {
		f, d := setup(t, domain.ExerciseTypeAudioRecognition, "Major or minor?", "Strum")
		playbacks := keeping(d, "Strum", "Arpeggio")
		playbacks[0].TempoBPM = 80

		updated, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Playbacks: playbacks})

		require.NoError(t, err)
		assert.Equal(t, 80, updated.Playbacks[0].TempoBPM)
	})

	t.Run("a playback no exercise plays can be removed", func(t *testing.T) {
		f, d := setup(t, domain.ExerciseTypeAudioRecognition, "Major or minor?", "Strum")

		updated, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Playbacks: keeping(d, "Strum")})

		require.NoError(t, err)
		require.Len(t, updated.Playbacks, 1)
		assert.Equal(t, "Strum", updated.Playbacks[0].Names["en"])
	})

	t.Run("an update that leaves the playbacks alone is never refused", func(t *testing.T) {
		f, d := setup(t, domain.ExerciseTypeAudioRecognition, "Major or minor?", "Strum")

		_, err := f.svc.UpdateDiagram(ctx, teacherCaller(), d.ID, application.DiagramUpdate{Names: names("A minor chord")})

		require.NoError(t, err)
	})
}

// playbackIDNamed is the id of d's playback named name in English.
func playbackIDNamed(t *testing.T, d domain.Diagram, name string) string {
	t.Helper()
	for _, p := range d.Playbacks {
		if p.Names["en"] == name {
			return p.ID
		}
	}
	t.Fatalf("diagram %s has no playback %q", d.ID, name)
	return ""
}
