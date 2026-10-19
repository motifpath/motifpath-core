//go:build integration

package bdd

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerSongChartEmbedSteps(sc *godog.ScenarioContext, w *world) {
	// Given
	sc.Step(`^"([^"]+)" is a song chart that has never been published$`, func(title string) error {
		return w.hasSongChart(songChartSeeder, title, true, nil)
	})
	sc.Step(`^the draft of "([^"]+)" has been retitled "([^"]+)" and not published$`, func(title, newTitle string) error {
		if err := w.changesChartTitle(songChartSeeder, title, newTitle); err != nil {
			return err
		}
		return w.mustSucceed("retitling " + title)
	})
	sc.Step(`^the song chart "([^"]+)" has been withdrawn$`, w.seedWithdrawal)
	sc.Step(`^"([^"]+)" has a video content node "([^"]+)"$`, func(_, slug string) error {
		return w.putContentNode(slug, domain.ContentTypeVideo)
	})
	sc.Step(`^"([^"]+)" has an article "([^"]+)" whose body embeds the song chart "([^"]+)"$`, w.hasArticleEmbeddingChart)

	// When
	sc.Step(`^"([^"]+)" lists the song charts that are published$`, func(caller string) error {
		return w.listsSongChartsAs(caller, statusOf(generated.SongChartStatusPublished), "")
	})
	sc.Step(`^"([^"]+)" searches the song charts that are published for "([^"]+)"$`, func(caller, q string) error {
		return w.listsSongChartsAs(caller, statusOf(generated.SongChartStatusPublished), q)
	})
	sc.Step(`^"([^"]+)" lists song charts that are drafts$`, func(caller string) error {
		return w.listsSongChartsAs(caller, statusOf(generated.SongChartStatusDraft), "")
	})
	sc.Step(`^"([^"]+)" lists song charts that are withdrawn$`, func(caller string) error {
		return w.listsSongChartsAs(caller, statusOf(generated.SongChartStatusWithdrawn), "")
	})
	sc.Step(`^"([^"]+)" lists song charts without choosing status$`, func(caller string) error {
		return w.listsSongChartsAs(caller, nil, "")
	})
	sc.Step(`^"([^"]+)" creates an article whose body embeds the song chart "([^"]+)"$`, func(_, title string) error {
		id, err := w.chartID(title)
		if err != nil {
			return err
		}
		return w.createsArticleEmbedding("Forró songs", id)
	})
	sc.Step(`^"([^"]+)" creates an article whose body embeds a song chart that doesn't exist$`, func(string) error {
		return w.createsArticleEmbedding("Forró songs", deterministicUUID("song-chart", "missing"))
	})
	sc.Step(`^"([^"]+)" adds rich expanded content to "([^"]+)" that embeds the song chart "([^"]+)"$`, w.addsExpandedContentEmbeddingChart)
	sc.Step(`^"([^"]+)" creates an exercise whose prompt embeds the song chart "([^"]+)"$`, w.createsExerciseEmbeddingChart)
	sc.Step(`^"([^"]+)" changes the title of "([^"]+)" to "([^"]+)"$`, w.changesArticleTitle)

	// Then
	sc.Step(`^the list holds "([^"]+)" only, with its published revision (\d+)$`, w.listHoldsOnlyAtRevision)
	sc.Step(`^the list holds "([^"]+)" with its published title "([^"]+)"$`, w.listHoldsWithPublishedTitle)
	sc.Step(`^the article is saved with the song chart "([^"]+)" in its body$`, w.articleEmbedsChart)
	sc.Step(`^the article is saved with the song chart "([^"]+)" still in its body$`, w.articleEmbedsChart)
	sc.Step(`^the expanded content is saved with the song chart "([^"]+)"$`, w.expandedContentEmbedsChart)
	sc.Step(`^the content is refused as invalid$`, w.requestRejectedInvalid)
	sc.Step(`^the exercise is refused as invalid$`, w.requestRejectedInvalid)
	sc.Step(`^the rejection identifies the song chart node's songChartId as the source of the error$`, w.rejectionNamesSongChartID)
}

func statusOf(s generated.SongChartStatus) *generated.SongChartStatus { return &s }

// ── Building content that embeds a chart ─────────────────────────────────────

func songChartPromptNode(id uuid.UUID) generated.PromptNode {
	attrs := map[string]interface{}{"songChartId": id.String()}
	return generated.PromptNode{Type: generated.PromptNodeTypeSongChart, Attrs: &attrs}
}

func docEmbeddingChart(id uuid.UUID) generated.PromptDocument {
	text := "Play along:"
	paragraph := generated.PromptNode{Type: generated.PromptNodeTypeParagraph, Content: &[]generated.PromptNode{{Type: generated.PromptNodeTypeText, Text: &text}}}
	return generated.PromptDocument{Type: generated.PromptDocumentTypeDoc, Content: []generated.PromptNode{paragraph, songChartPromptNode(id)}}
}

func (w *world) createsArticleEmbedding(title string, chartID uuid.UUID) error {
	doc := docEmbeddingChart(chartID)
	return w.createsContentNodeWithBody(domain.ContentTypeArticle, title, "s", "c", "beginner", nil, &doc)
}

func (w *world) hasArticleEmbeddingChart(_, title, chart string) error {
	id, err := w.chartID(chart)
	if err != nil {
		return err
	}
	if err := w.createsArticleEmbedding(title, id); err != nil {
		return err
	}
	created, ok := w.lastResp.(generated.CreateContentNode201JSONResponse)
	if !ok {
		return fmt.Errorf("setting up %q: %#v (err=%v)", title, w.lastResp, w.lastErr)
	}
	w.charts().articleIDs[title] = created.ContentNodeId
	w.charts().articleCharts[title] = id
	return nil
}

func (w *world) changesArticleTitle(_, title, newTitle string) error {
	id, ok := w.charts().articleIDs[title]
	if !ok {
		return fmt.Errorf("no article %q in this scenario", title)
	}
	doc := docEmbeddingChart(w.charts().articleCharts[title])
	resp, err := w.handler.UpdateContentNode(w.ctx(), generated.UpdateContentNodeRequestObject{
		ContentNodeId: id,
		Body: &generated.UpdateContentNodeRequest{
			Title: newTitle, Classification: w.classificationInputFor("s", "c", "beginner"), RichContent: &doc, LanguageCodes: []string{"en"},
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) addsExpandedContentEmbeddingChart(_, slug, chart string) error {
	id, err := w.chartID(chart)
	if err != nil {
		return err
	}
	doc := docEmbeddingChart(id)
	trigger, hide := 30, 60
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(slug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType: generated.CreateExpandedContentRequestContentTypeRichText, RichContent: &doc,
			TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsExerciseEmbeddingChart(_, chart string) error {
	id, err := w.chartID(chart)
	if err != nil {
		return err
	}
	body := w.exerciseBody("Play the song")
	body.Prompt = docEmbeddingChart(id)
	resp, err := w.handler.CreateExercise(w.ctx(), generated.CreateExerciseRequestObject{Body: &body})
	w.lastResp, w.lastErr = resp, err
	return err
}

// ── Listing ──────────────────────────────────────────────────────────────────

func (w *world) listsSongChartsAs(caller string, status *generated.SongChartStatus, q string) error {
	params := generated.ListSongChartsParams{Status: status}
	if q != "" {
		params.Q = &q
	}
	resp, err := w.handler.ListSongCharts(w.identityCtx(caller), generated.ListSongChartsRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listedCharts() ([]generated.SongChartSummary, error) {
	page, ok := w.lastResp.(generated.ListSongCharts200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("expected a list of song charts, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return page.Items, nil
}

func (w *world) listHoldsOnlyAtRevision(title string, revision int) error {
	items, err := w.listedCharts()
	if err != nil {
		return err
	}
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	if len(items) != 1 || items[0].SongChartId != id {
		return fmt.Errorf("expected only %q, got %+v", title, items)
	}
	p := items[0].PublishedRevision
	if p == nil || p.RevisionNumber != revision || p.Title != title || p.Artist == "" {
		return fmt.Errorf("expected %q published at revision %d with its artist, got %+v", title, revision, p)
	}
	return nil
}

func (w *world) listHoldsWithPublishedTitle(title, publishedTitle string) error {
	items, err := w.listedCharts()
	if err != nil {
		return err
	}
	id, err := w.chartID(title)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.SongChartId == id {
			if item.PublishedRevision == nil || item.PublishedRevision.Title != publishedTitle {
				return fmt.Errorf("expected the published title %q, got %+v", publishedTitle, item.PublishedRevision)
			}
			return nil
		}
	}
	return fmt.Errorf("expected %q in the list, got %+v", title, items)
}

// ── Then ─────────────────────────────────────────────────────────────────────

// embeddedChartIDs lists the songChartId of every songChart node in doc.
func embeddedChartIDs(doc *generated.PromptDocument) []string {
	if doc == nil {
		return nil
	}
	var ids []string
	var walk func(nodes []generated.PromptNode)
	walk = func(nodes []generated.PromptNode) {
		for _, node := range nodes {
			if node.Type == generated.PromptNodeTypeSongChart && node.Attrs != nil {
				if id, ok := (*node.Attrs)["songChartId"].(string); ok {
					ids = append(ids, id)
				}
			}
			if node.Content != nil {
				walk(*node.Content)
			}
		}
	}
	walk(doc.Content)
	return ids
}

func (w *world) expectEmbedded(doc *generated.PromptDocument, chart string) error {
	id, err := w.chartID(chart)
	if err != nil {
		return err
	}
	ids := embeddedChartIDs(doc)
	if len(ids) != 1 || ids[0] != id.String() {
		return fmt.Errorf("expected the song chart %q embedded, got %v", chart, ids)
	}
	return nil
}

func (w *world) articleEmbedsChart(chart string) error {
	switch resp := w.lastResp.(type) {
	case generated.CreateContentNode201JSONResponse:
		return w.expectEmbedded(resp.RichContent, chart)
	case generated.UpdateContentNode200JSONResponse:
		return w.expectEmbedded(resp.RichContent, chart)
	}
	return fmt.Errorf("expected a saved article, got %#v (err=%v)", w.lastResp, w.lastErr)
}

func (w *world) expandedContentEmbedsChart(chart string) error {
	resp, ok := w.lastResp.(generated.CreateExpandedContent201JSONResponse)
	if !ok {
		return fmt.Errorf("expected saved expanded content, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return w.expectEmbedded(resp.RichContent, chart)
}

func (w *world) rejectionNamesSongChartID() error {
	errs, err := w.validationErrors()
	if err != nil {
		return err
	}
	for _, e := range errs {
		if e.Field == "rich_content" && strings.Contains(e.Reason, "songChartId") {
			return nil
		}
	}
	return fmt.Errorf("expected rich_content's songChartId named, got %+v", errs)
}
