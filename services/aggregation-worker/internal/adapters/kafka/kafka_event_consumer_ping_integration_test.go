//go:build integration

package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestKafkaEventConsumer_Ping backs the readiness probe's kafka_broker check:
// a reachable seed broker pings clean, an address with nothing listening
// reports the failure the probe surfaces as 503.
func TestKafkaEventConsumer_Ping(t *testing.T) {
	broker := setupBroker(t)

	t.Run("reachable broker", func(t *testing.T) {
		consumer := NewKafkaEventConsumer([]string{broker}, &fakeHandler{}, testLogger())
		t.Cleanup(func() { assert.NoError(t, consumer.Close()) })

		assert.NoError(t, consumer.Ping(context.Background()))
	})

	t.Run("unreachable broker", func(t *testing.T) {
		consumer := NewKafkaEventConsumer([]string{"127.0.0.1:1"}, &fakeHandler{}, testLogger())
		t.Cleanup(func() { assert.NoError(t, consumer.Close()) })

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		assert.Error(t, consumer.Ping(ctx))
	})
}
