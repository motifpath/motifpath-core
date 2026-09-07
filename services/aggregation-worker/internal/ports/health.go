package ports

import "context"

// Pinger reports whether a dependency is currently reachable. Implemented by
// KafkaEventConsumer (broker reachability) and MongoCompletionStateRepository
// (MongoDB reachability) to back the readiness probe; faked in tests.
type Pinger interface {
	Ping(ctx context.Context) error
}
