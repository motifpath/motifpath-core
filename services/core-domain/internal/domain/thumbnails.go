package domain

// thumbnailProblems returns a thumbnail_url error when thumbnail is set but
// is not an absolute http or https URL; nil means the item has none.
func thumbnailProblems(thumbnail *string) []FieldError {
	if thumbnail != nil && !isHTTPURL(*thumbnail) {
		return []FieldError{{Field: "thumbnail_url", Reason: "must be an absolute http or https URL"}}
	}
	return nil
}
