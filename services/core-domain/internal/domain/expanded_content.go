package domain

import "time"

// ExpandedContentType is the media format of an ExpandedContent item.
type ExpandedContentType string

const (
	ExpandedContentTypeImage ExpandedContentType = "image"
	ExpandedContentTypeGif   ExpandedContentType = "gif"
)

// ExpandedContent is an expositive media item (image or GIF) attached to a
// content node and shown to the student at a specific point during content
// consumption: at a video timestamp for video nodes, or at a paragraph
// position for article nodes.
type ExpandedContent struct {
	ID                 string
	ContentNodeID      string
	ContentType        ExpandedContentType
	MediaURL           string
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
	mediaURL string,
	triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS *int,
	caption *string,
	createdAt time.Time,
) (ExpandedContent, error) {
	var errs []FieldError

	switch contentType {
	case ExpandedContentTypeImage, ExpandedContentTypeGif:
	default:
		errs = append(errs, FieldError{Field: "content_type", Reason: "must be image or gif"})
	}
	if mediaURL == "" {
		errs = append(errs, FieldError{Field: "media_url", Reason: "must not be empty"})
	}

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
		TriggerAtSeconds:   triggerAtSeconds,
		HideAtSeconds:      hideAtSeconds,
		TriggerAtParagraph: triggerAtParagraph,
		DurationMS:         durationMS,
		Caption:            caption,
		CreatedAt:          createdAt,
	}, nil
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
