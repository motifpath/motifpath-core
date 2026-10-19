package repo

import "time"

// outboxDocument mirrors the `publish_outbox` collection schema from
// ADR-012. EventID is stored as the document's _id, giving uniqueness for
// free from MongoDB's primary-key constraint.
type outboxDocument struct {
	EventID          string    `bson:"_id"`
	Status           string    `bson:"status"`
	Attempts         int       `bson:"attempts"`
	LastError        string    `bson:"last_error,omitempty"`
	NextAttemptAt    time.Time `bson:"next_attempt_at,omitempty"`
	ResolutionReason string    `bson:"resolution_reason,omitempty"`
	CreatedAt        time.Time `bson:"created_at"`
	UpdatedAt        time.Time `bson:"updated_at"`
}
