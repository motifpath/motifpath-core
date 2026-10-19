package ports

import "context"

// FeltRatingReader counts felt-rated practice sessions from the Aggregation
// Worker's `practice_sessions` collection, across all students. It is
// read-only from this service's perspective — the worker is the only writer.
type FeltRatingReader interface {
	// FeltRatedSessions returns how many sessions have a felt rating of each
	// of templateKeys that the session practised. A template with none is
	// absent from the result.
	FeltRatedSessions(ctx context.Context, templateKeys []string) (map[string]int, error)
}
