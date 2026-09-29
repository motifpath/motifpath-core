package main

import (
	"context"
	"fmt"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// paragraphs builds a PromptDocument of one plain paragraph per entry.
func paragraphs(texts ...string) domain.PromptDocument {
	content := make([]domain.PromptNode, len(texts))
	for i, text := range texts {
		content[i] = domain.PromptNode{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{{Type: domain.PromptNodeTypeText, Text: text}}}
	}
	return domain.PromptDocument{Type: "doc", Content: content}
}

// headedParagraph builds a PromptDocument of a level-3 heading over one paragraph.
func headedParagraph(heading, text string) domain.PromptDocument {
	level := 3
	return domain.PromptDocument{Type: "doc", Content: []domain.PromptNode{
		{Type: domain.PromptNodeTypeHeading, Attrs: &domain.PromptNodeAttrs{Level: &level}, Content: []domain.PromptNode{{Type: domain.PromptNodeTypeText, Text: heading}}},
		{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{{Type: domain.PromptNodeTypeText, Text: text}}},
	}}
}

// intervalsRef shows a diagram with its interval labels, the way an author
// embeds one by default.
func intervalsRef(diagramID string) *domain.DiagramRef {
	return &domain.DiagramRef{DiagramID: diagramID, Layers: domain.DiagramLayers{Intervals: true}}
}

// seededLessons is the content seedDiagramLessons creates.
type seededLessons struct {
	video     domain.ContentNode // cues of every kind: image, rich text, diagram
	article   domain.ContentNode // paragraph pop-ups of every kind, three published versions and unpublished edits
	scenarios domain.ContentNode // one diagram cue per way a student sees an embedded diagram
}

// seedDiagramLessons creates a guitar video and article that use the seeded
// diagrams the way lessons do: the video carries timed cues (an image, a
// rich-text note and a diagram), and the article carries paragraph pop-ups
// of the same three kinds. The article is published, edited and published
// again twice, then edited once more without publishing, so its version
// history has three versions and its current draft is ahead of the latest.
// Neither node is placed in a learning path, so no seeded progress depends
// on them.
func seedDiagramLessons(ctx context.Context, teacher domain.User, content *application.ContentService, classifier *classificationSeeder, diagrams seededDiagrams) (seededLessons, error) {
	skillID, err := classifier.skillID(ctx, "Scales")
	if err != nil {
		return seededLessons{}, err
	}
	conceptID, err := classifier.conceptID(ctx, "Pentatonic scale shapes")
	if err != nil {
		return seededLessons{}, err
	}
	base := application.ContentNodeInput{
		SkillIDs: []string{skillID}, ConceptIDs: []string{conceptID}, Difficulty: domain.DifficultyLevelIntermediate,
		Languages: []string{"en"}, InstrumentIDs: []string{diagrams.guitar.ID},
	}
	video, err := seedDiagramVideo(ctx, teacher, content, base, diagrams.pentatonicPos1.ID)
	if err != nil {
		return seededLessons{}, err
	}
	article, err := seedDiagramArticle(ctx, teacher, content, base, diagrams.pentatonicPos2.ID)
	if err != nil {
		return seededLessons{}, err
	}
	scenarios, err := seedDiagramScenarioVideo(ctx, teacher, content, base, diagrams)
	if err != nil {
		return seededLessons{}, err
	}
	return seededLessons{video: video, article: article, scenarios: scenarios}, nil
}

// lessonExtra is one cue or pop-up: an image, a rich-text note or a diagram.
type lessonExtra struct {
	kind     domain.ExpandedContentType
	mediaURL *string
	rich     *domain.PromptDocument
	diagram  *domain.DiagramRef
	// from and to are seconds into a video, or a paragraph and a duration in
	// milliseconds in an article.
	from, to int
	caption  string
}

// seedDiagramVideo creates and publishes a video with an image, a rich-text
// and a diagram cue.
func seedDiagramVideo(ctx context.Context, teacher domain.User, content *application.ContentService, input application.ContentNodeInput, diagramID string) (domain.ContentNode, error) {
	input.Title, input.ContentType = "Pentatonic box shapes", domain.ContentTypeVideo
	input.MediaURL = stringPtr("https://samplelib.com/lib/preview/mp4/sample-10s.mp4")
	input.ThumbnailURL = stringPtr("https://placehold.co/640x360/png?text=Pentatonic+box+shapes")
	video, err := content.CreateContentNode(ctx, teacher, input)
	if err != nil {
		return domain.ContentNode{}, fmt.Errorf("create diagram video: %w", err)
	}
	if _, err := content.PublishContentNode(ctx, teacher, video.ID); err != nil {
		return domain.ContentNode{}, fmt.Errorf("publish diagram video: %w", err)
	}
	cues := []lessonExtra{
		{domain.ExpandedContentTypeImage, stringPtr("https://placehold.co/640x360/png?text=Box+1"), nil, nil, 0, 2, "Box 1 at the 5th fret"},
		{domain.ExpandedContentTypeRichText, nil, docPtr(headedParagraph("Listen for the root", "Every box starts and ends on A, the root of A minor.")), nil, 3, 5, "Where the root sits"},
		{domain.ExpandedContentTypeDiagram, nil, nil, intervalsRef(diagramID), 6, 9, "Position 1 on the fretboard"},
	}
	for _, cue := range cues {
		start, end, caption := cue.from, cue.to, cue.caption
		if _, err := content.CreateExpandedContent(ctx, teacher, video.ID, cue.kind, cue.mediaURL, cue.rich, cue.diagram, nil, &start, &end, nil, nil, &caption); err != nil {
			return domain.ContentNode{}, fmt.Errorf("create %s cue: %w", cue.kind, err)
		}
	}
	return video, nil
}

// diagramCue is one timed cue of the diagram scenario video: a diagram, a
// stack of diagrams, or a rich-text note with a diagram inline.
type diagramCue struct {
	diagram  *domain.DiagramRef
	stack    *domain.DiagramStackRef
	rich     *domain.PromptDocument
	from, to int
	caption  string
}

// noteWithDiagram builds a rich-text note whose diagram sits inline between
// two paragraphs, the way an author embeds one in a prompt document.
func noteWithDiagram(before string, ref domain.DiagramRef, after string) domain.PromptDocument {
	doc := paragraphs(before, after)
	diagram := domain.PromptNode{Type: domain.PromptNodeTypeDiagram, Attrs: &domain.PromptNodeAttrs{DiagramRef: &ref}}
	doc.Content = []domain.PromptNode{doc.Content[0], diagram, doc.Content[1]}
	return doc
}

// seedDiagramScenarioVideo creates and publishes a 30-second video whose cues
// cover each way a student sees an embedded diagram: a single diagram, a
// switch straight on to another one, a stack of two, a diagram inline in a
// rich-text note, a diagram whose author hid its labels, and a diagram shown
// through an interval subset. Like the other lessons it's in no learning
// path, so it's opened by its lesson URL (logged by the seed).
func seedDiagramScenarioVideo(ctx context.Context, teacher domain.User, content *application.ContentService, input application.ContentNodeInput, diagrams seededDiagrams) (domain.ContentNode, error) {
	input.Title, input.ContentType = "Reading diagrams in a lesson", domain.ContentTypeVideo
	input.MediaURL = stringPtr("https://samplelib.com/preview/mp4/sample-30s.mp4")
	input.ThumbnailURL = stringPtr("https://placehold.co/640x360/png?text=Reading+diagrams")
	video, err := content.CreateContentNode(ctx, teacher, input)
	if err != nil {
		return domain.ContentNode{}, fmt.Errorf("create diagram scenario video: %w", err)
	}
	if _, err := content.PublishContentNode(ctx, teacher, video.ID); err != nil {
		return domain.ContentNode{}, fmt.Errorf("publish diagram scenario video: %w", err)
	}

	pos1, pos2 := *intervalsRef(diagrams.pentatonicPos1.ID), *intervalsRef(diagrams.pentatonicPos2.ID)
	// The lick has a rhythm (a triplet turn at 80 BPM); this cue offers Play, as authored, once through.
	lickWithPlay := *intervalsRef(diagrams.teacherLick.ID)
	lickWithPlay.Playback = &domain.DiagramPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored}
	rootsOnly := domain.DiagramRef{DiagramID: diagrams.pentatonicPos1.ID, Layers: domain.DiagramLayers{Intervals: true, Subset: &[]string{"R"}}}
	cues := []diagramCue{
		{diagram: &pos1, from: 0, to: 5, caption: "1 · One diagram: position 1"},
		// Starts the second the previous cue ends: the diagram must switch in place.
		{diagram: &pos2, from: 5, to: 10, caption: "2 · Straight on to the next diagram: position 2"},
		{stack: &domain.DiagramStackRef{Stack: []domain.DiagramRef{pos1, pos2}}, from: 11, to: 15, caption: "3 · Two diagrams stacked: positions 1 and 2"},
		{rich: docPtr(noteWithDiagram("Both boxes join into one long shape:", *intervalsRef(diagrams.pentatonicJoined.ID), "Slide between them on the G string.")),
			from: 16, to: 20, caption: "4 · A diagram inside a rich-text note"},
		{diagram: &lickWithPlay, from: 21, to: 25, caption: "5 · Labels hidden by the diagram's author; press Play to hear the lick"},
		{diagram: &rootsOnly, from: 26, to: 30, caption: "6 · Only the roots of position 1"},
	}
	for _, cue := range cues {
		kind := domain.ExpandedContentTypeDiagram
		if cue.rich != nil {
			kind = domain.ExpandedContentTypeRichText
		}
		start, end, caption := cue.from, cue.to, cue.caption
		if _, err := content.CreateExpandedContent(ctx, teacher, video.ID, kind, nil, cue.rich, cue.diagram, cue.stack, &start, &end, nil, nil, &caption); err != nil {
			return domain.ContentNode{}, fmt.Errorf("create %q cue: %w", caption, err)
		}
	}
	return video, nil
}

// seedPlayableCue adds a diagram cue that offers Play to nodeID, a lesson on
// real students' paths (the admin's current one included), so the player can
// be tried where a student meets it: the blues lick, played as authored once
// through, from 0:11 to 0:25 — after the node's image cues.
func seedPlayableCue(ctx context.Context, teacher domain.User, content *application.ContentService, nodeID string, diagrams seededDiagrams) error {
	ref := *intervalsRef(diagrams.teacherLick.ID)
	ref.Playback = &domain.DiagramPlayback{Direction: domain.DiagramPlaybackDirectionAsAuthored}
	start, end, caption := 11, 25, "Press Play to hear the lick"
	if _, err := content.CreateExpandedContent(ctx, teacher, nodeID, domain.ExpandedContentTypeDiagram, nil, nil, &ref, nil, &start, &end, nil, nil, &caption); err != nil {
		return fmt.Errorf("create playable diagram cue: %w", err)
	}
	return nil
}

// seedDiagramArticle creates an article with an image, a rich-text and a
// diagram pop-up, publishes three versions of it, then edits it once more
// without publishing.
func seedDiagramArticle(ctx context.Context, teacher domain.User, content *application.ContentService, input application.ContentNodeInput, diagramID string) (domain.ContentNode, error) {
	input.Title, input.ContentType = "Soloing with two pentatonic boxes", domain.ContentTypeArticle
	input.RichContent = docPtr(paragraphs(
		"Most blues solos in A live in two boxes of the A minor pentatonic.",
		"Box 1 sits between the 5th and 8th frets, with the root under your first finger.",
		"Box 2 starts where box 1 ends, from the 7th to the 10th fret.",
		"Slide between them on the 3rd string to connect the two.",
	))
	article, err := content.CreateContentNode(ctx, teacher, input)
	if err != nil {
		return domain.ContentNode{}, fmt.Errorf("create diagram article: %w", err)
	}
	popups := []lessonExtra{
		{domain.ExpandedContentTypeImage, stringPtr("https://placehold.co/640x360/png?text=Blues+in+A"), nil, nil, 1, 5000, "A 12-bar blues in A"},
		{domain.ExpandedContentTypeRichText, nil, docPtr(headedParagraph("Why the root matters", "Landing on the root at the end of a phrase makes it sound resolved.")), nil, 2, 5000, "Why the root matters"},
		{domain.ExpandedContentTypeDiagram, nil, nil, intervalsRef(diagramID), 3, 5000, "Box 2"},
	}
	for _, popup := range popups {
		paragraph, duration, caption := popup.from, popup.to, popup.caption
		if _, err := content.CreateExpandedContent(ctx, teacher, article.ID, popup.kind, popup.mediaURL, popup.rich, popup.diagram, nil, nil, nil, &paragraph, &duration, &caption); err != nil {
			return domain.ContentNode{}, fmt.Errorf("create %s pop-up: %w", popup.kind, err)
		}
	}
	return publishRevisions(ctx, teacher, content, article, input, []string{
		"Soloing over a blues with two pentatonic boxes",
		"Soloing over a 12-bar blues with two pentatonic boxes",
		"Soloing over a 12-bar blues in A with two pentatonic boxes",
	})
}

// publishRevisions publishes node as it stands, then retitles it with each of
// titles in turn, publishing every revision but the last, so the node ends
// with len(titles) published versions and unpublished edits on top.
func publishRevisions(ctx context.Context, teacher domain.User, content *application.ContentService, node domain.ContentNode, input application.ContentNodeInput, titles []string) (domain.ContentNode, error) {
	if _, err := content.PublishContentNode(ctx, teacher, node.ID); err != nil {
		return domain.ContentNode{}, fmt.Errorf("publish %q: %w", node.Title, err)
	}
	for i, title := range titles {
		input.Title = title
		updated, err := content.UpdateContentNode(ctx, teacher, node.ID, input)
		if err != nil {
			return domain.ContentNode{}, fmt.Errorf("edit %q: %w", node.Title, err)
		}
		node = updated
		if i == len(titles)-1 {
			break
		}
		if _, err := content.PublishContentNode(ctx, teacher, node.ID); err != nil {
			return domain.ContentNode{}, fmt.Errorf("publish %q: %w", node.Title, err)
		}
	}
	return node, nil
}

func docPtr(doc domain.PromptDocument) *domain.PromptDocument { return &doc }
