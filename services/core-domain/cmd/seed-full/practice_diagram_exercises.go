package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// diagramCatalogNamespace is the namespace the basic guitar diagram catalog
// derives each diagram's stable id from its key, so a seed can name a
// catalog diagram without a lookup.
var diagramCatalogNamespace = uuid.MustParse("4ac75155-7804-5527-a6ba-01b73c0e3e1a")

// catalogDiagramID is the stable id of the catalog diagram with key.
func catalogDiagramID(key string) string {
	return uuid.NewSHA1(diagramCatalogNamespace, []byte(key)).String()
}

// fullNeckMinorPentatonicKey is the catalog's A minor pentatonic from the
// open strings to the 12th fret: too wide for a phone at tap size.
const fullNeckMinorPentatonicKey = "scale/minor-pentatonic/A/frets-0-12"

// seedPracticeDiagramExercises creates the exercises that show, in a guitar
// practice session, each way a diagram can look there, classified under
// skills on the admin's path so a session picks them:
//   - the full-neck A minor pentatonic whose roots are tapped: a board too
//     wide for a phone, scrolled with the overview strip under it;
//   - two pentatonic boxes stacked in the prompt, each in its own hue and
//     named in a legend, asked about with text answers;
//   - a catalog chord voicing in the prompt, drawn as a chord box (its chord
//     data has the fingers) with Box | Neck, asked by name.
func seedPracticeDiagramExercises(ctx context.Context, teacher domain.User, exerciseSvc *application.ExerciseService, chords *application.ChordCatalogService,
	classifier *classificationSeeder, diagrams seededDiagrams, challengeID string) ([]domain.Exercise, error) {
	pentatonic := exerciseClassification{skill: "play-pentatonic-positions", concept: "pentatonic-shapes"}
	openChords := exerciseClassification{skill: "play-open-chords", concept: "open-chord-shapes"}
	pentatonicSkill, pentatonicConcept, err := classifier.ids(ctx, pentatonic)
	if err != nil {
		return nil, err
	}
	chordSkill, chordConcept, err := classifier.ids(ctx, openChords)
	if err != nil {
		return nil, err
	}
	label := func(s string) *string { return &s }
	paragraph := func(s string) domain.PromptNode {
		return domain.PromptNode{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{{Type: domain.PromptNodeTypeText, Text: s}}}
	}

	wideTitle := "Tap every A of the A minor pentatonic, up to the 12th fret"
	wideStimulus := &domain.DiagramRef{
		DiagramID:        catalogDiagramID(fullNeckMinorPentatonicKey),
		Layers:           domain.DiagramLayers{Intervals: false},
		CorrectIntervals: &[]string{"R"},
	}
	wide, err := exerciseSvc.CreateExercise(ctx, teacher, wideTitle, domain.NewPlainTextPrompt(wideTitle), domain.ExerciseTypeImageRecognition,
		[]string{pentatonicSkill}, []string{pentatonicConcept}, nil, nil, wideStimulus, nil, nil, nil, nil, []string{"en"}, forGuitars())
	if err != nil {
		return nil, fmt.Errorf("create exercise %q: %w", wideTitle, err)
	}

	stackTitle := "Where do pentatonic boxes 1 and 2 meet?"
	stack := &domain.DiagramStackRef{Stack: []domain.DiagramRef{*intervalsRef(diagrams.pentatonicPos1.ID), *intervalsRef(diagrams.pentatonicPos2.ID)}}
	stackPrompt := domain.PromptDocument{Type: "doc", Content: []domain.PromptNode{
		paragraph("Boxes 1 and 2 of the A minor pentatonic, on one board."),
		{Type: domain.PromptNodeTypeDiagram, Attrs: &domain.PromptNodeAttrs{DiagramStackRef: stack}},
		paragraph(stackTitle),
	}}
	shared, err := exerciseSvc.CreateExercise(ctx, teacher, stackTitle, stackPrompt, domain.ExerciseTypeTextResponse,
		[]string{pentatonicSkill}, []string{pentatonicConcept}, nil, nil, nil, nil, []domain.Option{
			{ID: uuid.NewString(), IsCorrect: true, Label: label("They share the notes around the 7th and 8th frets")},
			{ID: uuid.NewString(), IsCorrect: false, Label: label("They share no notes")},
			{ID: uuid.NewString(), IsCorrect: false, Label: label("They share only the open strings")},
		}, nil, nil, []string{"en"}, forGuitars())
	if err != nil {
		return nil, fmt.Errorf("create exercise %q: %w", stackTitle, err)
	}

	search, err := chords.SearchChords(ctx, teacher, "A")
	if err != nil {
		return nil, fmt.Errorf("find the A chord in the chord catalog: %w", err)
	}
	if search.Chord == nil || len(search.Chord.Voicings) == 0 {
		return nil, fmt.Errorf("the chord catalog has no voicing of A")
	}
	chordTitle := "What chord is this?"
	chordPrompt := domain.PromptDocument{Type: "doc", Content: []domain.PromptNode{
		{Type: domain.PromptNodeTypeDiagram, Attrs: &domain.PromptNodeAttrs{DiagramRef: intervalsRef(search.Chord.Voicings[0].DiagramID)}},
		paragraph(chordTitle),
	}}
	nameChord, err := exerciseSvc.CreateExercise(ctx, teacher, chordTitle, chordPrompt, domain.ExerciseTypeTextResponse,
		[]string{chordSkill}, []string{chordConcept}, nil, nil, nil, nil, []domain.Option{
			{ID: uuid.NewString(), IsCorrect: true, Label: label("A major")},
			{ID: uuid.NewString(), IsCorrect: false, Label: label("A minor")},
			{ID: uuid.NewString(), IsCorrect: false, Label: label("D major")},
		}, nil, nil, []string{"en"}, forGuitars())
	if err != nil {
		return nil, fmt.Errorf("create exercise %q: %w", chordTitle, err)
	}

	exercises := []domain.Exercise{wide, shared, nameChord}
	for _, exercise := range exercises {
		if _, err := exerciseSvc.LinkExerciseToChallenge(ctx, teacher, challengeID, exercise.ID); err != nil {
			return nil, fmt.Errorf("link exercise %q to challenge: %w", exercise.Title, err)
		}
	}
	return exercises, nil
}
