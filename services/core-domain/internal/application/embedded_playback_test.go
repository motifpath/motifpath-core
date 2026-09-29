package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// withEmbeddedDiagram is a document with a paragraph and, after it, the
// guitar diagram "diagram-1" embedded to play with voice (nil for none).
// The embed may be nested, as an editor nests it inside other blocks.
func withEmbeddedDiagram(voice *string, diagramID string) *domain.PromptDocument {
	ref := &domain.DiagramRef{DiagramID: diagramID, Layers: domain.DiagramLayers{Intervals: true}}
	if voice != nil {
		ref.Playback = &domain.DiagramPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, VoiceID: voice}
	}
	doc := richTextContent("Listen to this lick.")
	doc.Content = append(doc.Content, domain.PromptNode{Type: domain.PromptNodeTypeBulletList, Content: []domain.PromptNode{
		{Type: domain.PromptNodeTypeListItem, Content: []domain.PromptNode{
			{Type: domain.PromptNodeTypeDiagram, Attrs: &domain.PromptNodeAttrs{DiagramRef: ref}},
		}},
	}})
	return &doc
}

// TestEmbeddedDiagramPlaybackVoice covers every place a diagram can be
// embedded in a document or an option: a voice it plays with must exist and
// play its diagram's instrument family, as for a usage's own diagram_ref.
func TestEmbeddedDiagramPlaybackVoice(t *testing.T) {
	ctx := context.Background()
	type door struct {
		name  string
		field string
		save  func(t *testing.T, doc *domain.PromptDocument, option *domain.DiagramRef) error
	}
	contentService := func(t *testing.T) (*application.ContentService, *fakeContentNodeRepository, *fakeExpandedContentRepository) {
		t.Helper()
		nodes, expanded, diagrams := newFakeContentNodeRepository(), newFakeExpandedContentRepository(), newFakeDiagramRepository()
		seedContentDiagram(t, diagrams, "diagram-1", "guitar")
		return newContentServiceWithDiagrams(nodes, expanded, seededSkillRepository(), seededConceptRepository(), newFakeContentNodeVersionRepository(), diagrams), nodes, expanded
	}
	exerciseService := func(t *testing.T) (*application.ExerciseService, *fakeExerciseRepository) {
		t.Helper()
		exercises, diagrams := newFakeExerciseRepository(), newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", nil)
		return newExerciseServiceWithDiagrams(newFakeChallengeRepository(), exercises, newFakeContentNodeRepository(), diagrams), exercises
	}
	article := func(doc *domain.PromptDocument) application.ContentNodeInput {
		return application.ContentNodeInput{Title: "Blues phrasing", ContentType: domain.ContentTypeArticle, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"},
			Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, RichContent: doc}
	}
	imageChoice := func(ref *domain.DiagramRef) []domain.Option {
		if ref == nil {
			ref = &domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}}
		}
		return []domain.Option{{ID: "opt-1", IsCorrect: true, DiagramRef: ref}, {ID: "opt-2", ImageURL: strPtr("https://cdn.example.com/b.png")}}
	}

	doors := []door{
		{name: "an article's body", field: "rich_content", save: func(t *testing.T, doc *domain.PromptDocument, _ *domain.DiagramRef) error {
			svc, _, _ := contentService(t)
			_, err := svc.CreateContentNode(ctx, teacherCaller(), article(doc))
			return err
		}},
		{name: "an edited article's body", field: "rich_content", save: func(t *testing.T, doc *domain.PromptDocument, _ *domain.DiagramRef) error {
			svc, _, _ := contentService(t)
			node, err := svc.CreateContentNode(ctx, teacherCaller(), article(articleBody()))
			require.NoError(t, err)
			_, err = svc.UpdateContentNode(ctx, teacherCaller(), node.ID, article(doc))
			return err
		}},
		{name: "a rich-text pop-up", field: "rich_content", save: func(t *testing.T, doc *domain.PromptDocument, _ *domain.DiagramRef) error {
			svc, nodes, _ := contentService(t)
			nodes.put(articleNode("node-1"))
			_, err := svc.CreateExpandedContent(ctx, teacherCaller(), "node-1", domain.ExpandedContentTypeRichText, nil, doc, nil, nil, nil, nil, intPtr(3), intPtr(8000), nil)
			return err
		}},
		{name: "an edited rich-text pop-up", field: "rich_content", save: func(t *testing.T, doc *domain.PromptDocument, _ *domain.DiagramRef) error {
			svc, nodes, _ := contentService(t)
			nodes.put(articleNode("node-1"))
			item, err := svc.CreateExpandedContent(ctx, teacherCaller(), "node-1", domain.ExpandedContentTypeRichText, nil, articleBody(), nil, nil, nil, nil, intPtr(3), intPtr(8000), nil)
			require.NoError(t, err)
			_, err = svc.UpdateExpandedContent(ctx, teacherCaller(), item.ID, domain.ExpandedContentTypeRichText, nil, doc, nil, nil, nil, nil, intPtr(3), intPtr(8000), nil)
			return err
		}},
		{name: "an exercise's prompt", field: "prompt", save: func(t *testing.T, doc *domain.PromptDocument, _ *domain.DiagramRef) error {
			svc, _ := exerciseService(t)
			_, err := svc.CreateExercise(ctx, teacherCaller(), "Name the lick", *doc, domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"},
				strPtr("https://cdn.example.com/a.png"), nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"})
			return err
		}},
		{name: "an edited exercise's prompt", field: "prompt", save: func(t *testing.T, doc *domain.PromptDocument, _ *domain.DiagramRef) error {
			svc, _ := exerciseService(t)
			exercise, err := svc.CreateExercise(ctx, teacherCaller(), "Name the lick", domain.NewPlainTextPrompt("Which lick?"), domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"},
				strPtr("https://cdn.example.com/a.png"), nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"})
			require.NoError(t, err)
			_, err = svc.UpdateExercise(ctx, teacherCaller(), exercise.ID, "Name the lick", *doc, []string{"skill-1"}, []string{"concept-1"},
				strPtr("https://cdn.example.com/a.png"), nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"})
			return err
		}},
		{name: "an exercise's remediation", field: "remediation_targets", save: func(t *testing.T, doc *domain.PromptDocument, _ *domain.DiagramRef) error {
			svc, _ := exerciseService(t)
			_, err := svc.CreateExercise(ctx, teacherCaller(), "Name the lick", domain.NewPlainTextPrompt("Which lick?"), domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"},
				strPtr("https://cdn.example.com/a.png"), nil, nil, nil, imageRecognitionOptions(), nil, []domain.RemediationTarget{{RichContent: doc}}, []string{"en"})
			return err
		}},
		{name: "an exercise option's thumbnail", field: "options", save: func(t *testing.T, doc *domain.PromptDocument, option *domain.DiagramRef) error {
			svc, _ := exerciseService(t)
			_, err := svc.CreateExercise(ctx, teacherCaller(), "Pick the lick", domain.NewPlainTextPrompt("Which one is the lick?"), domain.ExerciseTypeImageChoice, []string{"skill-1"}, []string{"concept-1"},
				nil, nil, nil, nil, imageChoice(option), nil, nil, []string{"en"})
			return err
		}},
	}

	optionPlaying := func(voice *string, diagramID string) *domain.DiagramRef {
		embedded := withEmbeddedDiagram(voice, diagramID)
		return embedded.Content[1].Content[0].Content[0].Attrs.DiagramRef
	}
	for _, d := range doors {
		t.Run(d.name+" accepts a voice of its diagram's family", func(t *testing.T) {
			require.NoError(t, d.save(t, withEmbeddedDiagram(strPtr("acoustic-guitar"), "diagram-1"), optionPlaying(strPtr("acoustic-guitar"), "diagram-1")))
		})
		t.Run(d.name+" leaves an embed without a voice unchecked, as before", func(t *testing.T) {
			require.NoError(t, d.save(t, withEmbeddedDiagram(nil, "not-a-diagram"), optionPlaying(nil, "not-a-diagram")))
		})
		for _, bad := range []struct{ what, voice, diagram string }{
			{what: "a voice of another family", voice: "piano", diagram: "diagram-1"},
			{what: "a voice that does not exist", voice: "banjo", diagram: "diagram-1"},
			{what: "a voice on a diagram that does not exist", voice: "acoustic-guitar", diagram: "not-a-diagram"},
		} {
			t.Run(d.name+" rejects "+bad.what, func(t *testing.T) {
				err := d.save(t, withEmbeddedDiagram(&bad.voice, bad.diagram), optionPlaying(&bad.voice, bad.diagram))

				var valErr *domain.ValidationError
				require.ErrorAs(t, err, &valErr)
				assert.Equal(t, d.field, valErr.Fields[0].Field)
			})
		}
	}
}
