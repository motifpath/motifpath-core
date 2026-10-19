package ports

import "context"

// Pinger reports whether a dependency is currently reachable. Implemented by
// PostgresPinger (the learning-graph store) and MongoCompletionStateReader
// (the completion-state store) for the readiness probe; faked in BDD tests to
// exercise the unreachable-dependency scenarios without a real database.
type Pinger interface {
	Ping(ctx context.Context) error
}
