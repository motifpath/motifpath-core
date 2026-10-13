//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// The review marker never reaches the worker's domain: the decoder reads an
// event marked as review exactly as the same event unmarked (pinned in the
// kafka adapter's tests). These steps pin what that means for progress and
// evidence.
func registerReviewSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]*)"'s lesson\.started for "([^"]*)", marked as review, arrives$`, func(student, contentNode string) error {
		return w.handle(domain.TrackingEvent{
			EventType:     domain.EventTypeLessonStarted,
			EventID:       w.nextEventID(),
			StudentID:     w.studentID(student),
			OccurredAt:    activityDay.Add(19 * 60 * 60 * 1e9),
			ContentNodeID: stableUUID("content-node", contentNode),
		})
	})
	sc.Step(`^"([^"]*)" is still completed for "([^"]*)"$`, func(contentNode, student string) error {
		status := w.completions.statuses[w.studentID(student)+"|"+stableUUID("content-node", contentNode)]
		if status != domain.CompletionStatusCompleted {
			return fmt.Errorf("expected %q completed for %q, got %q", contentNode, student, status)
		}
		return nil
	})
	sc.Step(`^"([^"]*)" answers the cell correctly (\d+) times on (\d+) different days, each marked as review$`, func(student string, times, days int) error {
		w.challenges[student] = &domain.TriggerContext{
			Source:      "challenge_sequence",
			ChallengeID: stableUUID("challenge", "reviewed-challenge"),
		}
		return w.answersTheCellOnDifferentDays(student, times, days)
	})
}
