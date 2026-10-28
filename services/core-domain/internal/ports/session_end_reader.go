package ports

import "context"

// SessionEndReader reads whether the Aggregation Worker has recorded a
// session's end, from its `practice_sessions` collection. It is read-only
// from this service's perspective — the worker is the only writer.
type SessionEndReader interface {
	// SessionEnded reports whether the end of the student's session
	// sessionID has been recorded; false for a session of another student,
	// or one never recorded.
	SessionEnded(ctx context.Context, studentID, sessionID string) (bool, error)
}
