package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

const (
	publishedChartID = "11111111-1111-4111-8111-111111111111"
	draftChartID     = "22222222-2222-4222-8222-222222222222"
	withdrawnChartID = "33333333-3333-4333-8333-333333333333"
	missingChartID   = "44444444-4444-4444-8444-444444444444"
)

// chartsForEmbedding holds a published chart, one never published, and one
// published and then withdrawn.
func chartsForEmbedding() *fakeSongChartRepository {
	charts := newFakeSongChartRepository()
	revision := &domain.SongChartRevisionSummary{Number: 1, Title: "Asa Branca"}
	charts.charts[publishedChartID] = domain.SongChart{ID: publishedChartID, Status: domain.SongChartPublished, PublishedRevision: revision}
	charts.charts[draftChartID] = domain.SongChart{ID: draftChartID, Status: domain.SongChartDraftStatus}
	charts.charts[withdrawnChartID] = domain.SongChart{ID: withdrawnChartID, Status: domain.SongChartWithdrawn, PublishedRevision: revision}
	return charts
}

func contentServiceWithCharts(nodes *fakeContentNodeRepository, expanded *fakeExpandedContentRepository) *application.ContentService {
	return application.NewContentService(nodes, expanded, seededKnowledgeNodeRepository(), newFakeContentNodeVersionRepository(), newFakeDiagramRepository(),
		seededInstrumentRepository(), newFakeVoiceRepository(), chartsForEmbedding(), idSequence(), func() time.Time { return fixedCreatedAt })
}

func bodyEmbedding(chartID string) *domain.PromptDocument {
	doc := richTextContent("Play along:")
	doc.Content = append(doc.Content, domain.PromptNode{Type: domain.PromptNodeTypeSongChart, Attrs: &domain.PromptNodeAttrs{SongChartID: &chartID}})
	return &doc
}

func articleEmbedding(chartID string) application.ContentNodeInput {
	return application.ContentNodeInput{
		Title: "Forró songs", ContentType: domain.ContentTypeArticle, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"},
		Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"pt_BR"}, RichContent: bodyEmbedding(chartID),
	}
}

func TestContentService_SongChartEmbeds(t *testing.T) {
	ctx := context.Background()

	accepted := []struct {
		name    string
		chartID string
	}{
		{name: "a published chart", chartID: publishedChartID},
		{name: "a chart published and withdrawn since", chartID: withdrawnChartID},
	}
	for _, tt := range accepted {
		t.Run("an article's body embeds "+tt.name, func(t *testing.T) {
			svc := contentServiceWithCharts(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

			node, err := svc.CreateContentNode(ctx, teacherCaller(), articleEmbedding(tt.chartID))

			require.NoError(t, err)
			assert.Equal(t, []string{tt.chartID}, node.RichContent.EmbeddedSongChartIDs())
		})
	}

	refused := []struct {
		name    string
		chartID string
	}{
		{name: "a chart never published", chartID: draftChartID},
		{name: "a chart that doesn't exist", chartID: missingChartID},
	}
	for _, tt := range refused {
		t.Run("an article's body can't embed "+tt.name, func(t *testing.T) {
			nodes := newFakeContentNodeRepository()
			svc := contentServiceWithCharts(nodes, newFakeExpandedContentRepository())

			_, err := svc.CreateContentNode(ctx, teacherCaller(), articleEmbedding(tt.chartID))

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "rich_content", valErr.Fields[0].Field)
			assert.Contains(t, valErr.Fields[0].Reason, "songChartId")
		})

		t.Run("rich expanded content can't embed "+tt.name, func(t *testing.T) {
			nodes := newFakeContentNodeRepository()
			nodes.put(videoNode("node-1"))
			svc := contentServiceWithCharts(nodes, newFakeExpandedContentRepository())

			_, err := svc.CreateExpandedContent(ctx, teacherCaller(), "node-1", domain.ExpandedContentTypeRichText, nil, bodyEmbedding(tt.chartID), nil, nil,
				intPtr(30), intPtr(60), nil, nil, nil)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "rich_content", valErr.Fields[0].Field)
		})
	}

	t.Run("an article can't be changed to embed a chart never published", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		svc := contentServiceWithCharts(nodes, newFakeExpandedContentRepository())
		node, err := svc.CreateContentNode(ctx, teacherCaller(), articleEmbedding(publishedChartID))
		require.NoError(t, err)

		_, err = svc.UpdateContentNode(ctx, teacherCaller(), node.ID, articleEmbedding(draftChartID))

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
	})

	t.Run("rich expanded content can't be changed to embed a chart that doesn't exist", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := contentServiceWithCharts(nodes, newFakeExpandedContentRepository())
		item, err := svc.CreateExpandedContent(ctx, teacherCaller(), "node-1", domain.ExpandedContentTypeRichText, nil, bodyEmbedding(publishedChartID), nil, nil,
			intPtr(30), intPtr(60), nil, nil, nil)
		require.NoError(t, err)

		_, err = svc.UpdateExpandedContent(ctx, teacherCaller(), item.ID, domain.ExpandedContentTypeRichText, nil, bodyEmbedding(missingChartID), nil, nil,
			intPtr(30), intPtr(60), nil, nil, nil)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
	})
}
