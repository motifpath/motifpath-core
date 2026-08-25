package repo

import (
	"time"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// eventDocument mirrors the `events` collection schema from ADR-008: one flat
// document per event. Fields that don't apply to a given event_type are left zero
// and omitted from the stored document via omitempty.
type eventDocument struct {
	EventID    string    `bson:"event_id"`
	EventType  string    `bson:"event_type"`
	StudentID  string    `bson:"student_id"`
	SessionID  string    `bson:"session_id"`
	OccurredAt time.Time `bson:"occurred_at"`
	ReceivedAt time.Time `bson:"received_at"`

	ContentContext *contentContextDoc `bson:"content_context,omitempty"`
	ExerciseID     string             `bson:"exercise_id,omitempty"`
	TriggerContext *triggerContextDoc `bson:"trigger_context,omitempty"`
	AttemptNumber  *int               `bson:"attempt_number,omitempty"`
	AnswerPayload  map[string]any     `bson:"answer_payload,omitempty"`
	Outcome        string             `bson:"outcome,omitempty"`
	FinalScore     *int               `bson:"final_score,omitempty"`

	// DurationSeconds (lesson.completed) and ElapsedSeconds (exercise.progress) are
	// both from events.yaml. ADR-008's schema table lists DurationSeconds but omits
	// ElapsedSeconds — stored anyway to avoid silently dropping real event data; the
	// ADR's table should be amended to match.
	DurationSeconds *int `bson:"duration_seconds,omitempty"`
	ElapsedSeconds  *int `bson:"elapsed_seconds,omitempty"`
}

type contentContextDoc struct {
	ContentNodeID string `bson:"content_node_id"`
	ContentType   string `bson:"content_type,omitempty"`
	TeacherID     string `bson:"teacher_id,omitempty"`
}

type triggerContextDoc struct {
	Source        string `bson:"source"`
	ContentNodeID string `bson:"content_node_id,omitempty"`
	ChallengeID   string `bson:"challenge_id,omitempty"`
}

func toDocument(event domain.TrackingEvent, receivedAt time.Time) eventDocument {
	base := event.Base()
	doc := eventDocument{
		EventID:    base.EventID,
		EventType:  string(base.EventType),
		StudentID:  base.StudentID,
		SessionID:  base.SessionID,
		OccurredAt: base.OccurredAt,
		ReceivedAt: receivedAt,
	}

	switch e := event.(type) {
	case domain.LessonStartedEvent:
		doc.ContentContext = toContentContextDoc(e.ContentContext)
	case domain.LessonResumedEvent:
		doc.ContentContext = toContentContextDoc(e.ContentContext)
	case domain.LessonCompletedEvent:
		doc.ContentContext = toContentContextDoc(e.ContentContext)
		doc.DurationSeconds = e.DurationSeconds
	case domain.ExerciseStartedEvent:
		doc.ExerciseID = e.ExerciseID
		doc.TriggerContext = toTriggerContextDoc(e.TriggerContext)
	case domain.ExerciseProgressEvent:
		doc.ExerciseID = e.ExerciseID
		doc.TriggerContext = toTriggerContextDoc(e.TriggerContext)
		doc.ElapsedSeconds = e.ElapsedSeconds
	case domain.ExerciseAnswerSentEvent:
		doc.ExerciseID = e.ExerciseID
		doc.TriggerContext = toTriggerContextDoc(e.TriggerContext)
		doc.AttemptNumber = &e.AttemptNumber
		doc.AnswerPayload = e.AnswerPayload
	case domain.ExerciseEndedEvent:
		doc.ExerciseID = e.ExerciseID
		doc.TriggerContext = toTriggerContextDoc(e.TriggerContext)
		doc.Outcome = string(e.Outcome)
		doc.FinalScore = e.FinalScore
	}

	return doc
}

func toContentContextDoc(cc domain.ContentContext) *contentContextDoc {
	return &contentContextDoc{
		ContentNodeID: cc.ContentNodeID,
		ContentType:   string(cc.ContentType),
		TeacherID:     cc.TeacherID,
	}
}

func toTriggerContextDoc(tc domain.TriggerContext) *triggerContextDoc {
	return &triggerContextDoc{
		Source:        string(tc.Source),
		ContentNodeID: tc.ContentNodeID,
		ChallengeID:   tc.ChallengeID,
	}
}
