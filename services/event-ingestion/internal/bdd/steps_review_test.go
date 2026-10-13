//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

func registerReviewSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" submits a lesson\.started event for content node "([^"]+)" whose content context is marked as review$`, w.submitLessonStartedInReview)
	sc.Step(`^"([^"]+)" submits an exercise\.ended event with outcome "([^"]+)" whose trigger context is marked as review$`, w.submitExerciseEndedInReview)
	sc.Step(`^"([^"]+)" submits an exercise\.started event whose trigger context has the review marker "([^"]+)"$`, w.submitExerciseStartedWithReviewMarker)
	sc.Step(`^the event is accepted and stored in the event log, with the review marker$`, w.eventStoredWithReviewMarker)
}

func (w *world) submitLessonStartedInReview(name, nodeIdentifier string) error {
	review := true
	v := newLessonStartedBody(name, deterministicUUID("event", name+":"+nodeIdentifier+":lesson-started-review"), nodeIdentifier)
	v.ContentContext.Review = &review
	w.submit(newTrackingEvent(v))
	return nil
}

func (w *world) submitExerciseEndedInReview(name, outcome string) error {
	attempt, err := w.attemptFor(name)
	if err != nil {
		return err
	}
	review := true
	trigger := attempt.triggerContext()
	trigger.Review = &review
	w.submit(newTrackingEvent(generated.ExerciseEndedEvent{
		EventId:        deterministicUUID("event", name+":ended-review"),
		EventType:      "exercise.ended",
		StudentId:      studentUUID(name),
		SessionId:      fixedSessionID,
		OccurredAt:     fixedOccurredAt,
		ExerciseId:     attempt.exerciseID,
		TriggerContext: trigger,
		Outcome:        generated.ExerciseEndedEventOutcome(outcome),
	}))
	return nil
}

// submitExerciseStartedWithReviewMarker sends the marker as a string, which
// the typed request body cannot hold, so the event is built from raw JSON.
func (w *world) submitExerciseStartedWithReviewMarker(name, marker string) error {
	raw := fmt.Sprintf(`{"event_id":%q,"event_type":"exercise.started","student_id":%q,"session_id":%q,"occurred_at":%q,"exercise_id":%q,"trigger_context":{"source":"challenge_sequence","review":%q}}`,
		deterministicUUID("event", name+":started-bad-review"), studentUUID(name), fixedSessionID, fixedOccurredAt.Format("2006-01-02T15:04:05Z07:00"),
		deterministicUUID("exercise", "review-validation"), marker)
	body := &generated.TrackingEvent{}
	if err := body.UnmarshalJSON([]byte(raw)); err != nil {
		return err
	}
	w.submit(body)
	return nil
}

func (w *world) eventStoredWithReviewMarker() error {
	resp, ok := w.ingestResp.(generated.IngestTrackingEvent202JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 202 response, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	event, saved := w.repo.saved[resp.EventId.String()]
	if !saved {
		return fmt.Errorf("event %s was not found in the repository", resp.EventId)
	}
	var review bool
	switch e := event.(type) {
	case domain.LessonStartedEvent:
		review = e.ContentContext.Review
	case domain.ExerciseEndedEvent:
		review = e.TriggerContext.Review
	default:
		return fmt.Errorf("no review marker is kept on %T", event)
	}
	if !review {
		return fmt.Errorf("expected the stored %s to carry the review marker", event.Base().EventType)
	}
	return nil
}
