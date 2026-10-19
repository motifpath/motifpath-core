package domain

import "time"

// OutboxStatus tracks the delivery state of a single tracking event's Kafka
// publish, per ADR-012.
type OutboxStatus string

const (
	OutboxStatusPending          OutboxStatus = "pending"
	OutboxStatusPublished        OutboxStatus = "published"
	OutboxStatusDead             OutboxStatus = "dead"
	OutboxStatusResolvedManually OutboxStatus = "resolved_manually"
)

const (
	// MaxPublishAttempts is the number of failed attempts (including the
	// initial one) after which an entry stops being retried automatically
	// and becomes dead. Per ADR-012.
	MaxPublishAttempts = 5

	// RetryInterval is the fixed delay between sweep attempts. Deliberately
	// coarse and non-exponential: kafka-go's own Writer already retries
	// transient broker errors internally before Publish ever returns an
	// error, so this sweep only needs to cover what that in-process retry
	// cannot -- an outage outlasting its own budget, or a process restart.
	RetryInterval = 30 * time.Second
)

// NextOutboxState decides an entry's status and next retry time after a
// failed publish attempt, given the attempt count so far (including the one
// that just failed). Once attemptsSoFar reaches MaxPublishAttempts, the
// entry is dead instead of being scheduled again.
func NextOutboxState(attemptsSoFar int, now time.Time) (status OutboxStatus, nextAttemptAt time.Time) {
	if attemptsSoFar >= MaxPublishAttempts {
		return OutboxStatusDead, time.Time{}
	}
	return OutboxStatusPending, now.Add(RetryInterval)
}
