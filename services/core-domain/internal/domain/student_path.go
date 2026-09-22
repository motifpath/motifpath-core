package domain

import "time"

// StudentPath is a student's own copy of a learning path template's items,
// created by copying the template at assign or enrollment time.
// Independently editable afterwards and never affected by later changes to
// the template it was copied from.
type StudentPath struct {
	ID                       string
	StudentID                string
	SourceTemplateID         string
	Title                    string
	AssignedBy               string
	AssignedAt               time.Time
	ArchivedAt               *time.Time
	SourceCourseEnrollmentID *string
	CourseCheckpointPosition *int
	Items                    []StudentPathItemRecord
}

// StudentPathItemRecord is one content node copied into a StudentPath at
// assign time — the persisted counterpart of the derived StudentPathItem
// view BuildStudentPathItems produces below.
type StudentPathItemRecord struct {
	Position             int
	ContentNodeID        string
	ContentNodeVersionID string
	SectionLabel         *string
}

// NewStudentPathFromTemplate copies template's items into a new,
// independently-editable StudentPath owned by studentID.
// contentNodeVersionIDs must carry one entry per content_node_id
// referenced by template.Items — the id of that node's latest published
// ContentNodeVersion at this instant. The application layer resolves every
// one before calling this and rejects the whole request (not found) if any
// node has never been published, rather than reaching this constructor
// with a partial map.
func NewStudentPathFromTemplate(id, studentID string, template LearningPath, assignedBy string, assignedAt time.Time, contentNodeVersionIDs map[string]string) (StudentPath, error) {
	items := make([]StudentPathItemRecord, len(template.Items))
	for i, item := range template.Items {
		versionID, ok := contentNodeVersionIDs[item.ContentNodeID]
		if !ok {
			return StudentPath{}, NewValidationError("learning_path_id", "no published content node version resolved for content node "+item.ContentNodeID)
		}
		items[i] = StudentPathItemRecord{
			Position:             item.Position,
			ContentNodeID:        item.ContentNodeID,
			ContentNodeVersionID: versionID,
			SectionLabel:         item.SectionLabel,
		}
	}

	return StudentPath{
		ID:               id,
		StudentID:        studentID,
		SourceTemplateID: template.ID,
		Title:            template.Title,
		AssignedBy:       assignedBy,
		AssignedAt:       assignedAt,
		Items:            items,
	}, nil
}

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
	// nil means the item is ungrouped.
	SectionLabel *string
	// ContentNodeVersionID is the ContentNodeVersion this item was pinned
	// to at copy time. Left blank by BuildStudentPathItems itself (it has
	// no version information to work from); the caller fills it in from
	// the StudentPathItemRecord this view item was derived from.
	ContentNodeVersionID string
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
// langLocked is keyed by content_node_id and holds true for items the
// caller's resolved locale cannot access — no language edge on the item's
// ContentNode (or a required Exercise) matches the locale, and neither has
// an "any" edge. This composes with, not replaces, prerequisite-based
// locking: an item not yet completed is locked if either reason applies.
// It never revokes access to an item the student has already completed,
// even if the locale check would otherwise fail it — completing a node
// while it was available must not later hide it behind a locale change.
//
// An item is locked unless every earlier item in the path is completed —
// position 1 is never locked by the prerequisite rule (there is no earlier
// item to block it), though it can still be locked by langLocked.
// current_position is the 1-based position of the first item that is not
// completed, or the path's last position if every item is completed.
func BuildStudentPathItems(items []LearningPathItem, raw map[string]CompletionStatus, langLocked map[string]bool) ([]StudentPathItem, int) {
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
		if status != CompletionStatusCompleted && langLocked[item.ContentNodeID] {
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
