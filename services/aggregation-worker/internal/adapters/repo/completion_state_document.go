package repo

import "time"

// completionStateDocument mirrors the `aggregates` collection schema fixed by
// ADR-011: one document per (student_id, content_node_id) pair.
type completionStateDocument struct {
	StudentID     string    `bson:"student_id"`
	ContentNodeID string    `bson:"content_node_id"`
	Status        string    `bson:"status"`
	UpdatedAt     time.Time `bson:"updated_at"`
	// StartedAt is when the student first started the node; absent until a
	// lesson event for it has arrived.
	StartedAt *time.Time `bson:"started_at,omitempty"`
}
