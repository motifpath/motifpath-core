package ports

import "context"

// EventConsumer drives the Kafka subscribe/commit loop for as long as ctx is
// live, returning nil on a clean shutdown (ctx cancelled) or an error on an
// unrecoverable read failure. Implemented by adapters/kafka.KafkaEventConsumer;
// exists as a port so cmd/main.go wires against an interface, not a concrete
// adapter type.
type EventConsumer interface {
	Run(ctx context.Context) error
}
