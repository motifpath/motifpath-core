//go:build integration

package bdd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
)

func registerIngestSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the Event Ingestion Service is operational and ready to accept events$`, func() error { return nil })

	sc.Step(`^student "([^"]+)" is authenticated with a valid session$`, w.studentIsAuthenticated)
	sc.Step(`^student "([^"]+)" is authenticated$`, w.studentIsAuthenticated)
	sc.Step(`^"([^"]+)" has an active exercise attempt for exercise "([^"]+)" triggered by challenge "([^"]+)"$`, w.hasActiveExerciseAttemptWithChallenge)
	sc.Step(`^"([^"]+)" has an active exercise attempt for exercise "([^"]+)"$`, w.hasActiveExerciseAttempt)
	sc.Step(`^"([^"]+)" has already submitted a lesson\.started event with identifier "([^"]+)"$`, w.hasAlreadySubmittedLessonStarted)
	sc.Step(`^no authentication token is provided$`, w.noAuthToken)

	sc.Step(`^"([^"]+)" submits a lesson\.started event for video content node "([^"]+)"$`, w.submitLessonStarted)
	sc.Step(`^"([^"]+)" submits a lesson\.completed event for content node "([^"]+)" with a duration of (\d+) seconds$`, w.submitLessonCompletedWithDuration)
	sc.Step(`^"([^"]+)" submits an answer to the exercise as attempt number (\d+)$`, w.submitExerciseAnswer)
	sc.Step(`^"([^"]+)" submits an exercise\.ended event with outcome "([^"]+)" and a final score of (\d+)$`, w.submitExerciseEndedWithScore)
	sc.Step(`^"([^"]+)" submits an exercise\.ended event with outcome "([^"]+)" and no final score$`, w.submitExerciseEndedNoScore)
	sc.Step(`^"([^"]+)" submits the same lesson\.started event again with identifier "([^"]+)"$`, w.resubmitLessonStarted)
	sc.Step(`^"([^"]+)" submits a lesson\.completed event for content node "([^"]+)" with no duration$`, w.submitLessonCompletedNoDuration)
	sc.Step(`^"([^"]+)" submits a lesson\.started event whose content context includes an unrecognised field "([^"]+)"$`, w.submitLessonStartedWithExtraField)
	sc.Step(`^"([^"]+)" submits an exercise\.progress event with (\d+) elapsed seconds$`, w.submitExerciseProgress)
	sc.Step(`^an unauthenticated request submits a lesson\.started event$`, w.unauthenticatedSubmitLessonStarted)
	sc.Step(`^"([^"]+)" submits an event that identifies student "([^"]+)" as the author$`, w.submitEventWithMismatchedStudent)
	sc.Step(`^"([^"]+)" submits an event with the event type field omitted$`, w.submitEventMissingEventType)
	sc.Step(`^"([^"]+)" submits an event with event type "([^"]+)"$`, w.submitEventWithUnknownEventType)
	sc.Step(`^"([^"]+)" submits a lesson\.started event with the content context field omitted$`, w.submitLessonStartedMissingContentContext)
	sc.Step(`^"([^"]+)" submits an exercise\.answer_sent event with attempt number (\d+)$`, w.submitExerciseAnswerSentWithAttemptNumber)
	sc.Step(`^"([^"]+)" submits an exercise\.answer_sent event with the trigger context field omitted$`, w.submitExerciseAnswerSentMissingTriggerContext)

	sc.Step(`^the event is accepted and stored in the event log$`, w.eventIsAcceptedAndStored)
	sc.Step(`^the server returns the submitted event identifier and a receipt timestamp$`, w.responseHasEventIDAndReceivedAt)
	sc.Step(`^the event is accepted without error$`, w.eventIsAcceptedAndStored)
	sc.Step(`^the submission is refused with an authentication error$`, w.submissionRefusedAuthError)
	sc.Step(`^the submission is rejected as invalid$`, w.submissionRejectedInvalid)
	sc.Step(`^the rejection identifies "([^"]+)" as the source of the error$`, w.rejectionIdentifiesField)
}

// ── Given ──────────────────────────────────────────────────────────────────

func (w *world) studentIsAuthenticated(name string) error {
	w.hasToken = true
	w.tokenStudentID = studentID(name)
	return nil
}

func (w *world) noAuthToken() error {
	w.hasToken = false
	return nil
}

func (w *world) hasActiveExerciseAttemptWithChallenge(name, exerciseName, challengeName string) error {
	challengeID := deterministicUUID("challenge", challengeName)
	w.exerciseAttempts[name] = exerciseAttempt{
		exerciseID:  deterministicUUID("exercise", exerciseName),
		challengeID: &challengeID,
	}
	return nil
}

func (w *world) hasActiveExerciseAttempt(name, exerciseName string) error {
	w.exerciseAttempts[name] = exerciseAttempt{
		exerciseID: deterministicUUID("exercise", exerciseName),
	}
	return nil
}

func (w *world) hasAlreadySubmittedLessonStarted(name, eventIdentifier string) error {
	body := newTrackingEvent(newLessonStartedBody(name, deterministicUUID("event", eventIdentifier), "resubmit-test-node"))
	w.submit(body)
	if _, ok := w.ingestResp.(generated.IngestTrackingEvent202JSONResponse); !ok {
		return fmt.Errorf("setup: expected initial submission to be accepted, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	return nil
}

// ── When ───────────────────────────────────────────────────────────────────

func (w *world) submitLessonStarted(name, nodeIdentifier string) error {
	eventID := deterministicUUID("event", name+":"+nodeIdentifier+":lesson-started")
	w.submit(newTrackingEvent(newLessonStartedBody(name, eventID, nodeIdentifier)))
	return nil
}

func (w *world) submitLessonCompletedWithDuration(name, nodeIdentifier, durationStr string) error {
	duration, err := strconv.Atoi(durationStr)
	if err != nil {
		return err
	}
	eventID := deterministicUUID("event", name+":"+nodeIdentifier+":lesson-completed")
	v := newLessonCompletedBody(name, eventID, nodeIdentifier)
	v.DurationSeconds = &duration
	w.submit(newTrackingEvent(v))
	return nil
}

func (w *world) submitLessonCompletedNoDuration(name, nodeIdentifier string) error {
	eventID := deterministicUUID("event", name+":"+nodeIdentifier+":lesson-completed-no-duration")
	w.submit(newTrackingEvent(newLessonCompletedBody(name, eventID, nodeIdentifier)))
	return nil
}

func (w *world) submitLessonStartedWithExtraField(name, fieldName string) error {
	eventID := deterministicUUID("event", name+":extra-field")
	v := newLessonStartedBody(name, eventID, "intro-to-chords")
	v.ContentContext.Set(fieldName, "some-value")
	w.submit(newTrackingEvent(v))
	return nil
}

func (w *world) submitLessonStartedMissingContentContext(name string) error {
	eventID := deterministicUUID("event", name+":missing-content-context")
	v := newLessonStartedBody(name, eventID, "intro-to-chords")
	v.ContentContext = generated.ContentContext{}
	w.submit(newTrackingEvent(v))
	return nil
}

func (w *world) resubmitLessonStarted(_, _ string) error {
	if w.lastSubmittedBody == nil {
		return fmt.Errorf("no previously submitted event to resubmit")
	}
	w.submit(w.lastSubmittedBody)
	return nil
}

func (w *world) submitExerciseAnswer(name, attemptNumberStr string) error {
	attemptNumber, err := strconv.Atoi(attemptNumberStr)
	if err != nil {
		return err
	}
	attempt, err := w.attemptFor(name)
	if err != nil {
		return err
	}
	eventID := deterministicUUID("event", name+":answer:"+attemptNumberStr)
	v := generated.ExerciseAnswerSentEvent{
		EventId:        eventID,
		EventType:      "exercise.answer_sent",
		StudentId:      studentUUID(name),
		SessionId:      fixedSessionID,
		OccurredAt:     fixedOccurredAt,
		ExerciseId:     attempt.exerciseID,
		TriggerContext: attempt.triggerContext(),
		AttemptNumber:  attemptNumber,
	}
	w.submit(newTrackingEvent(v))
	return nil
}

// submitExerciseAnswerSentWithAttemptNumber and submitExerciseAnswerSentMissingTriggerContext
// deliberately build their own exercise_id rather than looking one up via
// attemptFor: neither of their scenarios has a "has an active exercise attempt"
// Given step — each is testing one specific required-field failure in isolation,
// with everything else in the payload otherwise valid.

func (w *world) submitExerciseAnswerSentWithAttemptNumber(name, attemptNumberStr string) error {
	attemptNumber, err := strconv.Atoi(attemptNumberStr)
	if err != nil {
		return err
	}
	eventID := deterministicUUID("event", name+":answer-attempt-number:"+attemptNumberStr)
	v := generated.ExerciseAnswerSentEvent{
		EventId:        eventID,
		EventType:      "exercise.answer_sent",
		StudentId:      studentUUID(name),
		SessionId:      fixedSessionID,
		OccurredAt:     fixedOccurredAt,
		ExerciseId:     deterministicUUID("exercise", "attempt-number-validation"),
		TriggerContext: generated.TriggerContext{Source: "free_practice"},
		AttemptNumber:  attemptNumber,
	}
	w.submit(newTrackingEvent(v))
	return nil
}

func (w *world) submitExerciseAnswerSentMissingTriggerContext(name string) error {
	eventID := deterministicUUID("event", name+":answer-missing-trigger")
	v := generated.ExerciseAnswerSentEvent{
		EventId:        eventID,
		EventType:      "exercise.answer_sent",
		StudentId:      studentUUID(name),
		SessionId:      fixedSessionID,
		OccurredAt:     fixedOccurredAt,
		ExerciseId:     deterministicUUID("exercise", "trigger-context-validation"),
		TriggerContext: generated.TriggerContext{},
		AttemptNumber:  1,
	}
	w.submit(newTrackingEvent(v))
	return nil
}

func (w *world) submitExerciseProgress(name, elapsedStr string) error {
	elapsed, err := strconv.Atoi(elapsedStr)
	if err != nil {
		return err
	}
	attempt, err := w.attemptFor(name)
	if err != nil {
		return err
	}
	eventID := deterministicUUID("event", name+":progress:"+elapsedStr)
	v := generated.ExerciseProgressEvent{
		EventId:        eventID,
		EventType:      "exercise.progress",
		StudentId:      studentUUID(name),
		SessionId:      fixedSessionID,
		OccurredAt:     fixedOccurredAt,
		ExerciseId:     attempt.exerciseID,
		TriggerContext: attempt.triggerContext(),
		ElapsedSeconds: &elapsed,
	}
	w.submit(newTrackingEvent(v))
	return nil
}

func (w *world) submitExerciseEndedWithScore(name, outcome, scoreStr string) error {
	score, err := strconv.Atoi(scoreStr)
	if err != nil {
		return err
	}
	attempt, err := w.attemptFor(name)
	if err != nil {
		return err
	}
	eventID := deterministicUUID("event", name+":ended:"+outcome+":"+scoreStr)
	v := generated.ExerciseEndedEvent{
		EventId:        eventID,
		EventType:      "exercise.ended",
		StudentId:      studentUUID(name),
		SessionId:      fixedSessionID,
		OccurredAt:     fixedOccurredAt,
		ExerciseId:     attempt.exerciseID,
		TriggerContext: attempt.triggerContext(),
		Outcome:        generated.ExerciseEndedEventOutcome(outcome),
		FinalScore:     &score,
	}
	w.submit(newTrackingEvent(v))
	return nil
}

func (w *world) submitExerciseEndedNoScore(name, outcome string) error {
	attempt, err := w.attemptFor(name)
	if err != nil {
		return err
	}
	eventID := deterministicUUID("event", name+":ended-no-score:"+outcome)
	v := generated.ExerciseEndedEvent{
		EventId:        eventID,
		EventType:      "exercise.ended",
		StudentId:      studentUUID(name),
		SessionId:      fixedSessionID,
		OccurredAt:     fixedOccurredAt,
		ExerciseId:     attempt.exerciseID,
		TriggerContext: attempt.triggerContext(),
		Outcome:        generated.ExerciseEndedEventOutcome(outcome),
	}
	w.submit(newTrackingEvent(v))
	return nil
}

func (w *world) unauthenticatedSubmitLessonStarted() error {
	eventID := deterministicUUID("event", "unauthenticated-submission")
	w.submit(newTrackingEvent(newLessonStartedBody("nobody", eventID, "intro-to-chords")))
	return nil
}

func (w *world) submitEventWithMismatchedStudent(callerName, claimedName string) error {
	eventID := deterministicUUID("event", callerName+":claims:"+claimedName)
	v := newLessonStartedBody(claimedName, eventID, "intro-to-chords")
	w.submit(newTrackingEvent(v))
	return nil
}

// submitEventMissingEventType uses a zero-value TrackingEvent: its Discriminator()
// call fails on the empty union exactly as it would on a payload that had every
// other field but omitted event_type, since the handler checks the discriminator
// before looking at anything else.
func (w *world) submitEventMissingEventType(_ string) error {
	w.submit(&generated.TrackingEvent{})
	return nil
}

func (w *world) submitEventWithUnknownEventType(name, eventType string) error {
	raw := fmt.Sprintf(
		`{"event_id":%q,"event_type":%q,"student_id":%q,"session_id":%q,"occurred_at":%q}`,
		deterministicUUID("event", name+":unknown-type").String(),
		eventType,
		studentUUID(name).String(),
		fixedSessionID.String(),
		fixedOccurredAt.Format("2006-01-02T15:04:05Z07:00"),
	)
	body := &generated.TrackingEvent{}
	if err := body.UnmarshalJSON([]byte(raw)); err != nil {
		return err
	}
	w.submit(body)
	return nil
}

// ── Then ───────────────────────────────────────────────────────────────────

func (w *world) eventIsAcceptedAndStored() error {
	resp, ok := w.ingestResp.(generated.IngestTrackingEvent202JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 202 response, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	if _, saved := w.repo.saved[resp.EventId.String()]; !saved {
		return fmt.Errorf("event %s was not found in the repository", resp.EventId)
	}
	return nil
}

func (w *world) responseHasEventIDAndReceivedAt() error {
	resp, ok := w.ingestResp.(generated.IngestTrackingEvent202JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 202 response, got %#v", w.ingestResp)
	}
	if resp.EventId == (uuid.UUID{}) {
		return fmt.Errorf("response event_id was empty")
	}
	if resp.ReceivedAt.IsZero() {
		return fmt.Errorf("response received_at was zero")
	}
	return nil
}

func (w *world) submissionRefusedAuthError() error {
	if _, ok := w.ingestResp.(generated.IngestTrackingEvent401JSONResponse); !ok {
		return fmt.Errorf("expected a 401 response, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	return nil
}

func (w *world) submissionRejectedInvalid() error {
	if _, ok := w.ingestResp.(generated.IngestTrackingEvent400JSONResponse); !ok {
		return fmt.Errorf("expected a 400 response, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	return nil
}

func (w *world) rejectionIdentifiesField(field string) error {
	resp, ok := w.ingestResp.(generated.IngestTrackingEvent400JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 400 response, got %#v", w.ingestResp)
	}
	for _, e := range resp.Errors {
		if strings.Contains(e.Reason, field) {
			return nil
		}
	}
	return fmt.Errorf("no validation error mentioned %q; got %+v", field, resp.Errors)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (w *world) attemptFor(name string) (exerciseAttempt, error) {
	a, ok := w.exerciseAttempts[name]
	if !ok {
		return exerciseAttempt{}, fmt.Errorf("no active exercise attempt recorded for %q", name)
	}
	return a, nil
}

func newLessonStartedBody(studentName string, eventID uuid.UUID, nodeIdentifier string) generated.LessonStartedEvent {
	contentType := generated.ContentContextContentType("video")
	return generated.LessonStartedEvent{
		EventId:    eventID,
		EventType:  "lesson.started",
		StudentId:  studentUUID(studentName),
		SessionId:  fixedSessionID,
		OccurredAt: fixedOccurredAt,
		ContentContext: generated.ContentContext{
			ContentNodeId: deterministicUUID("node", nodeIdentifier),
			ContentType:   &contentType,
		},
	}
}

func newLessonCompletedBody(studentName string, eventID uuid.UUID, nodeIdentifier string) generated.LessonCompletedEvent {
	return generated.LessonCompletedEvent{
		EventId:    eventID,
		EventType:  "lesson.completed",
		StudentId:  studentUUID(studentName),
		SessionId:  fixedSessionID,
		OccurredAt: fixedOccurredAt,
		ContentContext: generated.ContentContext{
			ContentNodeId: deterministicUUID("node", nodeIdentifier),
		},
	}
}

// trackingEventSource is implemented by every generated.FromXxxEvent-settable type
// via the small adapter functions below, letting newTrackingEvent stay generic.
type trackingEventSetter interface {
	generated.LessonStartedEvent |
		generated.LessonCompletedEvent |
		generated.ExerciseAnswerSentEvent |
		generated.ExerciseProgressEvent |
		generated.ExerciseEndedEvent
}

func newTrackingEvent[T trackingEventSetter](v T) *generated.TrackingEvent {
	body := &generated.TrackingEvent{}
	switch typed := any(v).(type) {
	case generated.LessonStartedEvent:
		_ = body.FromLessonStartedEvent(typed)
	case generated.LessonCompletedEvent:
		_ = body.FromLessonCompletedEvent(typed)
	case generated.ExerciseAnswerSentEvent:
		_ = body.FromExerciseAnswerSentEvent(typed)
	case generated.ExerciseProgressEvent:
		_ = body.FromExerciseProgressEvent(typed)
	case generated.ExerciseEndedEvent:
		_ = body.FromExerciseEndedEvent(typed)
	}
	return body
}
