package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// seedDiagramExercises creates the three exercises a student answers on a
// prebuilt diagram, and links them into challengeID:
//   - image_recognition with a diagram stimulus: every marker is a choice
//     (derived by the server), the roots are correct, and the labels are
//     hidden so they don't give the answer away;
//   - image_choice whose three options are diagram thumbnails (an odd
//     count, so the last one sits alone on its row);
//   - image_recognition on the E major chord that offers Play: its sequence
//     plays reversed and loops, and the open high E sounds though it isn't
//     drawn (it stays an empty answer cell). The major third is correct.
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

	listen, err := seedListeningExercise(ctx, teacher, exerciseSvc, diagrams.eMajorChord, chordsID, openChordsID)
	if err != nil {
		return err
	}

	for _, exercise := range []domain.Exercise{tapRoots, pickChord, listen} {
		if _, err := exerciseSvc.LinkExerciseToChallenge(ctx, teacher, challengeID, exercise.ID); err != nil {
			return fmt.Errorf("link exercise %q to challenge: %w", exercise.Title, err)
		}
	}
	return nil
}

// seedListeningExercise asks for the E major chord's major third on a
// diagram the student can hear: labels off, the chord's sequence reversed
// and looping, and the open high E hidden but still sounding.
func seedListeningExercise(ctx context.Context, teacher domain.User, exerciseSvc *application.ExerciseService, eMajor domain.Diagram, skillID, conceptID string) (domain.Exercise, error) {
	var third, highE string
	for _, p := range eMajor.Positions {
		switch {
		case p.Interval == "3":
			third = p.ID
		case p.String != nil && *p.String == 1:
			highE = p.ID
		}
	}
	if third == "" || highE == "" {
		return domain.Exercise{}, fmt.Errorf("E major chord has no major third or high E position")
	}

	title := "Listen to the E major chord, then tap its major third"
	stimulus := &domain.DiagramRef{
		DiagramID:          eMajor.ID,
		Layers:             domain.DiagramLayers{Intervals: false, HiddenPositionIDs: &[]string{highE}},
		Playback:           &domain.DiagramPlayback{Direction: domain.DiagramPlaybackDirectionReversed, Loop: true},
		CorrectPositionIDs: &[]string{third},
	}
	exercise, err := exerciseSvc.CreateExercise(ctx, teacher, title, domain.NewPlainTextPrompt(title), domain.ExerciseTypeImageRecognition,
		[]string{skillID}, []string{conceptID}, nil, nil, stimulus, nil, nil, nil, nil, []string{"en"})
	if err != nil {
		return domain.Exercise{}, fmt.Errorf("create exercise %q: %w", title, err)
	}
	return exercise, nil
}
