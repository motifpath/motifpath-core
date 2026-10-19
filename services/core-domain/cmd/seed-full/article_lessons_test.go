package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

const (
	leadDiagramID    = "11111111-1111-4111-8111-111111111111"
	voicingDiagramID = "22222222-2222-4222-8222-222222222222"
	songChartID      = "33333333-3333-4333-8333-333333333333"
)

// asArticle builds an article node with body, so the domain's own rules for
// an article's text check it the way creating one through the API would.
func asArticle(t *testing.T, body domain.PromptDocument) {
	t.Helper()
	_, err := domain.NewContentNode("node-1", "teacher-1", domain.ContentTypeArticle, domain.ContentNodeFields{
		Title: "Article", SkillIDs: []string{"s1"}, ConceptIDs: []string{"c1"}, Difficulty: domain.DifficultyLevelBeginner,
		LanguageCodes: []string{"en"}, RichContent: &body,
	}, time.Now())
	require.NoError(t, err)
}

func TestSeededArticleLessonBodies(t *testing.T) {
	t.Run("the diagram-first article starts with its diagram and embeds a chord voicing and a song chart", func(t *testing.T) {
		body := diagramFirstBody(*intervalsRef(leadDiagramID), *intervalsRef(voicingDiagramID), songChartID)

		require.NotEmpty(t, body.Content)
		assert.Equal(t, domain.PromptNodeTypeDiagram, body.Content[0].Type)
		refs := body.EmbeddedDiagramRefs()
		require.Len(t, refs, 2)
		assert.Equal(t, leadDiagramID, refs[0].DiagramID)
		assert.Equal(t, voicingDiagramID, refs[1].DiagramID)
		assert.Equal(t, []string{songChartID}, body.EmbeddedSongChartIDs())
		asArticle(t, body)
	})

	t.Run("the cued article has a pop-up of every kind, one without a duration and one past its last paragraph", func(t *testing.T) {
		body := soloingBody()
		popups := soloingPopups(leadDiagramID)

		paragraphs := 0
		for _, node := range body.Content {
			if node.Type == domain.PromptNodeTypeParagraph {
				paragraphs++
			}
		}
		kinds := map[domain.ExpandedContentType]bool{}
		withoutDuration, pastTheEnd := 0, 0
		for _, popup := range popups {
			kinds[popup.kind] = true
			if popup.to == 0 {
				withoutDuration++
			}
			if popup.from > paragraphs {
				pastTheEnd++
			}
		}
		assert.Len(t, kinds, 3, "an image, a rich-text note and a diagram")
		assert.Positive(t, withoutDuration)
		assert.Equal(t, 1, pastTheEnd)
		asArticle(t, body)
	})
}

func TestAdminPathWalksThroughTheArticleLesson(t *testing.T) {
	// Done, done, then the article scenarios in the order a student meets them, each opening the
	// next: cues and Mark as done, a diagram-first article, an article whose challenge finishes it.
	// A step only in another language comes last, since a language lock holds every step after it.
	assert.Equal(t, []string{
		"video-beginner", "article-review", "article-cues", "article-diagram-first", "article-challenge",
		"video-intermediate", "article-notes-root-strings", "article-notes-top-strings",
		"article-caged-grips", "article-pentatonic-boxes", "article-pt-br-only",
	}, adminPathNodeKeys)
	assert.ElementsMatch(t, []string{"video-beginner", "article-review"}, adminPathCompletedKeys)

	seeded := []string{"article-cues"}
	for _, spec := range articleSpecs(seededDiagrams{}, songChartID) {
		seeded = append(seeded, spec.key)
	}
	assert.ElementsMatch(t, articleLessonKeys, seeded, "every article lesson on the path is seeded, once")
}

func TestSeededArticleLessonSpecs(t *testing.T) {
	specs := map[string]articleSpec{}
	for _, spec := range articleSpecs(seededDiagrams{}, songChartID) {
		specs[spec.key] = spec
	}

	assert.NotEmpty(t, specs["article-review"].challenge, "a done article reopens offering Practise again")
	assert.NotEmpty(t, specs["article-challenge"].challenge, "ends with Practise this")
	assert.Empty(t, specs["article-diagram-first"].challenge, "ends with Mark as done")
	assert.Equal(t, []string{"pt_BR"}, specs["article-pt-br-only"].languages)
	for key, spec := range specs {
		if key != "article-diagram-first" {
			t.Run(key, func(t *testing.T) { asArticle(t, spec.body) })
		}
	}
}
