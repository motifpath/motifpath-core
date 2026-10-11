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
	var playback *domain.DiagramRefPlayback
	if voice != nil {
		playback = &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, VoiceID: voice}
	}
	return withEmbeddedPlayback(playback, diagramID)
}

// withEmbeddedPlayback is withEmbeddedDiagram with the embed playing as
// playback says (nil for no playback config).
func withEmbeddedPlayback(playback *domain.DiagramRefPlayback, diagramID string) *domain.PromptDocument {
	ref := &domain.DiagramRef{DiagramID: diagramID, Layers: domain.DiagramLayers{Intervals: true}, Playback: playback}
	doc := richTextContent("Listen to this lick.")
	doc.Content = append(doc.Content, domain.PromptNode{Type: domain.PromptNodeTypeBulletList, Content: []domain.PromptNode{
		{Type: domain.PromptNodeTypeListItem, Content: []domain.PromptNode{
			{Type: domain.PromptNodeTypeDiagram, Attrs: &domain.PromptNodeAttrs{DiagramRef: ref}},
		}},
	}})
	return &doc
}

// givePlaybacks gives the seeded diagram id the playbacks ids, the first
// the default.
func givePlaybacks(t *testing.T, diagrams *fakeDiagramRepository, id string, ids ...string) {
	t.Helper()
	d, err := diagrams.GetByID(context.Background(), id)
	require.NoError(t, err)
	d.Playbacks = make([]domain.DiagramPlayback, len(ids))
	for i, playbackID := range ids {
		d.Playbacks[i] = domain.DiagramPlayback{ID: playbackID, TempoBPM: 90}
	}
	d.DefaultPlaybackID = &d.Playbacks[0].ID
	require.NoError(t, diagrams.Update(context.Background(), d))
}

// TestEmbeddedDiagramPlayback covers every place a diagram can be embedded
// in a document or an option: a voice it plays with must exist and play its
// diagram's instrument family, and a playback it chooses must be one of its
// diagram's, as for a usage's own diagram_ref.
func TestEmbeddedDiagramPlayback(t *testing.T) {
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
		givePlaybacks(t, diagrams, "diagram-1", "pb-strum", "pb-arp")
		return newContentServiceWithDiagrams(nodes, expanded, seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams), nodes, expanded
	}
	exerciseService := func(t *testing.T) (*application.ExerciseService, *fakeExerciseRepository) {
		t.Helper()
		exercises, diagrams := newFakeExerciseRepository(), newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", nil)
		givePlaybacks(t, diagrams, "diagram-1", "pb-strum", "pb-arp")
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
				strPtr("https://cdn.example.com/a.png"), nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"}, nil)
			return err
		}},
		{name: "an edited exercise's prompt", field: "prompt", save: func(t *testing.T, doc *domain.PromptDocument, _ *domain.DiagramRef) error {
			svc, _ := exerciseService(t)
			exercise, err := svc.CreateExercise(ctx, teacherCaller(), "Name the lick", domain.NewPlainTextPrompt("Which lick?"), domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"},
				strPtr("https://cdn.example.com/a.png"), nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"}, nil)
			require.NoError(t, err)
			_, err = svc.UpdateExercise(ctx, teacherCaller(), exercise.ID, "Name the lick", *doc, []string{"skill-1"}, []string{"concept-1"},
				strPtr("https://cdn.example.com/a.png"), nil, nil, nil, imageRecognitionOptions(), nil, nil, []string{"en"}, nil)
			return err
		}},
		{name: "an exercise's remediation", field: "remediation_targets", save: func(t *testing.T, doc *domain.PromptDocument, _ *domain.DiagramRef) error {
			svc, _ := exerciseService(t)
			_, err := svc.CreateExercise(ctx, teacherCaller(), "Name the lick", domain.NewPlainTextPrompt("Which lick?"), domain.ExerciseTypeImageRecognition, []string{"skill-1"}, []string{"concept-1"},
				strPtr("https://cdn.example.com/a.png"), nil, nil, nil, imageRecognitionOptions(), nil, []domain.RemediationTarget{{RichContent: doc}}, []string{"en"}, nil)
			return err
		}},
		{name: "an exercise option's thumbnail", field: "options", save: func(t *testing.T, doc *domain.PromptDocument, option *domain.DiagramRef) error {
			svc, _ := exerciseService(t)
			_, err := svc.CreateExercise(ctx, teacherCaller(), "Pick the lick", domain.NewPlainTextPrompt("Which one is the lick?"), domain.ExerciseTypeImageChoice, []string{"skill-1"}, []string{"concept-1"},
				nil, nil, nil, nil, imageChoice(option), nil, nil, []string{"en"}, nil)
			return err
		}},
	}

	optionPlaying := func(voice *string, diagramID string) *domain.DiagramRef {
		embedded := withEmbeddedDiagram(voice, diagramID)
		return embedded.Content[1].Content[0].Content[0].Attrs.DiagramRef
	}
	choosing := func(playbackID string) *domain.DiagramRefPlayback {
		return &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, PlaybackID: &playbackID}
	}
	optionChoosing := func(playbackID, diagramID string) *domain.DiagramRef {
		embedded := withEmbeddedPlayback(choosing(playbackID), diagramID)
		return embedded.Content[1].Content[0].Content[0].Attrs.DiagramRef
	}
	for _, d := range doors {
		t.Run(d.name+" accepts a voice of its diagram's family", func(t *testing.T) {
			require.NoError(t, d.save(t, withEmbeddedDiagram(strPtr("acoustic-guitar"), "diagram-1"), optionPlaying(strPtr("acoustic-guitar"), "diagram-1")))
		})
		t.Run(d.name+" leaves an embed without a voice unchecked, as before", func(t *testing.T) {
			require.NoError(t, d.save(t, withEmbeddedDiagram(nil, "not-a-diagram"), optionPlaying(nil, "not-a-diagram")))
		})
		t.Run(d.name+" accepts a playback its diagram has", func(t *testing.T) {
			require.NoError(t, d.save(t, withEmbeddedPlayback(choosing("pb-arp"), "diagram-1"), optionChoosing("pb-arp", "diagram-1")))
		})
		for _, bad := range []struct{ what, playback, diagram string }{
			{what: "a playback its diagram doesn't have", playback: "pb-gone", diagram: "diagram-1"},
			{what: "a playback on a diagram that does not exist", playback: "pb-arp", diagram: "not-a-diagram"},
		} {
			t.Run(d.name+" rejects "+bad.what, func(t *testing.T) {
				err := d.save(t, withEmbeddedPlayback(choosing(bad.playback), bad.diagram), optionChoosing(bad.playback, bad.diagram))

				var valErr *domain.ValidationError
				require.ErrorAs(t, err, &valErr)
				assert.Equal(t, d.field, valErr.Fields[0].Field)
			})
		}
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

// TestPlaybackRemovedFromItsDiagram covers every place a usage can choose a
// diagram's playback: once that playback is removed from the diagram, the
// usage keeps its id and plays the default, so saving the usage again with
// that same id must still succeed. Choosing a removed playback anew is still
// rejected.
func TestPlaybackRemovedFromItsDiagram(t *testing.T) {
	ctx := context.Background()
	// A door saves a usage of diagram-1 choosing playback, returning the
	// diagrams it plays from and a way to save the usage again choosing
	// another playback (or the same one).
	type door struct {
		name  string
		field string
		save  func(t *testing.T, playback string) (diagrams *fakeDiagramRepository, resave func(playback string) error)
	}
	choosing := func(playbackID string) *domain.DiagramRefPlayback {
		return &domain.DiagramRefPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored, PlaybackID: &playbackID}
	}
	refChoosing := func(playbackID string) *domain.DiagramRef {
		return &domain.DiagramRef{DiagramID: "diagram-1", Layers: domain.DiagramLayers{Intervals: true}, Playback: choosing(playbackID)}
	}
	contentService := func(t *testing.T) (*application.ContentService, *fakeContentNodeRepository, *fakeDiagramRepository) {
		t.Helper()
		nodes, diagrams := newFakeContentNodeRepository(), newFakeDiagramRepository()
		seedContentDiagram(t, diagrams, "diagram-1", "guitar")
		givePlaybacks(t, diagrams, "diagram-1", "pb-strum", "pb-arp")
		return newContentServiceWithDiagrams(nodes, newFakeExpandedContentRepository(), seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), diagrams), nodes, diagrams
	}
	exerciseService := func(t *testing.T, positions []domain.Position) (*application.ExerciseService, *fakeDiagramRepository) {
		t.Helper()
		diagrams := newFakeDiagramRepository()
		seedDiagram(t, diagrams, "diagram-1", "guitar", positions)
		givePlaybacks(t, diagrams, "diagram-1", "pb-strum", "pb-arp")
		return newExerciseServiceWithDiagrams(newFakeChallengeRepository(), newFakeExerciseRepository(), newFakeContentNodeRepository(), diagrams), diagrams
	}
	article := func(doc *domain.PromptDocument) application.ContentNodeInput {
		return application.ContentNodeInput{Title: "Blues phrasing", ContentType: domain.ContentTypeArticle, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"},
			Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, RichContent: doc}
	}
	// exercise saves an exercise of exerciseType, created then updated with
	// the parts usage gives it for a playback choice.
	type exerciseParts struct {
		exerciseType domain.ExerciseType
		prompt       domain.PromptDocument
		imageURL     *string
		diagramRef   *domain.DiagramRef
		options      []domain.Option
		remediation  []domain.RemediationTarget
	}
	exercise := func(positions []domain.Position, usage func(playback string) exerciseParts) func(t *testing.T, playback string) (*fakeDiagramRepository, func(string) error) {
		return func(t *testing.T, playback string) (*fakeDiagramRepository, func(string) error) {
			svc, diagrams := exerciseService(t, positions)
			p := usage(playback)
			created, err := svc.CreateExercise(ctx, teacherCaller(), "Name the lick", p.prompt, p.exerciseType, []string{"skill-1"}, []string{"concept-1"},
				p.imageURL, nil, p.diagramRef, nil, p.options, nil, p.remediation, []string{"en"}, nil)
			require.NoError(t, err)
			return diagrams, func(playback string) error {
				p := usage(playback)
				_, err := svc.UpdateExercise(ctx, teacherCaller(), created.ID, "Name the lick, edited", p.prompt, []string{"skill-1"}, []string{"concept-1"},
					p.imageURL, nil, p.diagramRef, nil, p.options, nil, p.remediation, []string{"en"}, nil)
				return err
			}
		}
	}
	fretted := domain.Position{ID: "pos-6-5", Interval: "R", NoteName: "A", String: intPtr(6), Fret: intPtr(5)}

	doors := []door{
		{name: "an article's body", field: "rich_content", save: func(t *testing.T, playback string) (*fakeDiagramRepository, func(string) error) {
			svc, _, diagrams := contentService(t)
			node, err := svc.CreateContentNode(ctx, teacherCaller(), article(withEmbeddedPlayback(choosing(playback), "diagram-1")))
			require.NoError(t, err)
			return diagrams, func(playback string) error {
				_, err := svc.UpdateContentNode(ctx, teacherCaller(), node.ID, article(withEmbeddedPlayback(choosing(playback), "diagram-1")))
				return err
			}
		}},
		{name: "a rich-text pop-up", field: "rich_content", save: func(t *testing.T, playback string) (*fakeDiagramRepository, func(string) error) {
			svc, nodes, diagrams := contentService(t)
			nodes.put(articleNode("node-1"))
			item, err := svc.CreateExpandedContent(ctx, teacherCaller(), "node-1", domain.ExpandedContentTypeRichText, nil, withEmbeddedPlayback(choosing(playback), "diagram-1"), nil, nil, nil, nil, intPtr(3), intPtr(8000), nil)
			require.NoError(t, err)
			return diagrams, func(playback string) error {
				_, err := svc.UpdateExpandedContent(ctx, teacherCaller(), item.ID, domain.ExpandedContentTypeRichText, nil, withEmbeddedPlayback(choosing(playback), "diagram-1"), nil, nil, nil, nil, intPtr(3), intPtr(8000), strPtr("Edited"))
				return err
			}
		}},
		{name: "a diagram pop-up", field: "diagram_ref", save: func(t *testing.T, playback string) (*fakeDiagramRepository, func(string) error) {
			svc, nodes, diagrams := contentService(t)
			nodes.put(videoNode("node-1"))
			item, err := svc.CreateExpandedContent(ctx, teacherCaller(), "node-1", domain.ExpandedContentTypeDiagram, nil, nil, refChoosing(playback), nil, intPtr(150), intPtr(165), nil, nil, nil)
			require.NoError(t, err)
			return diagrams, func(playback string) error {
				_, err := svc.UpdateExpandedContent(ctx, teacherCaller(), item.ID, domain.ExpandedContentTypeDiagram, nil, nil, refChoosing(playback), nil, intPtr(150), intPtr(165), nil, nil, strPtr("Edited"))
				return err
			}
		}},
		{name: "an exercise's prompt", field: "prompt", save: exercise(nil, func(playback string) exerciseParts {
			return exerciseParts{exerciseType: domain.ExerciseTypeImageRecognition, prompt: *withEmbeddedPlayback(choosing(playback), "diagram-1"),
				imageURL: strPtr("https://cdn.example.com/a.png"), options: imageRecognitionOptions()}
		})},
		{name: "an exercise's remediation", field: "remediation_targets", save: exercise(nil, func(playback string) exerciseParts {
			return exerciseParts{exerciseType: domain.ExerciseTypeImageRecognition, prompt: domain.NewPlainTextPrompt("Which lick?"),
				imageURL: strPtr("https://cdn.example.com/a.png"), options: imageRecognitionOptions(),
				remediation: []domain.RemediationTarget{{RichContent: withEmbeddedPlayback(choosing(playback), "diagram-1")}}}
		})},
		{name: "an exercise option's thumbnail", field: "options", save: exercise(nil, func(playback string) exerciseParts {
			return exerciseParts{exerciseType: domain.ExerciseTypeImageChoice, prompt: domain.NewPlainTextPrompt("Which one is the lick?"),
				options: []domain.Option{{ID: "opt-1", IsCorrect: true, DiagramRef: refChoosing(playback)}, {ID: "opt-2", ImageURL: strPtr("https://cdn.example.com/b.png")}}}
		})},
		{name: "an exercise's diagram stimulus", field: "diagram_ref", save: exercise([]domain.Position{fretted}, func(playback string) exerciseParts {
			ref := refChoosing(playback)
			ref.CorrectPositionIDs = &[]string{"pos-6-5"}
			return exerciseParts{exerciseType: domain.ExerciseTypeImageRecognition, prompt: domain.NewPlainTextPrompt("Tap every root"), diagramRef: ref}
		})},
	}

	for _, d := range doors {
		t.Run(d.name+" can be saved again after its playback is removed", func(t *testing.T) {
			diagrams, resave := d.save(t, "pb-arp")
			givePlaybacks(t, diagrams, "diagram-1", "pb-strum")

			require.NoError(t, resave("pb-arp"))
		})
		t.Run(d.name+" still rejects choosing a removed playback it didn't have", func(t *testing.T) {
			diagrams, resave := d.save(t, "pb-strum")
			givePlaybacks(t, diagrams, "diagram-1", "pb-strum")

			err := resave("pb-arp")

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, d.field, valErr.Fields[0].Field)
		})
	}
}
