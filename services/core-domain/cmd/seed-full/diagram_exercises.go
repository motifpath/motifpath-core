package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// seedDiagramExercises creates the two exercises a student answers on a
// prebuilt diagram, and links both into challengeID:
//   - image_recognition with a diagram stimulus: every marker is a choice
//     (derived by the server), the roots are correct, and the labels are
//     hidden so they don't give the answer away;
//   - image_choice whose three options are diagram thumbnails (an odd
//     count, so the last one sits alone on its row).
func seedDiagramExercises(ctx context.Context, teacher domain.User, exerciseSvc *application.ExerciseService, classifier *classificationSeeder, diagrams seededDiagrams, challengeID string) error {
	scalesID, err := classifier.skillID(ctx, "Scales")
	if err != nil {
		return err
	}
	pentatonicID, err := classifier.conceptID(ctx, "Pentatonic scale shapes")
	if err != nil {
		return err
	}
	chordsID, err := classifier.skillID(ctx, "Chords")
	if err != nil {
		return err
	}
	openChordsID, err := classifier.conceptID(ctx, "Open chord shapes")
	if err != nil {
		return err
	}

	rootsTitle := "Tap every root of the A minor pentatonic"
	roots := &domain.DiagramRef{
		DiagramID:        diagrams.pentatonicPos1.ID,
		Layers:           domain.DiagramLayers{Intervals: false},
		CorrectIntervals: &[]string{"R"},
	}
	tapRoots, err := exerciseSvc.CreateExercise(ctx, teacher, rootsTitle, domain.NewPlainTextPrompt(rootsTitle), domain.ExerciseTypeImageRecognition,
		[]string{scalesID}, []string{pentatonicID}, nil, nil, roots, nil, nil, nil, nil, []string{"en"})
	if err != nil {
		return fmt.Errorf("create exercise %q: %w", rootsTitle, err)
	}

	chordTitle := "Which diagram shows the E major chord?"
	pickChord, err := exerciseSvc.CreateExercise(ctx, teacher, chordTitle, domain.NewPlainTextPrompt(chordTitle), domain.ExerciseTypeImageChoice,
		[]string{chordsID}, []string{openChordsID}, nil, nil, nil, nil, []domain.Option{
			{ID: uuid.NewString(), IsCorrect: true, DiagramRef: intervalsRef(diagrams.eMajorChord.ID)},
			{ID: uuid.NewString(), IsCorrect: false, DiagramRef: intervalsRef(diagrams.cMajorOpen.ID)},
			{ID: uuid.NewString(), IsCorrect: false, DiagramRef: intervalsRef(diagrams.pentatonicPos1.ID)},
		}, nil, nil, []string{"en"})
	if err != nil {
		return fmt.Errorf("create exercise %q: %w", chordTitle, err)
	}

	for _, exercise := range []domain.Exercise{tapRoots, pickChord} {
		if _, err := exerciseSvc.LinkExerciseToChallenge(ctx, teacher, challengeID, exercise.ID); err != nil {
			return fmt.Errorf("link exercise %q to challenge: %w", exercise.Title, err)
		}
	}
	return nil
}
