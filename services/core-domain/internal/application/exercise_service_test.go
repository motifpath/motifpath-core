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
	return application.NewExerciseService(challenges, exercises, nodes, seededKnowledgeNodeRepository(), diagrams, exerciseInstruments(), newFakeVoiceRepository(), exerciseUsers(), idSequence(), func() time.Time { return fixedCreatedAt }, noShuffle)
}

// exerciseUsers are the teachers whose display names name an exercise's
// creator.
func exerciseUsers() *fakeUserRepository {
	users := newFakeUserRepository()
	users.put(domain.User{ID: teacherCaller().ID, ClerkUserID: "clerk-teacher-1", DisplayName: "Bob Ferreira"})
	users.put(domain.User{ID: otherTeacherCaller().ID, ClerkUserID: "clerk-teacher-2", DisplayName: "Carol Souza"})
	return users
}

// exerciseInstruments is a 6-string fretted guitar and a keyboard piano, the
// two families a diagram stimulus derives its options differently for.
func exerciseInstruments() *fakeInstrumentRepository {
	six := 6
	instruments := newFakeInstrumentRepository()
	instruments.byID["guitar"] = domain.Instrument{ID: "guitar", Names: domain.LocalizedText{"en": "Guitar"}, Family: domain.InstrumentFamilyFretted, StringCount: &six}
	instruments.byID["piano"] = domain.Instrument{ID: "piano", Names: domain.LocalizedText{"en": "Piano"}, Family: domain.InstrumentFamilyKeyboard}
	return instruments
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
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, &imageURL, nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		assert.Equal(t, "Root position of a C major triad", exercise.Title)
		assert.Empty(t, exercise.ChallengeIDs)
	})

	t.Run("a created exercise records its creator", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Chord name", domain.NewPlainTextPrompt("Name this chord"),
			domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		assert.Equal(t, teacherCaller().ID, exercise.CreatedBy)
	})

	t.Run("an admin creates an exercise", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), adminCaller(),
			"Name the interval", domain.NewPlainTextPrompt("Name the interval between the open low E and the 5th fret"),
			domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
	})

	t.Run("a teacher creates an exercise with skills and concepts", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		imageURL := "https://cdn.example.com/fretboard/descending-run.png"

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Alternate picking — descending run", domain.NewPlainTextPrompt("Play the descending run cleanly"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1", "skill-2"}, []string{"concept-1"}, &imageURL, nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"skill-1", "skill-2"}, exerciseSkillIDs(exercise))
	})

	t.Run("a teacher creates an exercise with a richly formatted prompt", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		richPrompt := richlyFormattedPrompt()

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Circle of fifths", richPrompt,
			domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

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
			"Circle of fifths", prompt, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		assert.Equal(t, prompt, exercise.Prompt)
	})

	t.Run("a teacher creates an exercise with a plain, unformatted prompt", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the note", domain.NewPlainTextPrompt("What note is this?"),
			domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		assert.Equal(t, domain.NewPlainTextPrompt("What note is this?"), exercise.Prompt)
	})

	t.Run("creating an exercise with an unstructured prompt is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.PromptDocument{Type: "not-a-doc"}, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

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
			"title", prompt, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

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
				"title", prompt, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

			var valErr *domain.ValidationError
			require.True(t, errors.As(err, &valErr))
			assertHasField(t, valErr, "prompt")
		})
	}

	t.Run("creating an exercise without a title is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "title")
	})

	t.Run("creating an exercise without a prompt is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.PromptDocument{}, domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "prompt")
	})

	t.Run("creating an exercise without an exercise type is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), "", []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "exercise_type")
	})

	t.Run("creating an exercise with an unrecognised type is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseType("multiple_choice"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "exercise_type")
	})

	t.Run("creating an exercise with zero correct options is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		label := "A major"

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil,
			[]domain.Option{{ID: "opt-1", IsCorrect: false, Label: &label}}, nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "options")
	})

	t.Run("creating an exercise with no skills is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, nil, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "skill_ids")
	})

	t.Run("creating an exercise with no concepts is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, nil, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "concept_ids")
	})

	t.Run("creating an exercise with a skill id that does not exist is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"missing-skill"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "skill_ids")
	})

	t.Run("creating an exercise with a concept given as a skill is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"concept-2"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "skill_ids")
	})

	t.Run("a student cannot create an exercise", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), studentCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

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
	diagram := domain.Diagram{ID: id, InstrumentID: instrumentID, Names: domain.LocalizedText{"en": id}, Positions: positions, Skills: []domain.KnowledgeNode{{ID: "skill-1"}}, Concepts: []domain.KnowledgeNode{{ID: "concept-1"}}}
	require.NoError(t, diagrams.Create(context.Background(), diagram))
	return diagram
}

func TestExerciseService_DiagramDrivenImageRecognition(t *testing.T) {
	rootPos := domain.Position{ID: "pos-root", Interval: "R", NoteName: "A"}
	thirdPos := domain.Position{ID: "pos-third", Interval: "3", NoteName: "C#"}

	fretted := func(id, interval string, str, fret int) domain.Position {
		return domain.Position{ID: id, Interval: interval, NoteName: "A", String: &str, Fret: &fret}
	}
	// A minor pentatonic fragment: frets 5–8, so its answer window is frets 5–9.
	pentatonic := []domain.Position{fretted("pos-6-5", "R", 6, 5), fretted("pos-4-7", "R", 4, 7), fretted("pos-6-8", "b3", 6, 8)}
	create := func(t *testing.T, positions []domain.Position, ref domain.DiagramRef) (domain.Exercise, error) {
		t.Helper()
		diagrams := newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", positions)
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)
		ref.DiagramID = "diagram-1"
		return svc.CreateExercise(context.Background(), teacherCaller(),
			"Find the roots", domain.NewPlainTextPrompt("Tap every root"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil,
			&ref, nil, nil, nil, nil, []string{"en"}, nil)
	}
	correctCells := func(exercise domain.Exercise) []domain.FretCell {
		var cells []domain.FretCell
		for _, opt := range exercise.Options {
			if opt.IsCorrect {
				cells = append(cells, *opt.FretCell)
			}
		}
		return cells
	}

	t.Run("a fretted diagram_ref derives one option per answer cell, correct only at the marked positions", func(t *testing.T) {
		exercise, err := create(t, pentatonic, domain.DiagramRef{CorrectPositionIDs: &[]string{"pos-6-5", "pos-4-7"}})

		require.NoError(t, err)
		require.Len(t, exercise.Options, 30)
		assert.ElementsMatch(t, []domain.FretCell{{String: 6, Fret: 5}, {String: 4, Fret: 7}}, correctCells(exercise))
		for _, opt := range exercise.Options {
			require.NotNil(t, opt.DiagramID)
			assert.Equal(t, "diagram-1", *opt.DiagramID)
			require.NotNil(t, opt.FretCell)
			assert.NotEqual(t, 0, opt.FretCell.Fret, "the window doesn't reach the nut")
			if *opt.FretCell == (domain.FretCell{String: 6, Fret: 8}) {
				require.NotNil(t, opt.DiagramPositionID)
				assert.Equal(t, "pos-6-8", *opt.DiagramPositionID)
				assert.False(t, opt.IsCorrect)
			}
			if *opt.FretCell == (domain.FretCell{String: 1, Fret: 5}) {
				assert.Nil(t, opt.DiagramPositionID, "an empty cell names no position")
			}
		}
	})

	t.Run("a stimulus that plays the hidden notes with a fretted voice is accepted", func(t *testing.T) {
		exercise, err := create(t, pentatonic, domain.DiagramRef{
			Layers:             domain.DiagramLayers{HiddenPositionIDs: &[]string{"pos-6-5"}},
			CorrectPositionIDs: &[]string{"pos-6-5"},
			Playback:           &domain.DiagramPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, VoiceID: strPtr("acoustic-guitar")},
		})

		require.NoError(t, err)
		require.NotNil(t, exercise.DiagramRef.Playback)
		assert.Equal(t, "acoustic-guitar", *exercise.DiagramRef.Playback.VoiceID)
	})

	t.Run("a stimulus that plays with a keyboard voice is rejected as diagram_ref", func(t *testing.T) {
		_, err := create(t, pentatonic, domain.DiagramRef{
			CorrectPositionIDs: &[]string{"pos-6-5"},
			Playback:           &domain.DiagramPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, VoiceID: strPtr("piano")},
		})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "diagram_ref", valErr.Fields[0].Field)
	})

	t.Run("a hidden position is still a cell, and still correct when marked", func(t *testing.T) {
		exercise, err := create(t, pentatonic, domain.DiagramRef{
			Layers:             domain.DiagramLayers{HiddenPositionIDs: &[]string{"pos-6-5"}},
			CorrectPositionIDs: &[]string{"pos-6-5"},
		})

		require.NoError(t, err)
		require.Len(t, exercise.Options, 30)
		assert.Equal(t, []domain.FretCell{{String: 6, Fret: 5}}, correctCells(exercise))
	})

	t.Run("open strings are cells when the window reaches the nut", func(t *testing.T) {
		exercise, err := create(t, []domain.Position{fretted("pos-6-0", "R", 6, 0), fretted("pos-5-2", "5", 5, 2)},
			domain.DiagramRef{CorrectPositionIDs: &[]string{"pos-6-0"}})

		require.NoError(t, err)
		require.Len(t, exercise.Options, 24)
		assert.Equal(t, []domain.FretCell{{String: 6, Fret: 0}}, correctCells(exercise))
	})

	t.Run("older correct intervals become the matching drawn positions, stored as correct positions", func(t *testing.T) {
		exercise, err := create(t, pentatonic, domain.DiagramRef{
			Layers:           domain.DiagramLayers{HiddenPositionIDs: &[]string{"pos-4-7"}},
			CorrectIntervals: &[]string{"R"},
		})

		require.NoError(t, err)
		assert.Equal(t, []domain.FretCell{{String: 6, Fret: 5}}, correctCells(exercise), "a hidden root isn't drawn, so wasn't correct before")
		require.NotNil(t, exercise.DiagramRef.CorrectPositionIDs)
		assert.Equal(t, []string{"pos-6-5"}, *exercise.DiagramRef.CorrectPositionIDs)
		assert.Nil(t, exercise.DiagramRef.CorrectIntervals)
	})

	t.Run("a correct position that isn't the diagram's is rejected", func(t *testing.T) {
		_, err := create(t, pentatonic, domain.DiagramRef{CorrectPositionIDs: &[]string{"pos-elsewhere"}})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "diagram_ref")
	})

	t.Run("a keyboard diagram keeps one option per position", func(t *testing.T) {
		diagrams := newFakeDiagramRepository()
		key := "A3"
		seedDiagram(t, diagrams, "diagram-1", "piano", []domain.Position{{ID: "pos-a3", Interval: "R", NoteName: "A", Key: &key}})
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)

		exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Find the root", domain.NewPlainTextPrompt("Tap the root"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil,
			&domain.DiagramRef{DiagramID: "diagram-1", CorrectPositionIDs: &[]string{"pos-a3"}}, nil,
			nil, nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		require.Len(t, exercise.Options, 1)
		assert.True(t, exercise.Options[0].IsCorrect)
		assert.Nil(t, exercise.Options[0].FretCell)
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
			nil, nil, nil, []string{"en"}, nil)

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
			nil, nil, nil, []string{"en"}, nil)

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
			nil, nil, nil, []string{"en"}, nil)

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
			nil, nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "diagram_stack_ref")
	})

	t.Run("saving a diagram exercise again", func(t *testing.T) {
		// A saved exercise's options are what students' answers name, so a save that
		// leaves a choice in place must keep its option id.
		setup := func(t *testing.T, instrumentID string, positions []domain.Position, correct []string) (*application.ExerciseService, domain.Exercise) {
			t.Helper()
			diagrams := newFakeDiagramRepository()
			seedDiagram(t, diagrams, "diagram-1", instrumentID, positions)
			seedDiagram(t, diagrams, "diagram-2", instrumentID, positions)
			svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)
			exercise, err := svc.CreateExercise(context.Background(), teacherCaller(),
				"Find the roots", domain.NewPlainTextPrompt("Tap every root"),
				domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil,
				&domain.DiagramRef{DiagramID: "diagram-1", CorrectPositionIDs: &correct}, nil, nil, nil, nil, []string{"en"}, nil)
			require.NoError(t, err)
			return svc, exercise
		}
		update := func(t *testing.T, svc *application.ExerciseService, id, diagramID string, correct []string) domain.Exercise {
			t.Helper()
			updated, err := svc.UpdateExercise(context.Background(), teacherCaller(), id,
				"Find every root", domain.NewPlainTextPrompt("Tap every root"),
				[]string{"skill-1"}, []string{"concept-1"}, nil, nil,
				&domain.DiagramRef{DiagramID: diagramID, CorrectPositionIDs: &correct}, nil, nil, nil, nil, []string{"en"}, nil)
			require.NoError(t, err)
			return updated
		}
		idsByCell := func(exercise domain.Exercise) map[domain.FretCell]string {
			ids := map[domain.FretCell]string{}
			for _, opt := range exercise.Options {
				ids[*opt.FretCell] = opt.ID
			}
			return ids
		}

		t.Run("keeps every cell's option id", func(t *testing.T) {
			svc, created := setup(t, "guitar", pentatonic, []string{"pos-6-5"})

			updated := update(t, svc, created.ID, "diagram-1", []string{"pos-6-5"})

			assert.Equal(t, idsByCell(created), idsByCell(updated))
		})

		t.Run("with other answers, keeps the ids and moves only which cells are correct", func(t *testing.T) {
			svc, created := setup(t, "guitar", pentatonic, []string{"pos-6-5"})

			updated := update(t, svc, created.ID, "diagram-1", []string{"pos-4-7"})

			assert.Equal(t, idsByCell(created), idsByCell(updated))
			assert.Equal(t, []domain.FretCell{{String: 4, Fret: 7}}, correctCells(updated))
		})

		t.Run("keeps a keyboard position's option id", func(t *testing.T) {
			key := "A3"
			svc, created := setup(t, "piano", []domain.Position{{ID: "pos-a3", Interval: "R", NoteName: "A", Key: &key}}, []string{"pos-a3"})

			updated := update(t, svc, created.ID, "diagram-1", []string{"pos-a3"})

			require.Len(t, updated.Options, 1)
			assert.Equal(t, created.Options[0].ID, updated.Options[0].ID)
		})

		t.Run("with another diagram, gives its options new ids", func(t *testing.T) {
			svc, created := setup(t, "guitar", pentatonic, []string{"pos-6-5"})

			updated := update(t, svc, created.ID, "diagram-2", []string{"pos-6-5"})

			for cell, id := range idsByCell(updated) {
				assert.NotEqual(t, idsByCell(created)[cell], id)
			}
		})
	})

	t.Run("supplying options alongside a diagram_ref is rejected", func(t *testing.T) {
		diagrams := newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", []domain.Position{rootPos})
		svc := newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams)

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"Name the root", domain.NewPlainTextPrompt("Which position is the root?"),
			domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"}, nil, nil,
			&domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, CorrectIntervals: &[]string{"R"}}, nil,
			imageRecognitionOptions(), nil, nil, []string{"en"}, nil)

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
			[]domain.RemediationTarget{{ContentNodeID: &nodeID}}, []string{"en"}, nil)

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
			[]domain.RemediationTarget{{RichContent: &rich, Caption: &caption}}, []string{"en"}, nil)

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
			[]domain.RemediationTarget{{ContentNodeID: &nodeID}, {RichContent: &rich}}, []string{"en"}, nil)

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
			[]domain.RemediationTarget{{ContentNodeID: &newTarget}}, []string{"en"}, nil)

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
			"t", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

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
			[]domain.RemediationTarget{{ContentNodeID: &nodeID, RichContent: &rich}}, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "remediation_targets")
	})

	t.Run("creating an exercise with a remediation target carrying neither a content node nor rich content is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil,
			[]domain.RemediationTarget{{}}, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "remediation_targets")
	})

	t.Run("creating an exercise with a remediation target referencing a non-existent content node is rejected", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())
		missing := "missing"

		_, err := svc.CreateExercise(context.Background(), teacherCaller(),
			"title", domain.NewPlainTextPrompt("prompt"), domain.ExerciseTypeTextResponse, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil,
			[]domain.RemediationTarget{{ContentNodeID: &missing}}, []string{"en"}, nil)

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
	firstPage := domain.PageRequest{Limit: 20, Offset: 0}

	t.Run("a teacher lists all exercises in the reusable pool", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeImageRecognition})
		exercises.put(domain.Exercise{ID: "picking-drill-01", ExerciseType: domain.ExerciseTypeTextResponse})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExercises(context.Background(), teacherCaller(), domain.ExerciseFilter{}, firstPage)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"triad-exercise-01", "picking-drill-01"}, idsOf(got.Items))
		assert.Equal(t, 2, got.Total)
	})

	t.Run("an admin lists all exercises in the reusable pool", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01"})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExercises(context.Background(), adminCaller(), domain.ExerciseFilter{}, firstPage)

		require.NoError(t, err)
		assert.Len(t, got.Items, 1)
	})

	t.Run("a teacher filters the exercise list by skill", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", Skills: []domain.KnowledgeNode{{ID: "skill-1"}}})
		exercises.put(domain.Exercise{ID: "picking-drill-01", Skills: []domain.KnowledgeNode{{ID: "skill-2"}}})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExercises(context.Background(), teacherCaller(), domain.ExerciseFilter{SkillID: "skill-2"}, firstPage)

		require.NoError(t, err)
		assert.Equal(t, []string{"picking-drill-01"}, idsOf(got.Items))
	})

	t.Run("a teacher filters the exercise list by exercise type", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeImageRecognition})
		exercises.put(domain.Exercise{ID: "chord-name-01", ExerciseType: domain.ExerciseTypeTextResponse})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExercises(context.Background(), teacherCaller(), domain.ExerciseFilter{ExerciseType: domain.ExerciseTypeTextResponse}, firstPage)

		require.NoError(t, err)
		assert.Equal(t, []string{"chord-name-01"}, idsOf(got.Items))
	})

	t.Run("the new filters each narrow the pool and combine with AND", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "bobs-triad", Title: "Major Triad Shapes", Concepts: []domain.KnowledgeNode{{ID: "concept-1"}}, Languages: []domain.Language{{Code: "en"}}, CreatedBy: "teacher-1"})
		exercises.put(domain.Exercise{ID: "carols-triad", Title: "Triad inversions", Concepts: []domain.KnowledgeNode{{ID: "concept-2"}}, Languages: []domain.Language{{Code: "pt"}}, CreatedBy: "teacher-2"})
		exercises.put(domain.Exercise{ID: "carols-picking", Title: "Picking drill", Concepts: []domain.KnowledgeNode{{ID: "concept-2"}}, Languages: []domain.Language{{Code: "pt"}}, CreatedBy: "teacher-2"})
		exercises.put(domain.Exercise{ID: "legacy-triad", Title: "Triad legacy", Languages: []domain.Language{{Code: "en"}}})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		tests := []struct {
			name   string
			filter domain.ExerciseFilter
			want   []string
		}{
			{name: "title search ignores case", filter: domain.ExerciseFilter{Query: "TRIAD"}, want: []string{"bobs-triad", "carols-triad", "legacy-triad"}},
			{name: "by concept", filter: domain.ExerciseFilter{ConceptID: "concept-2"}, want: []string{"carols-picking", "carols-triad"}},
			{name: "by language", filter: domain.ExerciseFilter{Language: "pt"}, want: []string{"carols-picking", "carols-triad"}},
			{name: "by creator, never matching an exercise with no recorded creator", filter: domain.ExerciseFilter{CreatedBy: "teacher-1"}, want: []string{"bobs-triad"}},
			{name: "filters combine", filter: domain.ExerciseFilter{Query: "triad", CreatedBy: "teacher-2"}, want: []string{"carols-triad"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := svc.ListExercises(context.Background(), teacherCaller(), tt.filter, firstPage)

				require.NoError(t, err)
				assert.Equal(t, tt.want, idsOf(got.Items))
			})
		}
	})

	t.Run("returns the requested page and the filtered total", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		for _, id := range []string{"ex-1", "ex-2", "ex-3", "ex-4", "ex-5"} {
			exercises.put(domain.Exercise{ID: id})
		}
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExercises(context.Background(), teacherCaller(), domain.ExerciseFilter{}, domain.PageRequest{Limit: 2, Offset: 4})

		require.NoError(t, err)
		assert.Equal(t, 5, got.Total)
		assert.Equal(t, []string{"ex-5"}, idsOf(got.Items))
	})

	t.Run("listing exercises when none exist returns an empty page", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		got, err := svc.ListExercises(context.Background(), teacherCaller(), domain.ExerciseFilter{}, firstPage)

		require.NoError(t, err)
		assert.Empty(t, got.Items)
		assert.Zero(t, got.Total)
	})

	t.Run("a student cannot list all exercises", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.ListExercises(context.Background(), studentCaller(), domain.ExerciseFilter{}, firstPage)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_ListExerciseCreators(t *testing.T) {
	ctx := context.Background()
	bob := application.Creator{UserID: teacherCaller().ID, DisplayName: "Bob Ferreira"}
	carol := application.Creator{UserID: otherTeacherCaller().ID, DisplayName: "Carol Souza"}
	pool := func() *fakeExerciseRepository {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "bobs-drill", CreatedBy: bob.UserID})
		exercises.put(domain.Exercise{ID: "bobs-quiz", CreatedBy: bob.UserID})
		exercises.put(domain.Exercise{ID: "carols-drill", CreatedBy: carol.UserID})
		exercises.put(domain.Exercise{ID: "legacy-drill"})
		return exercises
	}

	tests := []struct {
		name   string
		caller domain.User
		query  string
		want   []application.Creator
	}{
		{name: "a teacher gets every creator, each once, in name order", caller: teacherCaller(), want: []application.Creator{bob, carol}},
		{name: "an admin gets every creator", caller: adminCaller(), want: []application.Creator{bob, carol}},
		{name: "the query keeps creators whose name contains it, ignoring case and accents", caller: adminCaller(), query: "FÉRR", want: []application.Creator{bob}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newExerciseService(newFakeChallengeRepository(), pool())

			got, err := svc.ListExerciseCreators(ctx, tt.caller, tt.query)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("a pool with no recorded creator lists nobody", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "legacy-drill"})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		got, err := svc.ListExerciseCreators(ctx, teacherCaller(), "")

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a student cannot list exercise creators", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), pool())

		_, err := svc.ListExerciseCreators(ctx, studentCaller(), "")

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestExerciseService_UpdateExercise(t *testing.T) {
	t.Run("a teacher updates an exercise's title, prompt, and options", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "old title", Prompt: domain.NewPlainTextPrompt("old prompt"), Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		revisedPrompt := domain.NewPlainTextPrompt("Identify the root position, now with a cleaner prompt")
		exercise, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"Root position, revised", revisedPrompt, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

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
			"t", formatted, []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		assert.Equal(t, formatted, exercise.Prompt)
	})

	t.Run("a teacher replaces an exercise's skills", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "picking-drill-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("p"), Skills: []domain.KnowledgeNode{{ID: "skill-1"}}, Concepts: []domain.KnowledgeNode{{ID: "concept-1"}}, Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		exercise, err := svc.UpdateExercise(context.Background(), teacherCaller(), "picking-drill-01",
			"t", domain.NewPlainTextPrompt("p"), []string{"skill-1", "skill-2"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"skill-1", "skill-2"}, exerciseSkillIDs(exercise))
	})

	t.Run("updating an exercise does not change its links to challenges or content nodes", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("p"), ChallengeIDs: []string{"triad-challenge"}, Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		exercise, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"Root position, revised", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		require.NoError(t, err)
		assert.Equal(t, []string{"triad-challenge"}, exercise.ChallengeIDs)
	})

	t.Run("updating an exercise without a title is rejected", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("p"), Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		_, err := svc.UpdateExercise(context.Background(), teacherCaller(), "triad-exercise-01",
			"", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

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
			"t", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, []domain.Option{{ID: "opt-1", IsCorrect: false, Label: &label}}, nil, nil, []string{"en"}, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "options")
	})

	t.Run("updating an exercise that does not exist returns not found", func(t *testing.T) {
		svc := newExerciseService(newFakeChallengeRepository(), newFakeExerciseRepository())

		_, err := svc.UpdateExercise(context.Background(), teacherCaller(), "missing",
			"t", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot update an exercise", func(t *testing.T) {
		exercises := newFakeExerciseRepository()
		exercises.put(domain.Exercise{ID: "triad-exercise-01", ExerciseType: domain.ExerciseTypeTextResponse, Title: "t", Prompt: domain.NewPlainTextPrompt("p"), Options: textResponseOptions()})
		svc := newExerciseService(newFakeChallengeRepository(), exercises)

		_, err := svc.UpdateExercise(context.Background(), studentCaller(), "triad-exercise-01",
			"Hijacked title", domain.NewPlainTextPrompt("p"), []string{"skill-1"}, []string{"concept-1"}, nil, nil, nil, nil, textResponseOptions(), nil, nil, []string{"en"}, nil)

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
		svc := application.NewExerciseService(challenges, exercises, newFakeContentNodeRepository(), seededKnowledgeNodeRepository(), newFakeDiagramRepository(), exerciseInstruments(), newFakeVoiceRepository(), exerciseUsers(), idSequence(), func() time.Time { return fixedCreatedAt }, reverseShuffle)

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
		svc := application.NewExerciseService(challenges, exercises, newFakeContentNodeRepository(), seededKnowledgeNodeRepository(), newFakeDiagramRepository(), exerciseInstruments(), newFakeVoiceRepository(), exerciseUsers(), idSequence(), func() time.Time { return fixedCreatedAt }, reverseShuffle)

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
		svc := application.NewExerciseService(newFakeChallengeRepository(), exercises, nodes, seededKnowledgeNodeRepository(), newFakeDiagramRepository(), exerciseInstruments(), newFakeVoiceRepository(), exerciseUsers(), idSequence(), func() time.Time { return fixedCreatedAt }, reverseShuffle)

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
		exercises.put(domain.Exercise{ID: id, Skills: []domain.KnowledgeNode{{ID: skillID}}, Options: textResponseOptions()})
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
		svc := application.NewExerciseService(newFakeChallengeRepository(), exercises, newFakeContentNodeRepository(), seededKnowledgeNodeRepository(), newFakeDiagramRepository(), exerciseInstruments(), newFakeVoiceRepository(), exerciseUsers(), idSequence(), func() time.Time { return fixedCreatedAt }, reverseShuffle)

		session, err := svc.StartPracticeSession(context.Background(), "skill-1", 10)

		require.NoError(t, err)
		assert.Equal(t, "opt-2", session.Exercises[0].Options[0].ID)
	})
}
