package kafka

import (
	"time"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// wireEvent is the JSON shape published to the motifpath.events topic. Kept
// independent of the HTTP adapter's generated types: the Kafka wire format and the
// HTTP API contract are allowed to evolve separately even though they currently
// carry the same fields.
type wireEvent struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	StudentID  string    `json:"student_id"`
	SessionID  string    `json:"session_id"`
	OccurredAt time.Time `json:"occurred_at"`

	ContentContext *contentContextWire `json:"content_context,omitempty"`
	ExerciseID     string              `json:"exercise_id,omitempty"`
	TriggerContext *triggerContextWire `json:"trigger_context,omitempty"`
	AttemptNumber  *int                `json:"attempt_number,omitempty"`
	AnswerPayload  map[string]any      `json:"answer_payload,omitempty"`
	Outcome        string              `json:"outcome,omitempty"`
	FinalScore     *int                `json:"final_score,omitempty"`

	DurationSeconds *int `json:"duration_seconds,omitempty"`
	ElapsedSeconds  *int `json:"elapsed_seconds,omitempty"`
}

type contentContextWire struct {
	ContentNodeID string `json:"content_node_id"`
	ContentType   string `json:"content_type,omitempty"`
	TeacherID     string `json:"teacher_id,omitempty"`
}

type triggerContextWire struct {
	Source        string `json:"source"`
	ContentNodeID string `json:"content_node_id,omitempty"`
	ChallengeID   string `json:"challenge_id,omitempty"`
}

func toWireEvent(event domain.TrackingEvent) wireEvent {
	base := event.Base()
	w := wireEvent{
		EventID:    base.EventID,
		EventType:  string(base.EventType),
		StudentID:  base.StudentID,
		SessionID:  base.SessionID,
		OccurredAt: base.OccurredAt,
	}

	switch e := event.(type) {
	case domain.LessonStartedEvent:
		w.ContentContext = toContentContextWire(e.ContentContext)
	case domain.LessonResumedEvent:
		w.ContentContext = toContentContextWire(e.ContentContext)
	case domain.LessonCompletedEvent:
		w.ContentContext = toContentContextWire(e.ContentContext)
		w.DurationSeconds = e.DurationSeconds
	case domain.ExerciseStartedEvent:
		w.ExerciseID = e.ExerciseID
		w.TriggerContext = toTriggerContextWire(e.TriggerContext)
	case domain.ExerciseProgressEvent:
		w.ExerciseID = e.ExerciseID
		w.TriggerContext = toTriggerContextWire(e.TriggerContext)
		w.ElapsedSeconds = e.ElapsedSeconds
	case domain.ExerciseAnswerSentEvent:
		w.ExerciseID = e.ExerciseID
		w.TriggerContext = toTriggerContextWire(e.TriggerContext)
		w.AttemptNumber = &e.AttemptNumber
		w.AnswerPayload = e.AnswerPayload
	case domain.ExerciseEndedEvent:
		w.ExerciseID = e.ExerciseID
		w.TriggerContext = toTriggerContextWire(e.TriggerContext)
		w.Outcome = string(e.Outcome)
		w.FinalScore = e.FinalScore
	}

	return w
}

func toContentContextWire(cc domain.ContentContext) *contentContextWire {
	return &contentContextWire{
		ContentNodeID: cc.ContentNodeID,
		ContentType:   string(cc.ContentType),
		TeacherID:     cc.TeacherID,
	}
}

func toTriggerContextWire(tc domain.TriggerContext) *triggerContextWire {
	return &triggerContextWire{
		Source:        string(tc.Source),
		ContentNodeID: tc.ContentNodeID,
		ChallengeID:   tc.ChallengeID,
	}
}
