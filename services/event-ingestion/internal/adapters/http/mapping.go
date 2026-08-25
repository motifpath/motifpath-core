package http

import (
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

// toDomainEvent converts the generated wire-format union into a concrete domain
// event. Note that oapi-codegen's generated types only enforce required fields
// that are structurally impossible to omit (i.e. none — encoding/json leaves an
// absent non-pointer field at its zero value rather than erroring). The zero-value
// checks in the per-type functions below are what actually reject a missing
// required field; they are not redundant with unmarshaling.
func toDomainEvent(body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	if body == nil {
		return nil, fmt.Errorf("%w: request body", domain.ErrMissingRequiredField)
	}

	eventTypeStr, err := body.Discriminator()
	if err != nil {
		return nil, fmt.Errorf("%w: event_type", domain.ErrMissingRequiredField)
	}
	eventType := domain.EventType(eventTypeStr)

	switch eventType {
	case domain.EventTypeLessonStarted:
		return toLessonStartedEvent(eventType, body)
	case domain.EventTypeLessonResumed:
		return toLessonResumedEvent(eventType, body)
	case domain.EventTypeLessonCompleted:
		return toLessonCompletedEvent(eventType, body)
	case domain.EventTypeExerciseStarted:
		return toExerciseStartedEvent(eventType, body)
	case domain.EventTypeExerciseProgress:
		return toExerciseProgressEvent(eventType, body)
	case domain.EventTypeExerciseAnswerSent:
		return toExerciseAnswerSentEvent(eventType, body)
	case domain.EventTypeExerciseEnded:
		return toExerciseEndedEvent(eventType, body)
	default:
		return nil, fmt.Errorf("%w: %q", domain.ErrInvalidEventType, eventTypeStr)
	}
}

func toLessonStartedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsLessonStartedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	cc, err := toDomainContentContext(v.ContentContext)
	if err != nil {
		return nil, err
	}
	return domain.LessonStartedEvent{TrackingEventBase: base, ContentContext: cc}, nil
}

func toLessonResumedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsLessonResumedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	cc, err := toDomainContentContext(v.ContentContext)
	if err != nil {
		return nil, err
	}
	return domain.LessonResumedEvent{TrackingEventBase: base, ContentContext: cc}, nil
}

func toLessonCompletedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsLessonCompletedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	cc, err := toDomainContentContext(v.ContentContext)
	if err != nil {
		return nil, err
	}
	return domain.LessonCompletedEvent{TrackingEventBase: base, ContentContext: cc, DurationSeconds: v.DurationSeconds}, nil
}

func toExerciseStartedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsExerciseStartedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	exerciseID, err := requireUUID(v.ExerciseId, "exercise_id")
	if err != nil {
		return nil, err
	}
	tc, err := toDomainTriggerContext(v.TriggerContext)
	if err != nil {
		return nil, err
	}
	return domain.ExerciseStartedEvent{TrackingEventBase: base, ExerciseID: exerciseID, TriggerContext: tc}, nil
}

func toExerciseProgressEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsExerciseProgressEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	exerciseID, err := requireUUID(v.ExerciseId, "exercise_id")
	if err != nil {
		return nil, err
	}
	tc, err := toDomainTriggerContext(v.TriggerContext)
	if err != nil {
		return nil, err
	}
	return domain.ExerciseProgressEvent{TrackingEventBase: base, ExerciseID: exerciseID, TriggerContext: tc, ElapsedSeconds: v.ElapsedSeconds}, nil
}

func toExerciseAnswerSentEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsExerciseAnswerSentEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	exerciseID, err := requireUUID(v.ExerciseId, "exercise_id")
	if err != nil {
		return nil, err
	}
	tc, err := toDomainTriggerContext(v.TriggerContext)
	if err != nil {
		return nil, err
	}
	if v.AttemptNumber < 1 {
		return nil, fmt.Errorf("%w: attempt_number", domain.ErrMissingRequiredField)
	}
	var answerPayload map[string]any
	if v.AnswerPayload != nil {
		answerPayload = *v.AnswerPayload
	}
	return domain.ExerciseAnswerSentEvent{
		TrackingEventBase: base,
		ExerciseID:        exerciseID,
		TriggerContext:    tc,
		AttemptNumber:     v.AttemptNumber,
		AnswerPayload:     answerPayload,
	}, nil
}

func toExerciseEndedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsExerciseEndedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	exerciseID, err := requireUUID(v.ExerciseId, "exercise_id")
	if err != nil {
		return nil, err
	}
	tc, err := toDomainTriggerContext(v.TriggerContext)
	if err != nil {
		return nil, err
	}
	outcome, err := requireOutcome(v.Outcome)
	if err != nil {
		return nil, err
	}
	return domain.ExerciseEndedEvent{
		TrackingEventBase: base,
		ExerciseID:        exerciseID,
		TriggerContext:    tc,
		Outcome:           outcome,
		FinalScore:        v.FinalScore,
	}, nil
}

func toDomainBase(eventType domain.EventType, eventID, studentID, sessionID openapi_types.UUID, occurredAt time.Time) (domain.TrackingEventBase, error) {
	id, err := requireUUID(eventID, "event_id")
	if err != nil {
		return domain.TrackingEventBase{}, err
	}
	studentIDStr, err := requireUUID(studentID, "student_id")
	if err != nil {
		return domain.TrackingEventBase{}, err
	}
	sessionIDStr, err := requireUUID(sessionID, "session_id")
	if err != nil {
		return domain.TrackingEventBase{}, err
	}
	if occurredAt.IsZero() {
		return domain.TrackingEventBase{}, fmt.Errorf("%w: occurred_at", domain.ErrMissingRequiredField)
	}
	return domain.TrackingEventBase{
		EventID:    id,
		EventType:  eventType,
		StudentID:  studentIDStr,
		SessionID:  sessionIDStr,
		OccurredAt: occurredAt,
	}, nil
}

func requireUUID(id openapi_types.UUID, field string) (string, error) {
	if id == (openapi_types.UUID{}) {
		return "", fmt.Errorf("%w: %s", domain.ErrMissingRequiredField, field)
	}
	return id.String(), nil
}

func toDomainContentContext(cc generated.ContentContext) (domain.ContentContext, error) {
	nodeID, err := requireUUID(cc.ContentNodeId, "content_context.content_node_id")
	if err != nil {
		return domain.ContentContext{}, err
	}
	result := domain.ContentContext{ContentNodeID: nodeID}
	if cc.ContentType != nil {
		result.ContentType = domain.ContentType(*cc.ContentType)
	}
	if cc.TeacherId != nil {
		result.TeacherID = cc.TeacherId.String()
	}
	return result, nil
}

func toDomainTriggerContext(tc generated.TriggerContext) (domain.TriggerContext, error) {
	if tc.Source == "" {
		return domain.TriggerContext{}, fmt.Errorf("%w: trigger_context.source", domain.ErrMissingRequiredField)
	}
	result := domain.TriggerContext{Source: domain.TriggerSource(tc.Source)}
	if tc.ContentNodeId != nil {
		result.ContentNodeID = tc.ContentNodeId.String()
	}
	if tc.ChallengeId != nil {
		result.ChallengeID = tc.ChallengeId.String()
	}
	return result, nil
}

func requireOutcome(outcome generated.ExerciseEndedEventOutcome) (domain.ExerciseOutcome, error) {
	switch domain.ExerciseOutcome(outcome) {
	case domain.ExerciseOutcomeCompleted, domain.ExerciseOutcomeAbandoned:
		return domain.ExerciseOutcome(outcome), nil
	default:
		return "", fmt.Errorf("%w: outcome", domain.ErrMissingRequiredField)
	}
}
