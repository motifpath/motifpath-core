package domain

import (
	"strings"
	"time"
)

// LearningPathItem is a single content node at a given 1-based position
// within a learning path. Title and ContentType are denormalised from the
// referenced ContentNode for display.
type LearningPathItem struct {
	Position      int
	ContentNodeID string
	Title         string
	ContentType   ContentType
	// SectionLabel optionally groups this item with its immediate
	// neighbours under a named section in the path view. Nil means the
	// item is ungrouped. It names a competency or skill area, never a time
	// period or schedule.
	SectionLabel *string
}

// NewLearningPathItem is one resolved item the caller wants in a new
// learning path: the content node it points at (already looked up by the
// application layer to verify existence and to denormalise Title/
// ContentType) plus its optional section label.
type NewLearningPathItem struct {
	Node         ContentNode
	SectionLabel *string
}

// LearningPath is an ordered sequence of content nodes assigned to students
// as a structured curriculum.
type LearningPath struct {
	ID        string
	TeacherID string
	Title     string
	Items     []LearningPathItem
	CreatedAt time.Time
}

// NewLearningPath validates title and items and assigns each item its
// 1-based position in the order given. Each item's Node must already be the
// resolved ContentNode — the application layer fetches them to verify
// existence (a content_node_id that doesn't exist is a 400 with field
// "content_node_id", not something this constructor can check on its own)
// and this constructor reuses that same lookup to denormalise Title/
// ContentType rather than requiring a second round-trip.
//
// Section labels are normalised here (see normaliseSectionLabel) so the
// stored path is the single source of truth about which items belong to the
// same section — consumers must not have to re-derive that by trimming.
func NewLearningPath(id, teacherID, title string, pathItems []NewLearningPathItem, createdAt time.Time) (LearningPath, error) {
	var errs []FieldError

	if title == "" {
		errs = append(errs, FieldError{Field: "title", Reason: "must not be empty"})
	}
	if len(pathItems) == 0 {
		errs = append(errs, FieldError{Field: "items", Reason: "must contain at least one item"})
	}

	if len(errs) > 0 {
		return LearningPath{}, &ValidationError{Fields: errs}
	}

	items := make([]LearningPathItem, len(pathItems))
	for i, pathItem := range pathItems {
		items[i] = LearningPathItem{
			Position:      i + 1,
			ContentNodeID: pathItem.Node.ID,
			Title:         pathItem.Node.Title,
			ContentType:   pathItem.Node.ContentType,
			SectionLabel:  normaliseSectionLabel(pathItem.SectionLabel),
		}
	}

	return LearningPath{
		ID:        id,
		TeacherID: teacherID,
		Title:     title,
		Items:     items,
		CreatedAt: createdAt,
	}, nil
}

// normaliseSectionLabel trims a section label and collapses a blank one to
// nil. Two items naming the same section must compare equal regardless of
// incidental whitespace, and a label that is empty or whitespace-only names
// no section at all — storing it as present-but-blank would render an empty
// heading downstream.
func normaliseSectionLabel(label *string) *string {
	if label == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*label)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
