package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// soundOf is a diagram played as an exercise's sound: the playback with that
// English name, never looping, at tempoBPM when given (else the playback's own).
func soundOf(diagram domain.Diagram, playbackName string, tempoBPM *int) (*domain.DiagramRef, error) {
	for _, playback := range diagram.Playbacks {
		if playback.Names["en"] != playbackName {
			continue
		}
		id := playback.ID
		return &domain.DiagramRef{
			DiagramID: diagram.ID,
			Layers:    domain.DiagramLayers{Intervals: true},
			Playback:  &domain.DiagramRefPlayback{PlaybackID: &id, Direction: domain.DiagramPlaybackDirectionAsAuthored, TempoBPM: tempoBPM},
		}, nil
	}
	return nil, fmt.Errorf("diagram %s has no playback named %q", diagram.ID, playbackName)
}

// catalogChord is the chord catalog's first voicing of symbol, as a diagram.
func catalogChord(ctx context.Context, teacher domain.User, chords *application.ChordCatalogService, diagramSvc *application.DiagramService, symbol string) (domain.Diagram, error) {
	search, err := chords.SearchChords(ctx, teacher, symbol)
	if err != nil {
		return domain.Diagram{}, fmt.Errorf("find %s in the chord catalog: %w", symbol, err)
	}
	if search.Chord == nil || len(search.Chord.Voicings) == 0 {
		return domain.Diagram{}, fmt.Errorf("the chord catalog has no voicing of %s", symbol)
	}
	diagram, err := diagramSvc.GetDiagram(ctx, search.Chord.Voicings[0].DiagramID)
	if err != nil {
		return domain.Diagram{}, fmt.Errorf("load the %s voicing: %w", symbol, err)
	}
	return diagram, nil
}

// seedListeningDiagramExercises creates the listening exercises whose sound is
// a catalog chord voicing's playback rather than an uploaded clip, classified
// like the other chord-quality listening so a guitar practice session picks
// them, and links them into challengeID so the challenge run and its review
// show them too:
//   - audio_recognition on A minor, strummed down: is it major or minor?
//   - audio_recognition on A major, as an arpeggio at 72 BPM, which the
//     reveal plays again without offering the voicing's other playback;
//   - audio_selection of three strummed chords, G major, C major and A
//     minor, labelled "Sound 1" to "Sound 3": which one is minor?
func seedListeningDiagramExercises(ctx context.Context, teacher domain.User, exerciseSvc *application.ExerciseService, diagramSvc *application.DiagramService,
	chords *application.ChordCatalogService, classifier *classificationSeeder, challengeID string) ([]domain.Exercise, error) {
	skillID, conceptID, err := classifier.ids(ctx, listeningClassification)
	if err != nil {
		return nil, err
	}
	voicings := map[string]domain.Diagram{}
	for _, symbol := range []string{"Am", "A", "G", "C"} {
		if voicings[symbol], err = catalogChord(ctx, teacher, chords, diagramSvc, symbol); err != nil {
			return nil, err
		}
	}
	label := func(s string) *string { return &s }
	majorOrMinor := func() []domain.Option {
		return []domain.Option{
			{ID: uuid.NewString(), IsCorrect: false, Label: label("Major")},
			{ID: uuid.NewString(), IsCorrect: false, Label: label("Minor")},
		}
	}

	strummedMinor, err := soundOf(voicings["Am"], "Strum down", nil)
	if err != nil {
		return nil, err
	}
	minorOptions := majorOrMinor()
	minorOptions[1].IsCorrect = true
	minorTitle := "Major or minor?"
	hearMinor, err := exerciseSvc.CreateExercise(ctx, teacher, minorTitle, domain.NewPlainTextPrompt("Is this chord major or minor?"), domain.ExerciseTypeAudioRecognition,
		[]string{skillID}, []string{conceptID}, nil, nil, strummedMinor, nil, minorOptions, nil, nil, []string{"en"}, forGuitars())
	if err != nil {
		return nil, fmt.Errorf("create exercise %q: %w", minorTitle, err)
	}

	slowTempo := 72
	arpeggioMajor, err := soundOf(voicings["A"], "Arpeggio", &slowTempo)
	if err != nil {
		return nil, err
	}
	majorOptions := majorOrMinor()
	majorOptions[0].IsCorrect = true
	majorTitle := "Major or minor, note by note?"
	hearMajor, err := exerciseSvc.CreateExercise(ctx, teacher, majorTitle, domain.NewPlainTextPrompt("Is this chord, played one note at a time, major or minor?"),
		domain.ExerciseTypeAudioRecognition, []string{skillID}, []string{conceptID}, nil, nil, arpeggioMajor, nil, majorOptions, nil, nil, []string{"en"}, forGuitars())
	if err != nil {
		return nil, fmt.Errorf("create exercise %q: %w", majorTitle, err)
	}

	var soundOptions []domain.Option
	for i, symbol := range []string{"G", "C", "Am"} {
		sound, err := soundOf(voicings[symbol], "Strum down", nil)
		if err != nil {
			return nil, err
		}
		soundOptions = append(soundOptions, domain.Option{
			ID: uuid.NewString(), IsCorrect: symbol == "Am", Label: label(fmt.Sprintf("Sound %d", i+1)), DiagramRef: sound,
		})
	}
	pickTitle := "Which one is minor?"
	pickMinor, err := exerciseSvc.CreateExercise(ctx, teacher, pickTitle, domain.NewPlainTextPrompt("Which of these sounds is a minor chord?"), domain.ExerciseTypeAudioSelection,
		[]string{skillID}, []string{conceptID}, nil, nil, nil, nil, soundOptions, nil, nil, []string{"en"}, forGuitars())
	if err != nil {
		return nil, fmt.Errorf("create exercise %q: %w", pickTitle, err)
	}

	exercises := []domain.Exercise{hearMinor, hearMajor, pickMinor}
	for _, exercise := range exercises {
		if _, err := exerciseSvc.LinkExerciseToChallenge(ctx, teacher, challengeID, exercise.ID); err != nil {
			return nil, fmt.Errorf("link exercise %q to challenge: %w", exercise.Title, err)
		}
	}
	return exercises, nil
}
