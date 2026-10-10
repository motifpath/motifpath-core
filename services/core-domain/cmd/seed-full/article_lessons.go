package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// diagramFirstBody is an article that opens with a large diagram — the shape
// is the lesson — and embeds a chord voicing and a song chart further down.
func diagramFirstBody(lead, voicing domain.DiagramRef, songChartID string) domain.PromptDocument {
	text := func(s string) domain.PromptNode {
		return domain.PromptNode{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{{Type: domain.PromptNodeTypeText, Text: s}}}
	}
	return domain.PromptDocument{Type: "doc", Content: []domain.PromptNode{
		{Type: domain.PromptNodeTypeDiagram, Attrs: &domain.PromptNodeAttrs{DiagramRef: &lead}},
		text("Two boxes of the A minor pentatonic, joined into one long shape. Find the root (R) in each box before you play it."),
		text("Slide between the boxes on the G string, then play the whole shape up the neck and back down."),
		{Type: domain.PromptNodeTypeDiagram, Attrs: &domain.PromptNodeAttrs{DiagramRef: &voicing}},
		text("The E major chord, fingered as numbered. Strum it, then pick it one string at a time."),
		{Type: domain.PromptNodeTypeSongChart, Attrs: &domain.PromptNodeAttrs{SongChartID: &songChartID}},
		text("Put the shapes to work in a song you know: play its chords on the top strings only."),
	}}
}

// textQuestion is one text_response exercise of a seeded challenge: the
// question, its right answer and a wrong one.
type textQuestion struct{ title, right, wrong string }

// seedTextChallenge gives node a practice challenge of text_response
// exercises on one skill and concept, so finishing the lesson's step goes
// through a short practice run.
func seedTextChallenge(ctx context.Context, teacher domain.User, challengeSvc *application.ChallengeService, exerciseSvc *application.ExerciseService, classifier *classificationSeeder, node domain.ContentNode, keys exerciseClassification, questions []textQuestion) error {
	skillID, conceptID, err := classifier.ids(ctx, keys)
	if err != nil {
		return err
	}
	challenge, err := challengeSvc.CreateChallenge(ctx, teacher, node.ID, &skillID, nil, 70, nil, false, false)
	if err != nil {
		return fmt.Errorf("create challenge for %q: %w", node.Title, err)
	}
	for _, q := range questions {
		right, wrong := q.right, q.wrong
		options := []domain.Option{
			{ID: uuid.NewString(), IsCorrect: true, Label: &right},
			{ID: uuid.NewString(), IsCorrect: false, Label: &wrong},
		}
		exercise, err := exerciseSvc.CreateExercise(ctx, teacher, q.title, domain.NewPlainTextPrompt(q.title), domain.ExerciseTypeTextResponse,
			[]string{skillID}, []string{conceptID}, nil, nil, nil, nil, options, nil, nil, []string{"en"}, forGuitars())
		if err != nil {
			return fmt.Errorf("create exercise %q: %w", q.title, err)
		}
		if _, err := exerciseSvc.LinkExerciseToChallenge(ctx, teacher, challenge.ID, exercise.ID); err != nil {
			return fmt.Errorf("link exercise %q: %w", q.title, err)
		}
	}
	return nil
}

// articleSpec is one article lesson seedArticleLessons creates.
type articleSpec struct {
	key, title string
	keys       exerciseClassification
	languages  []string
	body       domain.PromptDocument
	challenge  []textQuestion
}

// seedArticleLessons creates and publishes the articles the admin's path
// walks through, one per way a student meets an article lesson, keyed for
// adminPathNodeKeys:
//
//   - article-review: done, with a challenge, so it reopens as a review that
//     offers only Practise again;
//   - article-diagram-first: starts with a diagram, with a chord voicing and
//     a song chart in its text; Mark as done opens the next step;
//   - article-challenge: ends with Practise this, whose challenge finishes it;
//   - article-pt-br-only: only in Portuguese (Brazil), so a student whose
//     language is English sees it explain itself and offer to read it in
//     Portuguese.
//
// The cued article (article-cues) is seedDiagramLessons' article.
func seedArticleLessons(ctx context.Context, teacher domain.User, svc services, classifier *classificationSeeder, diagrams seededDiagrams, songChartID string) (map[string]domain.ContentNode, error) {
	specs := articleSpecs(diagrams, songChartID)
	result := make(map[string]domain.ContentNode, len(specs))
	for _, spec := range specs {
		skillID, conceptID, err := classifier.ids(ctx, spec.keys)
		if err != nil {
			return nil, err
		}
		body := spec.body
		node, err := svc.content.CreateContentNode(ctx, teacher, application.ContentNodeInput{
			Title: spec.title, ContentType: domain.ContentTypeArticle, SkillIDs: []string{skillID}, ConceptIDs: []string{conceptID},
			Difficulty: domain.DifficultyLevelBeginner, Languages: spec.languages, RichContent: &body, InstrumentIDs: guitars,
		})
		if err != nil {
			return nil, fmt.Errorf("create article %q: %w", spec.title, err)
		}
		if _, err := svc.content.PublishContentNode(ctx, teacher, node.ID); err != nil {
			return nil, fmt.Errorf("publish article %q: %w", spec.title, err)
		}
		if len(spec.challenge) > 0 {
			if err := seedTextChallenge(ctx, teacher, svc.challenge, svc.exercise, classifier, node, spec.keys, spec.challenge); err != nil {
				return nil, err
			}
		}
		result[spec.key] = node
	}
	return result, nil
}

// articleLessonKeys are the article lessons on the admin's path:
// articleSpecs' plus article-cues, seedDiagramLessons' article.
var articleLessonKeys = []string{"article-review", "article-cues", "article-diagram-first", "article-challenge", "article-pt-br-only"}

// articleSpecs are the articles seedArticleLessons creates; see there.
func articleSpecs(diagrams seededDiagrams, songChartID string) []articleSpec {
	openChords := exerciseClassification{skill: "play-open-chords", concept: "open-chord-shapes"}
	pentatonic := exerciseClassification{skill: "play-pentatonic-positions", concept: "pentatonic-shapes"}
	return []articleSpec{
		{
			key: "article-review", title: "Three open chords", keys: openChords, languages: []string{"en"},
			body: headedParagraph("G, C and D", "Most songs you know can be played with these three open chords. Keep your fingers close to the frets."),
			challenge: []textQuestion{
				{"Which three open chords play most songs in G?", "G, C and D", "A, B and E"},
				{"Where should your fingers sit on a fret?", "Close to the fret wire", "In the middle of the fret"},
			},
		},
		{
			key: "article-diagram-first", title: "One shape across the neck", keys: pentatonic, languages: []string{"en"},
			body: diagramFirstBody(*intervalsRef(diagrams.pentatonicJoined.ID), *intervalsRef(diagrams.eMajorChord.ID), songChartID),
		},
		{
			key: "article-challenge", title: "Changing chords in time", keys: openChords, languages: []string{"en"},
			body: paragraphs(
				"A chord change sounds smooth when your hand moves before the beat, not on it.",
				"Lift all your fingers together, and land them as one shape.",
				"Start at a slow tempo and only speed up once every change lands on time.",
			),
			challenge: []textQuestion{
				{"When should your hand start moving to the next chord?", "Just before the beat", "On the beat"},
				{"How should your fingers land on the new chord?", "Together, as one shape", "One at a time"},
			},
		},
		{
			key: "article-pt-br-only", title: "Levada de samba no violão", keys: openChords, languages: []string{"pt_BR"},
			body: paragraphs(
				"A levada de samba alterna o polegar no baixo com os outros dedos nas cordas agudas.",
				"Comece devagar, contando em dois, e acentue o segundo tempo.",
			),
		},
	}
}

// seedAdminArticles seeds the article lessons of the admin's path, keyed as
// articleLessonKeys: seedArticleLessons' articles, embedding the first
// published song chart, and cued, the article with paragraph cues.
func seedAdminArticles(ctx context.Context, teacher domain.User, svc services, classifier *classificationSeeder, diagrams seededDiagrams, songCharts []domain.SongChart, cued domain.ContentNode) (map[string]domain.ContentNode, error) {
	chartID, err := publishedSongChartID(songCharts)
	if err != nil {
		return nil, err
	}
	articles, err := seedArticleLessons(ctx, teacher, svc, classifier, diagrams, chartID)
	if err != nil {
		return nil, fmt.Errorf("seed article lessons: %w", err)
	}
	articles["article-cues"] = cued
	return articles, nil
}

// publishedSongChartID is the id of the first seeded song chart students can
// open, for an article to embed.
func publishedSongChartID(charts []domain.SongChart) (string, error) {
	for _, chart := range charts {
		if chart.Status == domain.SongChartPublished {
			return chart.ID, nil
		}
	}
	return "", fmt.Errorf("no published song chart to embed")
}
