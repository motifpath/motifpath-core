package application_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// noShuffle is a shuffle func that never reorders anything — the default for
// tests that don't exercise shuffling, so their expected order stays
// deterministic without depending on shuffle behavior.
func noShuffle(int, func(i, j int)) {}

// reverseShuffle reverses element order deterministically — used by tests
// that need to observe "shuffling happened" without real randomness.
func reverseShuffle(n int, swap func(i, j int)) {
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

func newExerciseService(challenges *fakeChallengeRepository, exercises *fakeExerciseRepository) *application.ExerciseService {
	return newExerciseServiceWithNodes(challenges, exercises, newFakeContentNodeRepository())
}

func newExerciseServiceWithNodes(challenges *fakeChallengeRepository, exercises *fakeExerciseRepository, nodes *fakeContentNodeRepository) *application.ExerciseService {
	return newExerciseServiceWithDiagrams(challenges, exercises, nodes, newFakeDiagramRepository())
}

func newExerciseServiceWithDiagrams(challenges *fakeChallengeRepository, exercises *fakeExerciseRepository, nodes *fakeContentNodeRepository, diagrams *fakeDiagramRepository) *application.ExerciseService {
	return application.NewExerciseService(challenges, exercises, nodes, seededSkillRepository(), seededConceptRepository(), diagrams, idSequence(), func() time.Time { return fixedCreatedAt }, noShuffle)
}

func imageRecognitionOptions() []domain.Option {
	return []domain.Option{
		{ID: "opt-1", IsCorrect: true, Region: &domain.OptionRegion{X: 0.2, Y: 0.3, Width: 0.1, Height: 0.1, Shape: domain.OptionRegionShapeRectangle}},
		{ID: "opt-2", IsCorrect: false, Region: &domain.OptionRegion{X: 0.5, Y: 0.3, Width: 0.1, Height: 0.1, Shape: domain.OptionRegionShapeRectangle}},
	}
}

// richlyFormattedPrompt builds a PromptDocument exercising every node type
// and every mark type the exercise-prompt editor can produce.
func richlyFormattedPrompt() domain.PromptDocument {
	level := 2
	href := "https://example.com/circle-of-fifths"
	src := "https://cdn.example.com/library/circle-of-fifths.png"
	alt := "Circle of fifths diagram"
	color := "#6d28e0"
	cellBackground := "#f3ecff"
	cellBorder := "transparent"

	return domain.PromptDocument{
		Type: "doc",
		Content: []domain.PromptNode{
			{
				Type:  domain.PromptNodeTypeHeading,
				Attrs: &domain.PromptNodeAttrs{Level: &level},
				Content: []domain.PromptNode{
					{Type: domain.PromptNodeTypeText, Text: "Circle of fifths"},
				},
			},
			{
				Type: domain.PromptNodeTypeParagraph,
				Content: []domain.PromptNode{
					{
						Type: domain.PromptNodeTypeText,
						Text: "bold, italic, strike, and highlighted text, plus a link",
						Marks: []domain.PromptMark{
							{Type: domain.PromptMarkTypeBold},
							{Type: domain.PromptMarkTypeItalic},
							{Type: domain.PromptMarkTypeStrike},
							{Type: domain.PromptMarkTypeHighlight},
							{Type: domain.PromptMarkTypeLink, Attrs: &domain.PromptMarkAttrs{Href: &href}},
							{Type: domain.PromptMarkTypeTextStyle, Attrs: &domain.PromptMarkAttrs{Color: &color}},
						},
					},
				},
			},
			{
				Type: domain.PromptNodeTypeBulletList,
				Content: []domain.PromptNode{
					{Type: domain.PromptNodeTypeListItem, Content: []domain.PromptNode{
						{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{
							{Type: domain.PromptNodeTypeText, Text: "Major keys"},
						}},
					}},
				},
			},
			{
				Type: domain.PromptNodeTypeOrderedList,
				Content: []domain.PromptNode{
					{Type: domain.PromptNodeTypeListItem, Content: []domain.PromptNode{
						{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{
							{Type: domain.PromptNodeTypeText, Text: "Step one"},
						}},
					}},
				},
			},
			{
				Type: domain.PromptNodeTypeTable,
				Content: []domain.PromptNode{
					{Type: domain.PromptNodeTypeTableRow, Content: []domain.PromptNode{
						{Type: domain.PromptNodeTypeTableHeader, Content: []domain.PromptNode{
							{Type: domain.PromptNodeTypeText, Text: "Key"},
						}},
						{
							Type:  domain.PromptNodeTypeTableCell,
							Attrs: &domain.PromptNodeAttrs{BackgroundColor: &cellBackground, BorderColor: &cellBorder},
							Content: []domain.PromptNode{
								{Type: domain.PromptNodeTypeText, Text: "C major"},
							},
						},
					}},
				},
			},
			{
				Type:  domain.PromptNodeTypeImage,
				Attrs: &domain.PromptNodeAttrs{Src: &src, Alt: &alt},
			},
		},
	}
}

func textResponseOptions() []domain.Option {
	label1, label2 := "A major", "A minor"
	return []domain.Option{
		{ID: "opt-1", IsCorrect: true, Label: &label1},
		{ID: "opt-2", IsCorrect: false, Label: &label2},
	}
}

func TestExerciseService_CreateExercise(t *testing.T) {
	t.Run("a teacher creates a standalone image_recognition exercise", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		imageURL := "https://cdn.example.com/fretboard/c-major-triad.png"

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Root position of a C major triad", domain.NewPlainTextPrompt("Identify the root position of a C major triad"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, &imageURL, nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.Equal(t, "Root position of a C major triad", exercise.Title)
		assert.Empty(t, exercise.ChallengeIDs)
	})

	t.Run("an admin creates an exercise", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), adminCaller(),
			"Name the interval", domain.NewPlainTextPrompt("Name the interval between the open low E and the 5th fret"),
			domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
	})

	t.Run("a teacher creates an exercise with skills and concepts", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		imageURL := "https://cdn.example.com/fretboard/descending-run.png"

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Alternate picking — descending run", domain.NewPlainTextPrompt("Play the descending run cleanly"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1", "skill-2"}, []string{"concept-1"}, &imageURL, nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"skill-1", "skill-2"}, exerciseSkillIDs(exercise))
	})

	t.Run("a teacher creates an exercise with a richly formatted prompt", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		richPrompt := richlyFormattedPrompt()

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Circle of fifths", richPrompt,
			domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.Equal(t, richPrompt, exercise.Prompt)
	})

	t.Run("a teacher creates an exercise with a prompt using a custom font color and background color", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		color, background := "#6d28e0", "#f3ecff"
		prompt := domain.PromptDocument{
			Type: "doc",
			Content: []domain.PromptNode{
				{
					Type: domain.PromptNodeTypeParagraph,
					Content: []domain.PromptNode{
						{
							Type: domain.PromptNodeTypeText,
							Text: "Circle of fifths",
							Marks: []domain.PromptMark{
								{Type: domain.PromptMarkTypeTextStyle, Attrs: &domain.PromptMarkAttrs{Color: &color, BackgroundColor: &background}},
							},
						},
					},
				},
			},
		}

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Circle of fifths", prompt, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.Equal(t, prompt, exercise.Prompt)
	})

	t.Run("a teacher creates an exercise with a plain, unformatted prompt", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the note", domain.NewPlainTextPrompt("What note is this?"),
			domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.Equal(t, domain.NewPlainTextPrompt("What note is this?"), exercise.Prompt)
	})

	t.Run("creating an exercise with an unstructured prompt is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.PromptDocument{Type: "not-a-doc"}, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "prompt")
	})

	t.Run("creating an exercise with a prompt using an unsupported node type is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		prompt := domain.PromptDocument{
			Type: "doc",
			Content: []domain.PromptNode{
				{Type: "footnote"},
			},
		}

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", prompt, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "prompt")
	})

	// audio/video node types are valid in the rich_text ExpandedContent and
	// remediation-target editors, which share this same prompt-document
	// validation, but the exercise-prompt authoring toolbar never offers
	// them — an exercise's own prompt must still reject them.
	for _, nodeType := range []string{"audio", "video"} {
		t.Run("creating an exercise with a prompt using a "+nodeType+" node is rejected", func(t *testing.T) {
			svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
			prompt := domain.PromptDocument{
				Type: "doc",
				Content: []domain.PromptNode{
					{Type: domain.PromptNodeType(nodeType)},
				},
			}

			_, err := svc.CreateExercise(context.Background(), teacherCaller(),
				"title", prompt, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

			var valErr *domain.ValidationError
			require.True(t, errors.As(err, &valErr))
			assertHasField(t, valErr, "prompt")
		})
	}

	t.Run("creating an exercise without a title is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "title")
	})

	t.Run("creating an exercise without a prompt is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.PromptDocument{}, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "prompt")
	})

	t.Run("creating an exercise without an exercise type is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), "", []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "exercise_type")
	})

	t.Run("creating an exercise with an unrecognised type is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseType("multiple_choice"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "exercise_type")
	})

	t.Run("creating an exercise with zero correct options is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		label := "A major"

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil,
			[]domain.Option{{ID: "opt-1", IsCorrect: false, Label: &label}}, nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "options")
	})

	t.Run("creating an exercise with no skills is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, nil, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "skill_ids")
	})

	t.Run("creating an exercise with no concepts is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, nil, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "concept_ids")
	})

	t.Run("creating an exercise with a skill id that does not exist is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"missing-skill"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "skill_ids")
	})

	t.Run("a student cannot create an exercise", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), studentCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func richRemediationContent(caption string) domain.PromptDocument {
	return domain.PromptDocument{
		Type: "doc",
		Content: []domain.PromptNode{
			{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{
				{Type: domain.PromptNodeTypeText, Text: caption},
			}},
		},
	}
}

func seedDiagram(t *testing.T, diagrams *fakeDiagramRepository, id, instrumentID string, positions []domain.Position) domain.Diagram {
	t.Helper()
	diagram := domain.Diagram{ID: id, InstrumentID: instrumentID, Name: id, Positions: positions, Skills: []domain.Skill{{ID: "skill-1"}}, Concepts: []domain.Concept{{ID: "concept-1"}}}
	require.NoError(t, diagrams.Create(context.Background(), diagram))
	return diagram
}

func TestExerciseService_DiagramDrivenImageRecognition(t *testing.T) {
	rootPos := domain.Position{ID: "pos-root", Interval: "R", NoteName: "A"}
	thirdPos := domain.Position{ID: "pos-third", Interval: "3", NoteName: "C#"}
	fifthPos := domain.Position{ID: "pos-fifth", Interval: "5", NoteName: "E"}

	t.Run("diagram_ref derives options from the diagram's positions", func(t *testing.T) {
		diagrams := newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", []domain.Position{rootPos, thirdPos, fifthPos})
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the root", domain.NewPlainTextPrompt("Which position is the root?"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil,
			&domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"R"}}, nil,
			nil, nil, nil, []string{"en"})

		require.NoError(t, err)
		require.Len(t, exercise.Options, 3)
		correctCount := 0
		for _, opt := range exercise.Options {
			require.NotNil(t, opt.DiagramID)
			assert.Equal(t, "diagram-1", *opt.DiagramID)
			require.NotNil(t, opt.DiagramPositionID)
			if opt.IsCorrect {
				correctCount++
				assert.Equal(t, "pos-root", *opt.DiagramPositionID)
			}
		}
		assert.Equal(t, 1, correctCount)
	})

	t.Run("diagram_ref's layers.subset filters which positions become options", func(t *testing.T) {
		diagrams := newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", []domain.Position{rootPos, thirdPos, fifthPos})
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the root", domain.NewPlainTextPrompt("Which position is the root?"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil,
			&domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true, Subset: &[]string{"R", "5"}}, CorrectIntervals: &[]string{"R"}}, nil,
			nil, nil, nil, []string{"en"})

		require.NoError(t, err)
		require.Len(t, exercise.Options, 2)
	})

	t.Run("diagram_stack_ref combines options from every entry sharing an instrument", func(t *testing.T) {
		diagrams := newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", []domain.Position{rootPos})
		seedDiagram(t, diagrams, "diagram-2", "guitar", []domain.Position{thirdPos})
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the notes", domain.NewPlainTextPrompt("Identify each note"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil,
			&domain.DiagramStackRef{Stack: []domain.DiagramRef{
				{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"R"}},
				{DiagramID: "diagram-2", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"3"}},
			}},
			nil, nil, nil, []string{"en"})

		require.NoError(t, err)
		require.Len(t, exercise.Options, 2)
	})

	t.Run("a diagram_stack_ref mixing instruments is rejected", func(t *testing.T) {
		diagrams := newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", []domain.Position{rootPos})
		seedDiagram(t, diagrams, "diagram-2", "piano", []domain.Position{thirdPos})
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the notes", domain.NewPlainTextPrompt("Identify each note"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil,
			&domain.DiagramStackRef{Stack: []domain.DiagramRef{
				{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"R"}},
				{DiagramID: "diagram-2", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"3"}},
			}},
			nil, nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "diagram_stack_ref")
	})

	t.Run("a diagram_ref pointing at a non-existent diagram is rejected", func(t *testing.T) {
		diagrams := newFakeDiagramRepository()
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the root", domain.NewPlainTextPrompt("Which position is the root?"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil,
			&domain.DiagramRef{DiagramID: "missing", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"R"}}, nil,
			nil, nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "diagram_ref")
	})

	t.Run("a diagram_stack_ref entry pointing at a non-existent diagram is rejected", func(t *testing.T) {
		diagrams := newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", []domain.Position{rootPos})
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the root", domain.NewPlainTextPrompt("Which position is the root?"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil,
			nil, &domain.DiagramStackRef{Stack: []domain.DiagramRef{
				{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"R"}},
				{DiagramID: "missing", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"R"}},
			}},
			nil, nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "diagram_stack_ref")
	})

	t.Run("supplying options alongside a diagram_ref is rejected", func(t *testing.T) {
		diagrams := newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", []domain.Position{rootPos})
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the root", domain.NewPlainTextPrompt("Which position is the root?"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil,
			&domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"R"}}, nil,
			imageRecognitionOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "options")
	})
}

func TestExerciseService_RemediationTargets(t *testing.T) {
	t.Run("a teacher creates an exercise with an internal remediation target", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("triad-remediation"))
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), newFakeExerciseRepository(), nodes)
		nodeID := "triad-remediation"

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil,
			[]domain.RemediationTarget{{ContentNodeID: &nodeID}}, []string{"en"})

		require.NoError(t, err)
		require.Len(t, exercise.RemediationTargets, 1)
		require.NotNil(t, exercise.RemediationTargets[0].ContentNodeID)
		assert.Equal(t, nodeID, *exercise.RemediationTargets[0].ContentNodeID)
	})

	t.Run("a teacher creates an exercise with inline rich-content remediation", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		caption := "Watch this if the shape felt unfamiliar"
		rich := richRemediationContent("a helpful video")

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil,
			[]domain.RemediationTarget{{RichContent: &rich, Caption: &caption}}, []string{"en"})

		require.NoError(t, err)
		require.Len(t, exercise.RemediationTargets, 1)
		require.NotNil(t, exercise.RemediationTargets[0].RichContent)
		assert.Equal(t, rich, *exercise.RemediationTargets[0].RichContent)
		require.NotNil(t, exercise.RemediationTargets[0].Caption)
		assert.Equal(t, caption, *exercise.RemediationTargets[0].Caption)
	})

	t.Run("a teacher creates an exercise with multiple remediation targets in priority order", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("triad-remediation"))
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), newFakeExerciseRepository(), nodes)
		nodeID := "triad-remediation"
		rich := richRemediationContent("read this instead")

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil,
			[]domain.RemediationTarget{{ContentNodeID: &nodeID}, {RichContent: &rich}}, []string{"en"})

		require.NoError(t, err)
		require.Len(t, exercise.RemediationTargets, 2)
		require.NotNil(t, exercise.RemediationTargets[0].ContentNodeID)
		require.NotNil(t, exercise.RemediationTargets[1].RichContent)
	})

	t.Run("a teacher replaces an exercise's remediation targets", func(t *testing.T) {
		oldTarget := "triad-remediation"
		newTarget := "picking-fundamentals"
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode(oldTarget))
		nodes.put(videoNode(newTarget))
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{
			ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t",
			Prompt: domain.NewPlainTextPrompt("p"), Options: textResponseOptions(),
			RemediationTargets: []domain.RemediationTarget{{ContentNodeID: &oldTarget}},
		})
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), exercises, nodes)

		exercise, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"t", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil,
			[]domain.RemediationTarget{{ContentNodeID: &newTarget}}, []string{"en"})

		require.NoError(t, err)
		require.Len(t, exercise.RemediationTargets, 1)
		require.NotNil(t, exercise.RemediationTargets[0].ContentNodeID)
		assert.Equal(t, newTarget, *exercise.RemediationTargets[0].ContentNodeID)
	})

	t.Run("a teacher clears an exercise's remediation targets", func(t *testing.T) {
		oldTarget := "triad-remediation"
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{
			ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t",
			Prompt: domain.NewPlainTextPrompt("p"), Options: textResponseOptions(),
			RemediationTargets: []domain.RemediationTarget{{ContentNodeID: &oldTarget}},
		})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		exercise, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"t", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.Empty(t, exercise.RemediationTargets)
	})

	t.Run("creating an exercise with a remediation target carrying both a content node and rich content is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("triad-remediation"))
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), newFakeExerciseRepository(), nodes)
		nodeID := "triad-remediation"
		rich := richRemediationContent("x")

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil,
			[]domain.RemediationTarget{{ContentNodeID: &nodeID, RichContent: &rich}}, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "remediation_targets")
	})

	t.Run("creating an exercise with a remediation target carrying neither a content node nor rich content is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil,
			[]domain.RemediationTarget{{}}, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "remediation_targets")
	})

	t.Run("creating an exercise with a remediation target referencing a non-existent content node is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		missing := "missing"

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil,
			[]domain.RemediationTarget{{ContentNodeID: &missing}}, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "remediation_targets")
	})
}

func TestExerciseService_GetExercise(t *testing.T) {
	t.Run("retrieving an exercise that does not exist returns not found", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.GetExercise(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestExerciseService_ListExercises(t *testing.T) {
	t.Run("a teacher lists all exercises in the reusable pool", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeImageRecognition})
		exercises.put(domain.Exercise{ID: "picking-drill-01", ExerciseType: domain.ExerciseTypeTextResponse})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExercises(context.Background(), teacherCaller(), "", "")

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"triad-exercise-01", "picking-drill-01"}, idsOf(got))
	})

	t.Run("an admin lists all exercises in the reusable pool", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01"})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExercises(context.Background(), adminCaller(), "", "")

		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("a teacher filters the exercise list by skill", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", Skills: []domain.Skill{{ID: "skill-1"}}})
		exercises.put(domain.Exercise{ID: "picking-drill-01", Skills: []domain.Skill{{ID: "skill-2"}}})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExercises(context.Background(), teacherCaller(), "skill-2", "")

		require.NoError(t, err)
		assert.Equal(t, []string{"picking-drill-01"}, idsOf(got))
	})

	t.Run("a teacher filters the exercise list by exercise type", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeImageRecognition})
		exercises.put(domain.Exercise{ID: "chord-name-01", ExerciseType: domain.ExerciseTypeTextResponse})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExercises(context.Background(), teacherCaller(), "", domain.ExerciseTypeTextResponse)

		require.NoError(t, err)
		assert.Equal(t, []string{"chord-name-01"}, idsOf(got))
	})

	t.Run("listing exercises when none exist returns an empty list", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		got, err := svc.ListExercises(context.Background(), teacherCaller(), "", "")

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a student cannot list all exercises", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.ListExercises(context.Background(), studentCaller(), "", "")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_UpdateExercise(t *testing.T) {
	t.Run("a teacher updates an exercise's title, prompt, and options", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "old title", Prompt: domain.NewPlainTextPrompt("old prompt"), Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		revisedPrompt := domain.NewPlainTextPrompt("Identify the root position, now with a cleaner prompt")
		exercise, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"Root position, revised", revisedPrompt, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.Equal(t, "Root position, revised", exercise.Title)
		assert.Equal(t, revisedPrompt, exercise.Prompt)
	})

	t.Run("a teacher updates an exercise's prompt to add formatting it previously lacked", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("plain"), Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)
		formatted := domain.PromptDocument{
			Type: "doc",
			Content: []domain.PromptNode{
				{
					Type: domain.PromptNodeTypeParagraph,
					Content: []domain.PromptNode{
						{Type: domain.PromptNodeTypeText, Text: "bold text", Marks: []domain.PromptMark{{Type: domain.PromptMarkTypeBold}}},
					},
				},
				{
					Type: domain.PromptNodeTypeBulletList,
					Content: []domain.PromptNode{
						{Type: domain.PromptNodeTypeListItem, Content: []domain.PromptNode{
							{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{
								{Type: domain.PromptNodeTypeText, Text: "a list item"},
							}},
						}},
					},
				},
			},
		}

		exercise, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"t", formatted, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.Equal(t, formatted, exercise.Prompt)
	})

	t.Run("a teacher replaces an exercise's skills", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "picking-drill-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("p"), Skills: []domain.Skill{{ID: "skill-1"}}, Concepts: []domain.Concept{{ID: "concept-1"}}, Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		exercise, err := svc.UpdateExercise(context.Background(), teacherCaller(), "picking-drill-01",
			"t", domain.NewPlainTextPrompt("p"), []string{"skill-1", "skill-2"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"skill-1", "skill-2"}, exerciseSkillIDs(exercise))
	})

	t.Run("updating an exercise does not change its links to challenges or content nodes", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("p"), ChallengeIDs: []string{"triad-challenge"}, Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		exercise, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"Root position, revised", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		require.NoError(t, err)
		assert.Equal(t, []string{"triad-challenge"}, exercise.ChallengeIDs)
	})

	t.Run("updating an exercise without a title is rejected", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("p"), Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		_, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "title")
	})

	t.Run("updating an exercise with zero correct options is rejected", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("p"), Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)
		label := "A major"

		_, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"t", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, []domain.Option{{ID: "opt-1", IsCorrect: false, Label: &label}}, nil, nil, []string{"en"})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "options")
	})

	t.Run("updating an exercise that does not exist returns not found", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.UpdateExercise(context.Background(), teacherCaller(), "missing",
			"t", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot update an exercise", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("p"), Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		_, err := svc.UpdateExercise(context.Background(), studentCaller(), "triad-exercise-01",
			"Hijacked title", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"})

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_LinkExerciseToChallenge(t *testing.T) {
	t.Run("a teacher links an existing exercise into a challenge", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(challenges, exercises)

		exercise, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")

		require.NoError(t, err)
		assert.Equal(t, []string{"challenge-1"}, exercise.ChallengeIDs)
	})

	t.Run("the same exercise is linked into a second challenge without duplication", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		challenges.put(domain.Challenge{ID: "challenge-2"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(challenges, exercises)
		_, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")
		require.NoError(t, err)

		exercise, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-2", "exercise-1")

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"challenge-1", "challenge-2"}, exercise.ChallengeIDs)
		assert.Equal(t, 1, exercises.count())
	})

	t.Run("linking a non-existent exercise to a challenge returns not found", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newExerciseService(challenges, newFakeExerciseRepository())

		_, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("linking an exercise to a non-existent challenge returns not found", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		_, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "missing", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("linking an exercise that is already linked to the challenge is rejected", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{"challenge-1"}})
		svc := newExerciseService(challenges, exercises)

		_, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("a student cannot link an exercise to a challenge", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(challenges, exercises)

		_, err := svc.LinkExerciseToChallenge(context.Background(), studentCaller(), "challenge-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_UnlinkExerciseFromChallenge(t *testing.T) {
	t.Run("a teacher unlinks an exercise from a challenge", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{"challenge-1"}})
		svc := newExerciseService(challenges, exercises)

		err := svc.UnlinkExerciseFromChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")
		require.NoError(t, err)

		exercise, err := svc.GetExercise(context.Background(), "exercise-1")
		require.NoError(t, err)
		assert.Empty(t, exercise.ChallengeIDs)
	})

	t.Run("unlinking an exercise that is not linked to the challenge returns not found", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}})
		svc := newExerciseService(challenges, exercises)

		err := svc.UnlinkExerciseFromChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot unlink an exercise from a challenge", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{"challenge-1"}})
		svc := newExerciseService(challenges, exercises)

		err := svc.UnlinkExerciseFromChallenge(context.Background(), studentCaller(), "challenge-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_ListExercisesForChallenge(t *testing.T) {
	t.Run("a student lists the exercises linked to a challenge", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{"challenge-1"}, Options: textResponseOptions()})
		svc := newExerciseService(challenges, exercises)

		got, err := svc.ListExercisesForChallenge(context.Background(), "challenge-1")

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "exercise-1", got[0].ID)
	})

	t.Run("a student lists the exercises for a challenge with none linked", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		svc := newExerciseService(challenges, newFakeExerciseRepository())

		got, err := svc.ListExercisesForChallenge(context.Background(), "challenge-1")

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("listing exercises for a challenge that does not exist returns not found", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.ListExercisesForChallenge(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a challenge with shuffling disabled always returns exercises in link order", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "ordered-challenge", ShuffleExercises: false})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ChallengeIDs: []string{"ordered-challenge"}})
		exercises.put(domain.Exercise{ID: "ex-2", ChallengeIDs: []string{"ordered-challenge"}})
		exercises.put(domain.Exercise{ID: "ex-3", ChallengeIDs: []string{"ordered-challenge"}})
		svc := application.NewExerciseService(challenges, exercises, newFakeContentNodeRepository(), seededSkillRepository(), seededConceptRepository(), newFakeDiagramRepository(), idSequence(), func() time.Time { return fixedCreatedAt }, reverseShuffle)

		first, err := svc.ListExercisesForChallenge(context.Background(), "ordered-challenge")
		require.NoError(t, err)
		second, err := svc.ListExercisesForChallenge(context.Background(), "ordered-challenge")
		require.NoError(t, err)

		wantOrder := []string{"ex-1", "ex-2", "ex-3"}
		assert.Equal(t, wantOrder, idsOf(first))
		assert.Equal(t, wantOrder, idsOf(second))
	})

	t.Run("a challenge with shuffling enabled reorders exercises per the injected shuffle", func(t *testing.T) {
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "shuffled-challenge", ShuffleExercises: true, ShuffleOptions: true})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ChallengeIDs: []string{"shuffled-challenge"}, Options: textResponseOptions()})
		exercises.put(domain.Exercise{ID: "ex-2", ChallengeIDs: []string{"shuffled-challenge"}, Options: textResponseOptions()})
		exercises.put(domain.Exercise{ID: "ex-3", ChallengeIDs: []string{"shuffled-challenge"}, Options: textResponseOptions()})
		svc := application.NewExerciseService(challenges, exercises, newFakeContentNodeRepository(), seededSkillRepository(), seededConceptRepository(), newFakeDiagramRepository(), idSequence(), func() time.Time { return fixedCreatedAt }, reverseShuffle)

		got, err := svc.ListExercisesForChallenge(context.Background(), "shuffled-challenge")

		require.NoError(t, err)
		assert.Equal(t, []string{"ex-3", "ex-2", "ex-1"}, idsOf(got))
		assert.Equal(t, "opt-2", got[0].Options[0].ID)
	})
}

func idsOf(exercises []domain.Exercise) []string {
	ids := make([]string, len(exercises))
	for i, e := range exercises {
		ids[i] = e.ID
	}
	return ids
}

func TestExerciseService_LinkExerciseToContentNode(t *testing.T) {
	t.Run("a teacher links an existing exercise into a content node as a path exercise", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ContentNodeIDs: []string{}})
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), exercises, nodes)

		exercise, err := svc.LinkExerciseToContentNode(context.Background(), teacherCaller(), "node-1", "exercise-1")

		require.NoError(t, err)
		assert.Equal(t, []string{"node-1"}, exercise.ContentNodeIDs)
	})

	t.Run("an exercise can be a path exercise on a node and linked to a challenge at the same time", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		challenges := newFakeChallengeRepository()
		challenges.put(domain.Challenge{ID: "challenge-1"})
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ChallengeIDs: []string{}, ContentNodeIDs: []string{}})
		svc := newExerciseServiceWithNodes(challenges, exercises, nodes)
		_, err := svc.LinkExerciseToContentNode(context.Background(), teacherCaller(), "node-1", "exercise-1")
		require.NoError(t, err)

		exercise, err := svc.LinkExerciseToChallenge(context.Background(), teacherCaller(), "challenge-1", "exercise-1")

		require.NoError(t, err)
		assert.Equal(t, []string{"node-1"}, exercise.ContentNodeIDs)
		assert.Equal(t, []string{"challenge-1"}, exercise.ChallengeIDs)
	})

	t.Run("linking an exercise that is already a path exercise on the node is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ContentNodeIDs: []string{"node-1"}})
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), exercises, nodes)

		_, err := svc.LinkExerciseToContentNode(context.Background(), teacherCaller(), "node-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	})

	t.Run("linking a non-existent exercise to a content node as a path exercise returns not found", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), newFakeExerciseRepository(), nodes)

		_, err := svc.LinkExerciseToContentNode(context.Background(), teacherCaller(), "node-1", "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("linking an exercise as a path exercise to a non-existent content node returns not found", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ContentNodeIDs: []string{}})
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), exercises, newFakeContentNodeRepository())

		_, err := svc.LinkExerciseToContentNode(context.Background(), teacherCaller(), "missing", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot link an exercise to a content node as a path exercise", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ContentNodeIDs: []string{}})
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), exercises, nodes)

		_, err := svc.LinkExerciseToContentNode(context.Background(), studentCaller(), "node-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_UnlinkExerciseFromContentNode(t *testing.T) {
	t.Run("a teacher unlinks a path exercise from a content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ContentNodeIDs: []string{"node-1"}})
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), exercises, nodes)

		err := svc.UnlinkExerciseFromContentNode(context.Background(), teacherCaller(), "node-1", "exercise-1")
		require.NoError(t, err)

		exercise, err := svc.GetExercise(context.Background(), "exercise-1")
		require.NoError(t, err)
		assert.Empty(t, exercise.ContentNodeIDs)
	})

	t.Run("unlinking a path exercise that is not linked to the node returns not found", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ContentNodeIDs: []string{}})
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), exercises, nodes)

		err := svc.UnlinkExerciseFromContentNode(context.Background(), teacherCaller(), "node-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot unlink a path exercise from a content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "exercise-1", ContentNodeIDs: []string{"node-1"}})
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), exercises, nodes)

		err := svc.UnlinkExerciseFromContentNode(context.Background(), studentCaller(), "node-1", "exercise-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_ListPathExercisesForContentNode(t *testing.T) {
	t.Run("a student lists the path exercises for a node that has some, always in link order", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "ex-1", ContentNodeIDs: []string{"node-1"}})
		exercises.put(domain.Exercise{ID: "ex-2", ContentNodeIDs: []string{"node-1"}})
		svc := application.NewExerciseService(newFakeChallengeRepository(), exercises, nodes, seededSkillRepository(), seededConceptRepository(), newFakeDiagramRepository(), idSequence(), func() time.Time { return fixedCreatedAt }, reverseShuffle)

		first, err := svc.ListPathExercisesForContentNode(context.Background(), "node-1")
		require.NoError(t, err)
		second, err := svc.ListPathExercisesForContentNode(context.Background(), "node-1")
		require.NoError(t, err)

		assert.Equal(t, []string{"ex-1", "ex-2"}, idsOf(first))
		assert.Equal(t, []string{"ex-1", "ex-2"}, idsOf(second))
	})

	t.Run("a student lists the path exercises for a node that has none", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), newFakeExerciseRepository(), nodes)

		got, err := svc.ListPathExercisesForContentNode(context.Background(), "node-1")

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("listing path exercises for a content node that does not exist returns not found", func(t *testing.T) {
		svc := newExerciseServiceWithNodes(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository())

		_, err := svc.ListPathExercisesForContentNode(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestExerciseService_StartPracticeSession(t *testing.T) {
	putWithSkill := func(exercises *fakeExerciseRepository, id, skillID string) {
		exercises.put(domain.Exercise{ID: id, Skills: []domain.Skill{{ID: skillID}}, Options: textResponseOptions()})
	}

	t.Run("a student starts a practice session for a skill with enough linked exercises", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		for i := 1; i <= 12; i++ {
			putWithSkill(exercises, "ex-"+strconv.Itoa(i), "skill-1")
		}
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		session, err := svc.StartPracticeSession(context.Background(), "skill-1", 10)

		require.NoError(t, err)
		assert.Len(t, session.Exercises, 10)
		assert.Equal(t, "skill-1", session.SkillID)
		assert.NotEmpty(t, session.ID)
		for _, e := range session.Exercises {
			assert.Contains(t, exerciseSkillIDs(e), "skill-1")
		}
	})

	t.Run("a practice session returns fewer exercises when the linked pool is smaller than requested", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		for i := 1; i <= 3; i++ {
			putWithSkill(exercises, "ex-"+strconv.Itoa(i), "skill-2")
		}
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		session, err := svc.StartPracticeSession(context.Background(), "skill-2", 10)

		require.NoError(t, err)
		assert.Len(t, session.Exercises, 3)
	})

	t.Run("a practice session defaults its count when none is given", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		for i := 1; i <= 12; i++ {
			putWithSkill(exercises, "ex-"+strconv.Itoa(i), "skill-1")
		}
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		session, err := svc.StartPracticeSession(context.Background(), "skill-1", 10)

		require.NoError(t, err)
		assert.Len(t, session.Exercises, 10)
	})

	t.Run("starting a practice session for a skill with no matching exercises returns an empty session", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		session, err := svc.StartPracticeSession(context.Background(), "skill-1", 10)

		require.NoError(t, err)
		assert.Empty(t, session.Exercises)
	})

	t.Run("two practice sessions for the same skill may differ in composition and order", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		for i := 1; i <= 12; i++ {
			putWithSkill(exercises, "ex-"+strconv.Itoa(i), "skill-1")
		}
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		first, err := svc.StartPracticeSession(context.Background(), "skill-1", 10)
		require.NoError(t, err)
		second, err := svc.StartPracticeSession(context.Background(), "skill-1", 10)
		require.NoError(t, err)

		assert.NotEqual(t, first.ID, second.ID)
	})

	t.Run("starting a practice session without a skill is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.StartPracticeSession(context.Background(), "", 10)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "skill_id")
	})

	t.Run("starting a practice session with a count above the maximum is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.StartPracticeSession(context.Background(), "skill-1", 51)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "count")
	})

	t.Run("a practice session shuffles exercise and option order per the injected shuffle", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		putWithSkill(exercises, "ex-1", "skill-1")
		putWithSkill(exercises, "ex-2", "skill-1")
		putWithSkill(exercises, "ex-3", "skill-1")
		svc := application.NewExerciseService(newFakeChallengeRepository(), exercises, newFakeContentNodeRepository(), seededSkillRepository(), seededConceptRepository(), newFakeDiagramRepository(), idSequence(), func() time.Time { return fixedCreatedAt }, reverseShuffle)

		session, err := svc.StartPracticeSession(context.Background(), "skill-1", 10)

		require.NoError(t, err)
		assert.Equal(t, "opt-2", session.Exercises[0].Options[0].ID)
	})
}
