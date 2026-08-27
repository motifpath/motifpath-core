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
}

func (w *world) putExpandedContentRaw(slug, nodeSlug string, triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS *int) {
	w.expanded.put(domain.ExpandedContent{
		ID:                 expandedID(slug).String(),
		ContentNodeID:      nodeID(nodeSlug).String(),
		ContentType:        domain.ExpandedContentTypeImage,
		MediaURL:           "https://cdn.example.com/" + slug + ".png",
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
				MediaUrl:         "https://cdn.example.com/media.png",
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
				MediaUrl:           "https://cdn.example.com/media.png",
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
				MediaUrl:         fmt.Sprintf("https://cdn.example.com/media-%d.png", i),
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
		Body:          &generated.CreateExpandedContentRequest{MediaUrl: "https://cdn.example.com/media.png"},
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
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: "https://cdn.example.com/media.png",
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
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: "https://cdn.example.com/media.png",
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
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: "https://cdn.example.com/media.png",
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
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: "https://cdn.example.com/media.png",
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
			ContentType: generated.CreateExpandedContentRequestContentTypeImage, MediaUrl: "https://cdn.example.com/media.png",
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
	if resp.ContentType == "" || resp.MediaUrl == "" {
		return fmt.Errorf("expected a fully populated expanded content item, got %+v", resp)
	}
	if resp.TriggerAtSeconds == nil && resp.TriggerAtParagraph == nil {
		return fmt.Errorf("expected a trigger field to be set, got %+v", resp)
	}
	return nil
}
