package ports

import (
	"context"
	"time"
)

// TapBaselineReader finds a student's tap time: the median from their latest
// practice.tap_check_completed event. Timed practice answers are stamped with it so the
// grader can tell time spent knowing the answer apart from time spent tapping it.
type TapBaselineReader interface {
	// LatestTap returns the median tap of the student's newest tap check that occurred
	// strictly before the given time. found is false when the student has none.
	LatestTap(ctx context.Context, studentID string, before time.Time) (tapMs int, found bool, err error)
}
