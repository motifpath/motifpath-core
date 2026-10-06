package ports

import (
	"context"
	"time"
)

// TapCheckReader reads the tap checks a student did from the Aggregation
// Worker's `tap_checks` collection. It is read-only from this service's
// perspective — the worker is the only writer.
type TapCheckReader interface {
	// LastTapCheck returns when the student did their newest tap check;
	// found is false when they never did one.
	LastTapCheck(ctx context.Context, studentID string) (doneAt time.Time, found bool, err error)
}
