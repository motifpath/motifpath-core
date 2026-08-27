package domain

import "time"

// LearningPathItem is a single content node at a given 1-based position
// within a learning path. Title and ContentType are denormalised from the
// referenced ContentNode for display.
type LearningPathItem struct {
	Position      int
	ContentNodeID string
	Title         string
	ContentType   ContentType
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
// 1-based position in the order given. nodes must already be the resolved
// ContentNode for each item — the application layer fetches them to verify
// existence (a content_node_id that doesn't exist is a 400 with field
// "content_node_id", not something this constructor can check on its own)
// and this constructor reuses that same lookup to denormalise Title/
// ContentType rather than requiring a second round-trip.
func NewLearningPath(id, teacherID, title string, nodes []ContentNode, createdAt time.Time) (LearningPath, error) {
	var errs []FieldError

	if title == "" {
		errs = append(errs, FieldError{Field: "title", Reason: "must not be empty"})
	}
	if len(nodes) == 0 {
		errs = append(errs, FieldError{Field: "items", Reason: "must contain at least one item"})
	}

	if len(errs) > 0 {
		return LearningPath{}, &ValidationError{Fields: errs}
	}

	items := make([]LearningPathItem, len(nodes))
	for i, node := range nodes {
		items[i] = LearningPathItem{
			Position:      i + 1,
			ContentNodeID: node.ID,
			Title:         node.Title,
			ContentType:   node.ContentType,
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
