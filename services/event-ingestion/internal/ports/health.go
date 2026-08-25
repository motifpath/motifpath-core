package ports

import "context"

// Pinger reports whether a dependency is currently reachable. Implemented by
// MongoEventRepository and KafkaEventPublisher for the readiness probe; faked in
// BDD tests to exercise the down/not-yet-initialised scenarios without a real
// MongoDB or Kafka connection.
type Pinger interface {
	Ping(ctx context.Context) error
}
