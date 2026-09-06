package domain

// CompletionStatus is a student's progress on a single content node. The
// three non-locked values mirror exactly what the Aggregation Worker writes
// to MongoDB `aggregates` per ADR-011 (see CompletionStateReader) —
// CompletionStatusLocked is never stored anywhere; it exists only as an
// output of BuildStudentPathItems below, computed from path position.
type CompletionStatus string

const (
	CompletionStatusNotStarted CompletionStatus = "not_started"
	CompletionStatusInProgress CompletionStatus = "in_progress"
	CompletionStatusCompleted  CompletionStatus = "completed"
	CompletionStatusLocked     CompletionStatus = "locked"
)

// StudentPathItem is a LearningPathItem enriched with the student's current
// progress on it.
type StudentPathItem struct {
	Position      int
	ContentNodeID string
	Title         string
	ContentType   ContentType
	Status        CompletionStatus
	// SectionLabel is carried through unchanged from the LearningPathItem;
	// empty means the item is ungrouped.
	SectionLabel string
}

// BuildStudentPathItems combines a learning path's ordered items with a
// student's raw per-node completion state and derives the locked/unlocked
// view the SPA renders.
//
// raw is keyed by content_node_id and holds only the three states the
// Aggregation Worker can produce (not_started/in_progress/completed); a
// content node with no entry has never been touched by the student and is
// treated as not_started, mirroring CompletionStateRepository.GetStatus's
// own found=false handling in the aggregation-worker.
//
// An item is locked unless every earlier item in the path is completed —
// position 1 is never locked by this rule (there is no earlier item to
// block it). current_position is the 1-based position of the first item
// that is not completed, or the path's last position if every item is
// completed.
func BuildStudentPathItems(items []LearningPathItem, raw map[string]CompletionStatus) ([]StudentPathItem, int) {
	result := make([]StudentPathItem, len(items))
	priorCompleted := true
	currentPosition := 1
	foundCurrent := false

	for i, item := range items {
		status, ok := raw[item.ContentNodeID]
		if !ok {
			status = CompletionStatusNotStarted
		}
		if !priorCompleted {
			status = CompletionStatusLocked
		}

		result[i] = StudentPathItem{
			Position:      item.Position,
			ContentNodeID: item.ContentNodeID,
			Title:         item.Title,
			ContentType:   item.ContentType,
			Status:        status,
			SectionLabel:  item.SectionLabel,
		}

		if !foundCurrent && status != CompletionStatusCompleted {
			currentPosition = item.Position
			foundCurrent = true
		}
		priorCompleted = priorCompleted && status == CompletionStatusCompleted
	}

	if !foundCurrent && len(items) > 0 {
		currentPosition = items[len(items)-1].Position
	}

	return result, currentPosition
}
