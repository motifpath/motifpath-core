package ports

import "context"

// LastInHandSessionReader reads the items planned in a student's last session
// with an instrument in hand, from the Aggregation Worker's
// `practice_sessions` collection. It is read-only from this service's
// perspective — the worker is the only writer.
type LastInHandSessionReader interface {
	// LastInHandSessionItemKeys returns the item keys planned in the student's
	// latest started session with an instrument in hand, in planned order;
	// none when they have started no such session.
	LastInHandSessionItemKeys(ctx context.Context, studentID string) ([]string, error)
}
