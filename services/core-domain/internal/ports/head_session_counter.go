package ports

import "context"

// HeadSessionCounter counts the practice sessions a student started in the
// head, without an instrument, from the Aggregation Worker's
// `practice_sessions` collection. It is read-only from this service's
// perspective — the worker is the only writer.
type HeadSessionCounter interface {
	// HeadSessionsStarted returns how many sessions in the head the student
	// has started; a session whose start hasn't arrived isn't counted.
	HeadSessionsStarted(ctx context.Context, studentID string) (int, error)
}
