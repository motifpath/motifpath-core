//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerExpandedContentSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a video content node "([^"]+)" has expanded content items at seconds (\d+), (\d+), and (\d+)$`, w.videoNodeHasExpandedContentAtSeconds)
	sc.Step(`^an article content node "([^"]+)" has expanded content at paragraphs (\d+), (\d+), and (\d+)$`, w.articleNodeHasExpandedContentAtParagraphs)
	sc.Step(`^an expanded content item "([^"]+)" exists for "([^"]+)"$`, w.putExpandedContent)

	sc.Step(`^"([^"]+)" adds an image to "([^"]+)" with trigger_at_seconds (\d+)\s+and hide_at_seconds (\d+)$`, w.addsVideoExpandedContent(domain.ExpandedContentTypeImage))
	sc.Step(`^"([^"]+)" adds a GIF to "([^"]+)" with trigger_at_seconds (\d+)\s+and hide_at_seconds (\d+)$`, w.addsVideoExpandedContent(domain.ExpandedContentTypeGif))
	sc.Step(`^"([^"]+)" adds three images to "([^"]+)" at different timestamps$`, w.addsThreeExpandedContentItems)
	sc.Step(`^"([^"]+)" lists the expanded content for "([^"]+)"$`, w.listsExpandedContent)
	sc.Step(`^"([^"]+)" adds an image to "([^"]+)" with trigger_at_paragraph (\d+)\s+and duration_ms (\d+)$`, w.addsArticleExpandedContent(domain.ExpandedContentTypeImage))
	sc.Step(`^"([^"]+)" adds a GIF to "([^"]+)" with trigger_at_paragraph (\d+)\s+and duration_ms (\d+)$`, w.addsArticleExpandedContent(domain.ExpandedContentTypeGif))
	sc.Step(`^"([^"]+)" retrieves the expanded content item "([^"]+)"$`, w.retrievesExpandedContent)

	sc.Step(`^"([^"]+)" submits a create expanded content request with the content_type field omitted$`, w.submitsExpandedContentMissingType)
	sc.Step(`^"([^"]+)" submits a create expanded content request with the media_url field omitted$`, w.submitsExpandedContentMissingMediaURL)
	sc.Step(`^"([^"]+)" submits a create expanded content request with trigger_at_paragraph (\d+)\s+and duration_ms (\d+) for a video content node$`, w.submitsExpandedContentArticleFieldsForVideo)
	sc.Step(`^"([^"]+)" submits a create expanded content request with trigger_at_seconds (\d+)\s+and hide_at_seconds (\d+)$`, w.submitsExpandedContentVideoFields)
	sc.Step(`^"([^"]+)" submits a create expanded content request with trigger_at_seconds (\d+)\s+and hide_at_seconds (\d+) for an article content node$`, w.submitsExpandedContentVideoFieldsForArticle)
	sc.Step(`^"([^"]+)" submits a create expanded content request with trigger_at_paragraph (\d+)$`, w.submitsExpandedContentParagraphOnly)
	sc.Step(`^"([^"]+)" submits a create expanded content request with trigger_at_paragraph (\d+)\s+and duration_ms omitted$`, w.submitsExpandedContentParagraphOnly)
	sc.Step(`^"([^"]+)" adds expanded content to a content node ID that does not exist$`, w.addsExpandedContentToMissingNode)
	sc.Step(`^"([^"]+)" retrieves an expanded content item with an ID that does not exist$`, w.retrievesMissingExpandedContent)
	sc.Step(`^"([^"]+)" attempts to add expanded content to "([^"]+)"$`, w.attemptsAddExpandedContent)
	sc.Step(`^an unauthenticated request attempts to add expanded content$`, w.unauthAddsExpandedContent)

	sc.Step(`^the expanded content item is created and assigned a stable identifier$`, w.expandedContentCreated)
	sc.Step(`^the item records "([^"]+)" as its parent content node$`, w.expandedContentRecordsParent)
	sc.Step(`^the items are returned ordered by trigger_at_seconds ascending$`, w.itemsOrderedBySeconds)
	sc.Step(`^the items are returned ordered by trigger_at_paragraph ascending$`, w.itemsOrderedByParagraph)
	sc.Step(`^the response returns the item's type, media URL, trigger, and hide fields$`, w.expandedContentResponseComplete)

	sc.Step(`^an expanded content item "([^"]+)" exists for "([^"]+)" with trigger_at_seconds (\d+) and hide_at_seconds (\d+)$`, w.putExpandedContentAtSeconds)
	sc.Step(`^an expanded content item "([^"]+)" exists for "([^"]+)" at paragraph (\d+)$`, w.putExpandedContentAtParagraph)

	sc.Step(`^"([^"]+)" adds rich text content to "([^"]+)" with trigger_at_seconds (\d+) and hide_at_seconds (\d+)$`, w.addsRichTextExpandedContentAtSeconds)
	sc.Step(`^"([^"]+)" adds rich text content to "([^"]+)" with trigger_at_paragraph (\d+) and duration_ms (\d+)$`, w.addsRichTextExpandedContentAtParagraph)
	sc.Step(`^"([^"]+)" adds rich text content containing an embedded video to "([^"]+)" with trigger_at_seconds (\d+) and hide_at_seconds (\d+)$`, w.addsRichTextExpandedContentWithVideo)
	sc.Step(`^the item's content_type is "([^"]+)"$`, w.expandedContentTypeIs)
	sc.Step(`^the item's rich content contains a video node$`, w.expandedContentRichContentHasVideoNode)

	sc.Step(`^"([^"]+)" updates expanded content item "([^"]+)" with trigger_at_seconds (\d+) and hide_at_seconds (\d+) and caption "([^"]+)"$`, w.updatesExpandedContentAtSecondsWithCaption)
	sc.Step(`^"([^"]+)" updates expanded content item "([^"]+)" with trigger_at_paragraph (\d+) and duration_ms (\d+)$`, w.updatesExpandedContentAtParagraph)
	sc.Step(`^"([^"]+)" deletes expanded content item "([^"]+)"$`, w.deletesExpandedContent)
	sc.Step(`^retrieving expanded content item "([^"]+)" returns not found$`, w.retrievingExpandedContentReturnsNotFound)

	sc.Step(`^"([^"]+)" submits an update expanded content request for "([^"]+)" with trigger_at_seconds (\d+) and hide_at_seconds (\d+)$`, w.submitsUpdateExpandedContentAtSeconds)
	sc.Step(`^"([^"]+)" submits an update expanded content request for "([^"]+)" with the media_url field omitted$`, w.submitsUpdateExpandedContentMissingMediaURL)
	sc.Step(`^"([^"]+)" submits a create expanded content request with content_type "([^"]+)" and the rich_content field omitted$`, w.submitsExpandedContentRichTextMissingRichContent)
	sc.Step(`^"([^"]+)" submits a create expanded content request with content_type "([^"]+)" carrying both media_url and rich_content$`, w.submitsExpandedContentBothMediaAndRich)
	sc.Step(`^"([^"]+)" attempts to update an expanded content item with an ID that does not exist$`, w.attemptsUpdateMissingExpandedContent)
	sc.Step(`^"([^"]+)" attempts to delete an expanded content item with an ID that does not exist$`, w.attemptsDeleteMissingExpandedContent)
	sc.Step(`^"([^"]+)" attempts to update expanded content item "([^"]+)" with caption "([^"]+)"$`, w.attemptsUpdateExpandedContentCaption)
	sc.Step(`^"([^"]+)" attempts to delete expanded content item "([^"]+)"$`, w.attemptsDeleteExpandedContent)
	sc.Step(`^an unauthenticated request attempts to update expanded content item "([^"]+)" with caption "([^"]+)"$`, w.unauthUpdatesExpandedContentCaption)
	sc.Step(`^an unauthenticated request attempts to delete expanded content item "([^"]+)"$`, w.unauthDeletesExpandedContent)

	sc.Step(`^the item's trigger_at_seconds is (\d+)$`, w.expandedContentTriggerAtSecondsIs)
	sc.Step(`^the item's hide_at_seconds is (\d+)$`, w.expandedContentHideAtSecondsIs)
	sc.Step(`^the item's caption is "([^"]+)"$`, w.expandedContentCaptionIs)
	sc.Step(`^the item's trigger_at_paragraph is (\d+)$`, w.expandedContentTriggerAtParagraphIs)
	sc.Step(`^the item's duration_ms is (\d+)$`, w.expandedContentDurationMsIs)
}

func (w *world) putExpandedContentAtSeconds(slug, nodeSlug, triggerStr, hideStr string) error {
	trigger, err := parseInt(triggerStr)
	if err != nil {
		return err
	}
	hide, err := parseInt(hideStr)
	if err != nil {
		return err
	}
	if err := w.putContentNodeIfAbsent(nodeSlug, domain.ContentTypeVideo); err != nil {
		return err
	}
	w.putExpandedContentRaw(slug, nodeSlug, &trigger, &hide, nil, nil)
	return nil
}

func (w *world) putExpandedContentAtParagraph(slug, nodeSlug, paragraphStr string) error {
	paragraph, err := parseInt(paragraphStr)
	if err != nil {
		return err
	}
	duration := 5000
	if err := w.putContentNodeIfAbsent(nodeSlug, domain.ContentTypeArticle); err != nil {
		return err
	}
	w.putExpandedContentRaw(slug, nodeSlug, nil, nil, &paragraph, &duration)
	return nil
}

// putContentNodeIfAbsent ensures nodeSlug exists, without clobbering it if a
// preceding Given step already created it (e.g. explicitly, or with a
// different content type than contentType would default to).
func (w *world) putContentNodeIfAbsent(nodeSlug string, contentType domain.ContentType) error {
	if _, err := w.nodes.GetByID(w.ctx(), nodeID(nodeSlug).String()); err == nil {
		return nil
	}
	return w.putContentNode(nodeSlug, contentType)
}

// richTextDoc builds a minimal valid generated.PromptDocument for rich_text
// expanded content, optionally embedding a video PromptNode.
func richTextDoc(withVideo bool) generated.PromptDocument {
	content := []generated.PromptNode{{Type: generated.PromptNodeTypeParagraph, Content: &[]generated.PromptNode{
		{Type: generated.PromptNodeTypeText, Text: strPtr("Rich text content.")},
	}}}
	if withVideo {
		content = append(content, generated.PromptNode{Type: generated.PromptNodeTypeVideo})
	}
	return generated.PromptDocument{Type: generated.Doc, Content: content}
}

func (w *world) addsRichTextExpandedContentAtSeconds(name, nodeSlug, triggerStr, hideStr string) error {
	trigger, err := parseInt(triggerStr)
	if err != nil {
		return err
	}
	hide, err := parseInt(hideStr)
	if err != nil {
		return err
	}
	rich := richTextDoc(false)
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType:      generated.CreateExpandedContentRequestContentTypeRichText,
			RichContent:      &rich,
			TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) addsRichTextExpandedContentAtParagraph(name, nodeSlug, paragraphStr, durationStr string) error {
	paragraph, err := parseInt(paragraphStr)
	if err != nil {
		return err
	}
	duration, err := parseInt(durationStr)
	if err != nil {
		return err
	}
	rich := richTextDoc(false)
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType:        generated.CreateExpandedContentRequestContentTypeRichText,
			RichContent:        &rich,
			TriggerAtParagraph: &paragraph, DurationMs: &duration,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) addsRichTextExpandedContentWithVideo(name, nodeSlug, triggerStr, hideStr string) error {
	trigger, err := parseInt(triggerStr)
	if err != nil {
		return err
	}
	hide, err := parseInt(hideStr)
	if err != nil {
		return err
	}
	rich := richTextDoc(true)
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType:      generated.CreateExpandedContentRequestContentTypeRichText,
			RichContent:      &rich,
			TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) expandedContentTypeIs(want string) error {
	resp, ok := w.lastResp.(generated.CreateExpandedContent201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if string(resp.ContentType) != want {
		return fmt.Errorf("expected content_type %q, got %q", want, resp.ContentType)
	}
	return nil
}

func (w *world) expandedContentRichContentHasVideoNode() error {
	resp, ok := w.lastResp.(generated.CreateExpandedContent201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.RichContent == nil {
		return fmt.Errorf("expected rich_content to be set, got %+v", resp)
	}
	for _, node := range resp.RichContent.Content {
		if node.Type == generated.PromptNodeTypeVideo {
			return nil
		}
	}
	return fmt.Errorf("expected rich_content to contain a video node, got %+v", resp.RichContent)
}

func (w *world) updatesExpandedContentAtSecondsWithCaption(name, slug, triggerStr, hideStr, caption string) error {
	trigger, err := parseInt(triggerStr)
	if err != nil {
		return err
	}
	hide, err := parseInt(hideStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.UpdateExpandedContent(w.ctx(), generated.UpdateExpandedContentRequestObject{
		ExpandedContentId: expandedID(slug),
		Body: &generated.UpdateExpandedContentRequest{
			ContentType:      generated.UpdateExpandedContentRequestContentTypeImage,
			MediaUrl:         strPtr("https://cdn.example.com/media.png"),
			TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
			Caption: &caption,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) updatesExpandedContentAtParagraph(name, slug, paragraphStr, durationStr string) error {
	paragraph, err := parseInt(paragraphStr)
	if err != nil {
		return err
	}
	duration, err := parseInt(durationStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.UpdateExpandedContent(w.ctx(), generated.UpdateExpandedContentRequestObject{
		ExpandedContentId: expandedID(slug),
		Body: &generated.UpdateExpandedContentRequest{
			ContentType:        generated.UpdateExpandedContentRequestContentTypeImage,
			MediaUrl:           strPtr("https://cdn.example.com/media.png"),
			TriggerAtParagraph: &paragraph, DurationMs: &duration,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) deletesExpandedContent(name, slug string) error {
	resp, err := w.handler.DeleteExpandedContent(w.ctx(), generated.DeleteExpandedContentRequestObject{ExpandedContentId: expandedID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievingExpandedContentReturnsNotFound(slug string) error {
	resp, err := w.handler.GetExpandedContent(w.ctx(), generated.GetExpandedContentRequestObject{ExpandedContentId: expandedID(slug)})
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.GetExpandedContent404JSONResponse); !ok {
		return fmt.Errorf("expected a 404 response, got %#v", resp)
	}
	return nil
}

func (w *world) submitsUpdateExpandedContentAtSeconds(name, slug, triggerStr, hideStr string) error {
	trigger, err := parseInt(triggerStr)
	if err != nil {
		return err
	}
	hide, err := parseInt(hideStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.UpdateExpandedContent(w.ctx(), generated.UpdateExpandedContentRequestObject{
		ExpandedContentId: expandedID(slug),
		Body: &generated.UpdateExpandedContentRequest{
			ContentType:      generated.UpdateExpandedContentRequestContentTypeImage,
			MediaUrl:         strPtr("https://cdn.example.com/media.png"),
			TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsUpdateExpandedContentMissingMediaURL(name, slug string) error {
	resp, err := w.handler.UpdateExpandedContent(w.ctx(), generated.UpdateExpandedContentRequestObject{
		ExpandedContentId: expandedID(slug),
		Body: &generated.UpdateExpandedContentRequest{
			ContentType: generated.UpdateExpandedContentRequestContentTypeImage,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExpandedContentRichTextMissingRichContent(name, contentType string) error {
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(w.lastNodeSlug),
		Body:          &generated.CreateExpandedContentRequest{ContentType: generated.CreateExpandedContentRequestContentType(contentType)},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExpandedContentBothMediaAndRich(name, contentType string) error {
	rich := richTextDoc(false)
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(w.lastNodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType: generated.CreateExpandedContentRequestContentType(contentType),
			MediaUrl:    strPtr("https://cdn.example.com/media.png"),
			RichContent: &rich,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsUpdateMissingExpandedContent(string) error {
	resp, err := w.handler.UpdateExpandedContent(w.ctx(), generated.UpdateExpandedContentRequestObject{
		ExpandedContentId: deterministicUUID("expanded", "does-not-exist"),
		Body: &generated.UpdateExpandedContentRequest{
			ContentType: generated.UpdateExpandedContentRequestContentTypeImage,
			MediaUrl:    strPtr("https://cdn.example.com/media.png"),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsDeleteMissingExpandedContent(string) error {
	resp, err := w.handler.DeleteExpandedContent(w.ctx(), generated.DeleteExpandedContentRequestObject{ExpandedContentId: deterministicUUID("expanded", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsUpdateExpandedContentCaption(name, slug, caption string) error {
	resp, err := w.handler.UpdateExpandedContent(w.ctx(), generated.UpdateExpandedContentRequestObject{
		ExpandedContentId: expandedID(slug),
		Body: &generated.UpdateExpandedContentRequest{
			ContentType: generated.UpdateExpandedContentRequestContentTypeImage,
			MediaUrl:    strPtr("https://cdn.example.com/media.png"),
			Caption:     &caption,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsDeleteExpandedContent(name, slug string) error {
	return w.deletesExpandedContent(name, slug)
}

func (w *world) unauthUpdatesExpandedContentCaption(slug, caption string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsUpdateExpandedContentCaption("", slug, caption)
}

func (w *world) unauthDeletesExpandedContent(slug string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.deletesExpandedContent("", slug)
}

func (w *world) expandedContentTriggerAtSecondsIs(wantStr string) error {
	want, err := parseInt(wantStr)
	if err != nil {
		return err
	}
	resp, ok := w.lastResp.(generated.UpdateExpandedContent200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.TriggerAtSeconds == nil || *resp.TriggerAtSeconds != want {
		return fmt.Errorf("expected trigger_at_seconds %d, got %+v", want, resp.TriggerAtSeconds)
	}
	return nil
}

func (w *world) expandedContentHideAtSecondsIs(wantStr string) error {
	want, err := parseInt(wantStr)
	if err != nil {
		return err
	}
	resp, ok := w.lastResp.(generated.UpdateExpandedContent200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.HideAtSeconds == nil || *resp.HideAtSeconds != want {
		return fmt.Errorf("expected hide_at_seconds %d, got %+v", want, resp.HideAtSeconds)
	}
	return nil
}

func (w *world) expandedContentCaptionIs(want string) error {
	resp, ok := w.lastResp.(generated.UpdateExpandedContent200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.Caption == nil || *resp.Caption != want {
		return fmt.Errorf("expected caption %q, got %+v", want, resp.Caption)
	}
	return nil
}

func (w *world) expandedContentTriggerAtParagraphIs(wantStr string) error {
	want, err := parseInt(wantStr)
	if err != nil {
		return err
	}
	resp, ok := w.lastResp.(generated.UpdateExpandedContent200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.TriggerAtParagraph == nil || *resp.TriggerAtParagraph != want {
		return fmt.Errorf("expected trigger_at_paragraph %d, got %+v", want, resp.TriggerAtParagraph)
	}
	return nil
}

func (w *world) expandedContentDurationMsIs(wantStr string) error {
	want, err := parseInt(wantStr)
	if err != nil {
		return err
	}
	resp, ok := w.lastResp.(generated.UpdateExpandedContent200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.DurationMs == nil || *resp.DurationMs != want {
		return fmt.Errorf("expected duration_ms %d, got %+v", want, resp.DurationMs)
	}
	return nil
}

func (w *world) putExpandedContentRaw(slug, nodeSlug string, triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS *int) {
	w.expanded.put(domain.ExpandedContent{
		ID:                 expandedID(slug).String(),
		ContentNodeID:      nodeID(nodeSlug).String(),
		ContentType:        domain.ExpandedContentTypeImage,
		MediaURL:           strPtr("https://cdn.example.com/" + slug + ".png"),
		TriggerAtSeconds:   triggerAtSeconds,
		HideAtSeconds:      hideAtSeconds,
		TriggerAtParagraph: triggerAtParagraph,
		DurationMS:         durationMS,
		CreatedAt:          fixedNow,
	})
}

func (w *world) videoNodeHasExpandedContentAtSeconds(nodeSlug, s1, s2, s3 string) error {
	if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
		return err
	}
	for i, s := range []string{s1, s2, s3} {
		seconds, err := parseInt(s)
		if err != nil {
			return err
		}
		hide := seconds + 10
		w.putExpandedContentRaw(fmt.Sprintf("%s-item-%d", nodeSlug, i), nodeSlug, &seconds, &hide, nil, nil)
	}
	return nil
}

func (w *world) articleNodeHasExpandedContentAtParagraphs(nodeSlug, p1, p2, p3 string) error {
	if err := w.putContentNode(nodeSlug, domain.ContentTypeArticle); err != nil {
		return err
	}
	for i, p := range []string{p1, p2, p3} {
		paragraph, err := parseInt(p)
		if err != nil {
			return err
		}
		duration := 5000
		w.putExpandedContentRaw(fmt.Sprintf("%s-item-%d", nodeSlug, i), nodeSlug, nil, nil, &paragraph, &duration)
	}
	return nil
}

func (w *world) putExpandedContent(slug, nodeSlug string) error {
	seconds, hide := 90, 100
	if err := w.putContentNodeIfAbsent(nodeSlug, domain.ContentTypeVideo); err != nil {
		return err
	}
	w.putExpandedContentRaw(slug, nodeSlug, &seconds, &hide, nil, nil)
	return nil
}

func (w *world) addsVideoExpandedContent(contentType domain.ExpandedContentType) func(name, nodeSlug, triggerStr, hideStr string) error {
	return func(name, nodeSlug, triggerStr, hideStr string) error {
		trigger, err := parseInt(triggerStr)
		if err != nil {
			return err
		}
		hide, err := parseInt(hideStr)
		if err != nil {
			return err
		}
		resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
			ContentNodeId: nodeID(nodeSlug),
			Body: &generated.CreateExpandedContentRequest{
				ContentType:      generated.CreateExpandedContentRequestContentType(contentType),
				MediaUrl:         strPtr("https://cdn.example.com/media.png"),
				TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
			},
		})
		w.lastResp, w.lastErr = resp, err
		return err
	}
}

func (w *world) addsArticleExpandedContent(contentType domain.ExpandedContentType) func(name, nodeSlug, paragraphStr, durationStr string) error {
	return func(name, nodeSlug, paragraphStr, durationStr string) error {
		paragraph, err := parseInt(paragraphStr)
		if err != nil {
			return err
		}
		duration, err := parseInt(durationStr)
		if err != nil {
			return err
		}
		resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
			ContentNodeId: nodeID(nodeSlug),
			Body: &generated.CreateExpandedContentRequest{
				ContentType:        generated.CreateExpandedContentRequestContentType(contentType),
				MediaUrl:           strPtr("https://cdn.example.com/media.png"),
				TriggerAtParagraph: &paragraph, DurationMs: &duration,
			},
		})
		w.lastResp, w.lastErr = resp, err
		return err
	}
}

func (w *world) addsThreeExpandedContentItems(name, nodeSlug string) error {
	w.multiCreateIDs = nil
	for i := 0; i < 3; i++ {
		trigger := 10 + i*20
		hide := trigger + 5
		resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
			ContentNodeId: nodeID(nodeSlug),
			Body: &generated.CreateExpandedContentRequest{
				ContentType:      generated.CreateExpandedContentRequestContentTypeImage,
				MediaUrl:         strPtr(fmt.Sprintf("https://cdn.example.com/media-%d.png", i)),
				TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
			},
		})
		w.lastResp, w.lastErr = resp, err
		if err != nil {
			return err
		}
		created, ok := resp.(generated.CreateExpandedContent201JSONResponse)
		if !ok {
			return fmt.Errorf("expected a 201 response, got %#v", resp)
		}
		w.multiCreateIDs = append(w.multiCreateIDs, created.ExpandedContentId)
	}
	return nil
}

func (w *world) listsExpandedContent(name, nodeSlug string) error {
	resp, err := w.handler.ListExpandedContent(w.ctx(), generated.ListExpandedContentRequestObject{ContentNodeId: nodeID(nodeSlug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesExpandedContent(name, slug string) error {
	resp, err := w.handler.GetExpandedContent(w.ctx(), generated.GetExpandedContentRequestObject{ExpandedContentId: expandedID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExpandedContentMissingType(string) error {
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(w.lastNodeSlug),
		Body:          &generated.CreateExpandedContentRequest{MediaUrl: strPtr("https://cdn.example.com/media.png")},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExpandedContentMissingMediaURL(string) error {
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(w.lastNodeSlug),
		Body:          &generated.CreateExpandedContentRequest{ContentType: generated.CreateExpandedContentRequestContentTypeImage},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExpandedContentArticleFieldsForVideo(name, paragraphStr, durationStr string) error {
	paragraph, err := parseInt(paragraphStr)
	if err != nil {
		return err
	}
	duration, err := parseInt(durationStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(w.lastNodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: strPtr("https://cdn.example.com/media.png"),
			TriggerAtParagraph: &paragraph, DurationMs: &duration,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExpandedContentVideoFields(name, triggerStr, hideStr string) error {
	trigger, err := parseInt(triggerStr)
	if err != nil {
		return err
	}
	hide, err := parseInt(hideStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(w.lastNodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: strPtr("https://cdn.example.com/media.png"),
			TriggerAtSeconds: &trigger, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsExpandedContentVideoFieldsForArticle(name, triggerStr, hideStr string) error {
	return w.submitsExpandedContentVideoFields(name, triggerStr, hideStr)
}

func (w *world) submitsExpandedContentParagraphOnly(name, paragraphStr string) error {
	paragraph, err := parseInt(paragraphStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(w.lastNodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: strPtr("https://cdn.example.com/media.png"),
			TriggerAtParagraph: &paragraph,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) addsExpandedContentToMissingNode(string) error {
	seconds, hide := 90, 100
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: deterministicUUID("node", "does-not-exist"),
		Body: &generated.CreateExpandedContentRequest{
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: strPtr("https://cdn.example.com/media.png"),
			TriggerAtSeconds: &seconds, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesMissingExpandedContent(string) error {
	resp, err := w.handler.GetExpandedContent(w.ctx(), generated.GetExpandedContentRequestObject{ExpandedContentId: deterministicUUID("expanded", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsAddExpandedContent(name, nodeSlug string) error {
	seconds, hide := 90, 100
	resp, err := w.handler.CreateExpandedContent(w.ctx(), generated.CreateExpandedContentRequestObject{
		ContentNodeId: nodeID(nodeSlug),
		Body: &generated.CreateExpandedContentRequest{
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: strPtr("https://cdn.example.com/media.png"),
			TriggerAtSeconds: &seconds, HideAtSeconds: &hide,
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthAddsExpandedContent() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsAddExpandedContent("", "intro-to-triads")
}

func (w *world) expandedContentCreated() error {
	if _, ok := w.lastResp.(generated.CreateExpandedContent201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) expandedContentRecordsParent(nodeSlug string) error {
	resp, ok := w.lastResp.(generated.CreateExpandedContent201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.ContentNodeId != nodeID(nodeSlug) {
		return fmt.Errorf("expected content_node_id %s, got %s", nodeID(nodeSlug), resp.ContentNodeId)
	}
	return nil
}

func (w *world) itemsOrderedBySeconds() error {
	resp, ok := w.lastResp.(generated.ListExpandedContent200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	prev := -1
	for _, item := range resp.Items {
		if item.TriggerAtSeconds == nil {
			return fmt.Errorf("item %s had no trigger_at_seconds", item.ExpandedContentId)
		}
		if *item.TriggerAtSeconds < prev {
			return fmt.Errorf("items not in ascending trigger_at_seconds order: %+v", resp.Items)
		}
		prev = *item.TriggerAtSeconds
	}
	return nil
}

func (w *world) itemsOrderedByParagraph() error {
	resp, ok := w.lastResp.(generated.ListExpandedContent200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	prev := -1
	for _, item := range resp.Items {
		if item.TriggerAtParagraph == nil {
			return fmt.Errorf("item %s had no trigger_at_paragraph", item.ExpandedContentId)
		}
		if *item.TriggerAtParagraph < prev {
			return fmt.Errorf("items not in ascending trigger_at_paragraph order: %+v", resp.Items)
		}
		prev = *item.TriggerAtParagraph
	}
	return nil
}

func (w *world) expandedContentResponseComplete() error {
	resp, ok := w.lastResp.(generated.GetExpandedContent200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.ContentType == "" || resp.MediaUrl == nil || *resp.MediaUrl == "" {
		return fmt.Errorf("expected a fully populated expanded content item, got %+v", resp)
	}
	if resp.TriggerAtSeconds == nil && resp.TriggerAtParagraph == nil {
		return fmt.Errorf("expected a trigger field to be set, got %+v", resp)
	}
	return nil
}
