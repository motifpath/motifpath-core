package domain

import "time"

// ExpandedContentType is the format of an ExpandedContent item.
type ExpandedContentType string

const (
	ExpandedContentTypeImage    ExpandedContentType = "image"
	ExpandedContentTypeGif      ExpandedContentType = "gif"
	ExpandedContentTypeRichText ExpandedContentType = "rich_text"
)

// ExpandedContent is an expositive item (image, GIF, or rich content)
// attached to a content node and shown to the student at a specific point
// during content consumption: at a video timestamp for video nodes, or at a
// paragraph position for article nodes. image/gif carry MediaURL; rich_text
// carries RichContent instead — video or audio embeds live inside the rich
// content itself via its media nodes, so there is no separate video/audio
// content type.
type ExpandedContent struct {
	ID                 string
	ContentNodeID      string
	ContentType        ExpandedContentType
	MediaURL           *string
	RichContent        *PromptDocument
	TriggerAtSeconds   *int
	HideAtSeconds      *int
	TriggerAtParagraph *int
	DurationMS         *int
	Caption            *string
	CreatedAt          time.Time
}

// NewExpandedContent validates and constructs an ExpandedContent item. The
// trigger/hide field group required depends on parentType — video nodes use
// TriggerAtSeconds/HideAtSeconds, article nodes use
// TriggerAtParagraph/DurationMS — mixing fields from both groups, or
// omitting the required group, is rejected. Pointers distinguish "field
// omitted" (nil) from "field present with a zero value", which plain ints
// can't — this is what lets a video node's stray trigger_at_paragraph be
// reported distinctly from a genuinely missing trigger_at_seconds.
func NewExpandedContent(
	id, contentNodeID string,
	parentType ContentType,
	contentType ExpandedContentType,
	mediaURL *string,
	richContent *PromptDocument,
	triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS *int,
	caption *string,
	createdAt time.Time,
) (ExpandedContent, error) {
	errs := validateExpandedContentContent(contentType, mediaURL, richContent)

	switch parentType {
	case ContentTypeVideo:
		errs = append(errs, validateVideoTrigger(triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS)...)
	case ContentTypeArticle:
		errs = append(errs, validateArticleTrigger(triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS)...)
	}

	if len(errs) > 0 {
		return ExpandedContent{}, &ValidationError{Fields: errs}
	}

	return ExpandedContent{
		ID:                 id,
		ContentNodeID:      contentNodeID,
		ContentType:        contentType,
		MediaURL:           mediaURL,
		RichContent:        richContent,
		TriggerAtSeconds:   triggerAtSeconds,
		HideAtSeconds:      hideAtSeconds,
		TriggerAtParagraph: triggerAtParagraph,
		DurationMS:         durationMS,
		Caption:            caption,
		CreatedAt:          createdAt,
	}, nil
}

// Update validates and returns a copy of item with its content, trigger/hide
// position, and caption replaced. ID, ContentNodeID, and CreatedAt carry
// over unchanged. parentType is the item's parent content node's type,
// unchanged since content nodes are immutable there — the same trigger/hide
// rules NewExpandedContent enforces apply identically here.
func (item ExpandedContent) Update(
	parentType ContentType,
	contentType ExpandedContentType,
	mediaURL *string,
	richContent *PromptDocument,
	triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS *int,
	caption *string,
) (ExpandedContent, error) {
	errs := validateExpandedContentContent(contentType, mediaURL, richContent)

	switch parentType {
	case ContentTypeVideo:
		errs = append(errs, validateVideoTrigger(triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS)...)
	case ContentTypeArticle:
		errs = append(errs, validateArticleTrigger(triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS)...)
	}

	if len(errs) > 0 {
		return ExpandedContent{}, &ValidationError{Fields: errs}
	}

	updated := item
	updated.ContentType = contentType
	updated.MediaURL = mediaURL
	updated.RichContent = richContent
	updated.TriggerAtSeconds = triggerAtSeconds
	updated.HideAtSeconds = hideAtSeconds
	updated.TriggerAtParagraph = triggerAtParagraph
	updated.DurationMS = durationMS
	updated.Caption = caption
	return updated, nil
}

// validateExpandedContentContent checks contentType and the media_url/
// rich_content pair shared by creation and update: image/gif require
// media_url and forbid rich_content; rich_text requires rich_content and
// forbids media_url.
func validateExpandedContentContent(contentType ExpandedContentType, mediaURL *string, richContent *PromptDocument) []FieldError {
	var errs []FieldError

	switch contentType {
	case ExpandedContentTypeImage, ExpandedContentTypeGif:
		if mediaURL == nil || *mediaURL == "" {
			errs = append(errs, FieldError{Field: "media_url", Reason: "is required when content_type is image or gif"})
		}
		if richContent != nil {
			errs = append(errs, FieldError{Field: "rich_content", Reason: "must be absent when content_type is image or gif"})
		}
	case ExpandedContentTypeRichText:
		if mediaURL != nil {
			errs = append(errs, FieldError{Field: "media_url", Reason: "must be absent when content_type is rich_text"})
		}
		if richContent == nil {
			errs = append(errs, FieldError{Field: "rich_content", Reason: "is required when content_type is rich_text"})
		} else {
			for _, docErr := range validatePromptDocument(*richContent) {
				errs = append(errs, FieldError{Field: "rich_content", Reason: docErr.Reason})
			}
		}
	default:
		errs = append(errs, FieldError{Field: "content_type", Reason: "must be image, gif, or rich_text"})
	}

	return errs
}

func validateVideoTrigger(triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS *int) []FieldError {
	var errs []FieldError

	if triggerAtParagraph != nil || durationMS != nil {
		errs = append(errs, FieldError{Field: "trigger_at_paragraph", Reason: "must be absent for a video content node"})
	}
	if triggerAtSeconds == nil {
		errs = append(errs, FieldError{Field: "trigger_at_seconds", Reason: "is required for a video content node"})
	}
	switch {
	case hideAtSeconds == nil:
		errs = append(errs, FieldError{Field: "hide_at_seconds", Reason: "is required for a video content node"})
	case triggerAtSeconds != nil && *hideAtSeconds <= *triggerAtSeconds:
		errs = append(errs, FieldError{Field: "hide_at_seconds", Reason: "must be greater than trigger_at_seconds"})
	}

	return errs
}

func validateArticleTrigger(triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS *int) []FieldError {
	var errs []FieldError

	if triggerAtSeconds != nil || hideAtSeconds != nil {
		errs = append(errs, FieldError{Field: "trigger_at_seconds", Reason: "must be absent for an article content node"})
	}
	switch {
	case triggerAtParagraph == nil:
		errs = append(errs, FieldError{Field: "trigger_at_paragraph", Reason: "is required for an article content node"})
	case *triggerAtParagraph < 1:
		errs = append(errs, FieldError{Field: "trigger_at_paragraph", Reason: "must be at least 1"})
	}
	switch {
	case durationMS == nil:
		errs = append(errs, FieldError{Field: "duration_ms", Reason: "is required for an article content node"})
	case *durationMS < 1:
		errs = append(errs, FieldError{Field: "duration_ms", Reason: "must be at least 1"})
	}

	return errs
}
