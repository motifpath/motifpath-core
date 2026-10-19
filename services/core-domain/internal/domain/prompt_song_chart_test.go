package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

const embeddedChartID = "11111111-1111-4111-8111-111111111111"

func songChartNode(id *string) domain.PromptNode {
	return domain.PromptNode{Type: domain.PromptNodeTypeSongChart, Attrs: &domain.PromptNodeAttrs{SongChartID: id}}
}

func docWith(node domain.PromptNode) *domain.PromptDocument {
	return &domain.PromptDocument{Type: "doc", Content: []domain.PromptNode{
		{Type: domain.PromptNodeTypeParagraph, Content: []domain.PromptNode{{Type: domain.PromptNodeTypeText, Text: "Play along:"}}},
		node,
	}}
}

func articleWith(doc *domain.PromptDocument) (domain.ContentNode, error) {
	return domain.NewContentNode("node-1", "teacher-1", domain.ContentTypeArticle, domain.ContentNodeFields{
		Title: "Forró songs", SkillIDs: []string{"s"}, ConceptIDs: []string{"c"}, Difficulty: domain.DifficultyLevelBeginner,
		LanguageCodes: []string{"pt_BR"}, RichContent: doc,
	}, time.Now())
}

func TestSongChartPromptNode(t *testing.T) {
	id := embeddedChartID

	t.Run("an article's body embeds a song chart", func(t *testing.T) {
		node, err := articleWith(docWith(songChartNode(&id)))

		require.NoError(t, err)
		assert.Equal(t, []string{embeddedChartID}, node.RichContent.EmbeddedSongChartIDs())
	})

	t.Run("rich expanded content embeds a song chart", func(t *testing.T) {
		_, err := domain.NewExpandedContent("item-1", "node-1", domain.ContentTypeVideo, domain.ExpandedContentTypeRichText,
			nil, docWith(songChartNode(&id)), nil, nil, intPtr(30), intPtr(60), nil, nil, nil, time.Now())

		require.NoError(t, err)
	})

	invalid := []struct {
		name string
		id   *string
	}{
		{name: "without a song chart", id: nil},
		{name: "whose song chart isn't a UUID", id: ptrTo("asa-branca")},
	}
	for _, tt := range invalid {
		t.Run("a song chart node "+tt.name+" is refused", func(t *testing.T) {
			_, err := articleWith(docWith(songChartNode(tt.id)))

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "rich_content", valErr.Fields[0].Field)
			assert.Contains(t, valErr.Fields[0].Reason, "songChartId")
		})
	}

	t.Run("an exercise's prompt can't embed a song chart", func(t *testing.T) {
		title, exerciseType, skillIDs, conceptIDs, options, languageCodes, createdAt := minimalExerciseArgs()

		_, err := domain.NewExercise("ex-1", title, *docWith(songChartNode(&id)), exerciseType, skillIDs, conceptIDs,
			nil, nil, nil, nil, options, nil, nil, languageCodes, createdAt)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Fields[0].Reason, "songChart")
	})

	t.Run("an exercise's remediation content can't embed a song chart", func(t *testing.T) {
		title, exerciseType, skillIDs, conceptIDs, options, languageCodes, createdAt := minimalExerciseArgs()
		remediation := []domain.RemediationTarget{{RichContent: docWith(songChartNode(&id))}}

		_, err := domain.NewExercise("ex-1", title, domain.NewPlainTextPrompt("Name it"), exerciseType, skillIDs, conceptIDs,
			nil, nil, nil, nil, options, nil, remediation, languageCodes, createdAt)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
	})
}

func ptrTo[T any](v T) *T { return &v }
