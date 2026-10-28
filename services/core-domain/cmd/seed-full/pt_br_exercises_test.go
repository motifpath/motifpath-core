package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestTranslatedExercise(t *testing.T) {
	phrases := map[string]string{
		"What chord is this?": "Que acorde é este?",
		"Look closely.":       "Olhe com atenção.",
		"A major":             "A maior",
		"A minor":             "A menor",
	}
	label := func(s string) *string { return &s }
	seconds := 30
	audio := "https://example.test/a.mp3"
	diagram := &domain.DiagramRef{DiagramID: "a-chord"}

	t.Run("its title, every text of its prompt and every option label are in Portuguese; the rest is kept", func(t *testing.T) {
		en := domain.Exercise{
			ID: "en-1", Title: "What chord is this?", ExerciseType: domain.ExerciseTypeTextResponse,
			Prompt: domain.PromptDocument{Type: "doc", Content: []domain.PromptNode{
				{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{{Type: domain.PromptNodeTypeText, Text: "Look closely."}}},
				{Type: domain.PromptNodeTypeDiagram, Attrs: &domain.PromptNodeAttrs{DiagramRef: diagram}},
				{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{{Type: domain.PromptNodeTypeText, Text: "What chord is this?"}}},
			}},
			Skills: []domain.KnowledgeNode{{ID: "skill-1"}}, Concepts: []domain.KnowledgeNode{{ID: "concept-1"}},
			AudioURL: &audio, EstimatedDurationSeconds: &seconds, InstrumentIDs: []string{"guitar"},
			Options: []domain.Option{
				{ID: "o-1", IsCorrect: true, Label: label("A major")},
				{ID: "o-2", IsCorrect: false, Label: label("A minor")},
			},
			Languages: []domain.Language{{Code: "en"}},
		}

		pt, err := translatedExercise(en, phrases)

		require.NoError(t, err)
		assert.Equal(t, "Que acorde é este?", pt.Title)
		assert.Equal(t, "Olhe com atenção.", pt.Prompt.Content[0].Content[0].Text)
		assert.Equal(t, diagram, pt.Prompt.Content[1].Attrs.DiagramRef)
		assert.Equal(t, "Que acorde é este?", pt.Prompt.Content[2].Content[0].Text)
		require.Len(t, pt.Options, 2)
		assert.Equal(t, "A maior", *pt.Options[0].Label)
		assert.True(t, pt.Options[0].IsCorrect)
		assert.Equal(t, "A menor", *pt.Options[1].Label)
		assert.NotEqual(t, "o-1", pt.Options[0].ID, "a copy's options are its own")
		assert.NotEqual(t, pt.Options[0].ID, pt.Options[1].ID)
		assert.Equal(t, en.Skills, pt.Skills)
		assert.Equal(t, en.Concepts, pt.Concepts)
		assert.Equal(t, en.AudioURL, pt.AudioURL)
		assert.Equal(t, en.EstimatedDurationSeconds, pt.EstimatedDurationSeconds)
		assert.Equal(t, en.InstrumentIDs, pt.InstrumentIDs)
		assert.Equal(t, []domain.Language{{Code: "pt_BR"}}, pt.Languages)
		assert.Equal(t, "What chord is this?", en.Title, "the English exercise is left as it is")
		assert.Equal(t, "Look closely.", en.Prompt.Content[0].Content[0].Text)
		assert.Equal(t, "A major", *en.Options[0].Label)
	})

	t.Run("an exercise answered on a diagram leaves its options to be derived again from the diagram", func(t *testing.T) {
		en := domain.Exercise{
			Title: "What chord is this?", ExerciseType: domain.ExerciseTypeImageRecognition, DiagramRef: diagram,
			Options: []domain.Option{{ID: "derived", DiagramID: &diagram.DiagramID}},
		}

		pt, err := translatedExercise(en, phrases)

		require.NoError(t, err)
		assert.Equal(t, diagram, pt.DiagramRef)
		assert.Empty(t, pt.Options)
	})

	t.Run("an exercise whose sound is a diagram keeps its options, translated", func(t *testing.T) {
		en := domain.Exercise{
			Title: "What chord is this?", ExerciseType: domain.ExerciseTypeAudioRecognition, DiagramRef: diagram,
			Options: []domain.Option{{ID: "o-1", IsCorrect: true, Label: label("A major")}},
		}

		pt, err := translatedExercise(en, phrases)

		require.NoError(t, err)
		require.Len(t, pt.Options, 1)
		assert.Equal(t, "A maior", *pt.Options[0].Label)
	})

	t.Run("a phrase with no translation is refused, naming it", func(t *testing.T) {
		for name, en := range map[string]domain.Exercise{
			"title":  {Title: "Name this chord"},
			"prompt": {Title: "What chord is this?", Prompt: domain.NewPlainTextPrompt("Name this chord")},
			"label":  {Title: "What chord is this?", Options: []domain.Option{{ID: "o", Label: label("Name this chord")}}},
		} {
			_, err := translatedExercise(en, phrases)

			assert.ErrorContains(t, err, `"Name this chord"`, name)
		}
	})
}

// Every English phrase the seeded exercises use has a Portuguese one, so no
// seeded exercise is left out of a pt-BR student's practice.
func TestEverySeededExercisePhraseIsTranslated(t *testing.T) {
	var phrases []string
	for _, s := range exerciseTypeSpecs(testMedia) {
		phrases = append(phrases, s.title)
		for _, o := range s.options {
			if o.Label != nil {
				phrases = append(phrases, *o.Label)
			}
		}
	}
	for _, a := range articleSpecs(seededDiagrams{}, "chart") {
		for _, q := range a.challenge {
			phrases = append(phrases, q.title, q.right, q.wrong)
		}
	}
	for _, q := range videoBeginnerQuestions {
		phrases = append(phrases, q.title, q.right, q.wrong)
	}
	phrases = append(phrases, listeningTitle)

	for _, p := range phrases {
		assert.NotEmpty(t, ptBRPhrases[p], "no Portuguese for %q", p)
	}
}
