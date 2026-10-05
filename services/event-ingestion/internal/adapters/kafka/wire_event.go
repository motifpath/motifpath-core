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
	Outcome        string              `json:"outcome,omitempty"`
	FinalScore     *int                `json:"final_score,omitempty"`

	DurationSeconds *int `json:"duration_seconds,omitempty"`
	ElapsedSeconds  *int `json:"elapsed_seconds,omitempty"`

	// practice.* fields. The ones a practice event requires even at their
	// zero value -- answered_count, left_early, felt_ratings -- are pointers, so they
	// are always present on the event that carries them and absent on the others.
	PracticeSessionID string                `json:"practice_session_id,omitempty"`
	InstrumentID      string                `json:"instrument_id,omitempty"`
	Minutes           *int                  `json:"minutes,omitempty"`
	PlannedItems      []plannedItemWire     `json:"planned_items,omitempty"`
	ItemKey           string                `json:"item_key,omitempty"`
	Response          *practiceResponseWire `json:"response,omitempty"`
	TapMs             *int                  `json:"tap_ms,omitempty"`
	AnsweredCount     *int                  `json:"answered_count,omitempty"`
	LeftEarly         *bool                 `json:"left_early,omitempty"`
	FeltRatings       *[]feltRatingWire     `json:"felt_ratings,omitempty"`
	MedianTapMs       *int                  `json:"median_tap_ms,omitempty"`
	TapCount          *int                  `json:"tap_count,omitempty"`
}

type plannedItemWire struct {
	ItemKey string `json:"item_key"`
	Reason  string `json:"reason"`
}

// practiceResponseWire carries the raw response with only its shape's properties.
type practiceResponseWire struct {
	ResponseType     string   `json:"response_type"`
	NoteName         string   `json:"note_name,omitempty"`
	String           *int     `json:"string,omitempty"`
	Fret             *int     `json:"fret,omitempty"`
	OptionIDs        []string `json:"option_ids,omitempty"`
	LatencyMs        *int     `json:"latency_ms,omitempty"`
	AudioMs          *int     `json:"audio_ms,omitempty"`
	Rating           string   `json:"rating,omitempty"`
	TempoBPM         *int     `json:"tempo_bpm,omitempty"`
	ChangesPerMinute *int     `json:"changes_per_minute,omitempty"`
}

type feltRatingWire struct {
	DrillTemplateKey string `json:"drill_template_key"`
	Felt             string `json:"felt"`
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
	case domain.ExerciseEndedEvent:
		w.ExerciseID = e.ExerciseID
		w.TriggerContext = toTriggerContextWire(e.TriggerContext)
		w.Outcome = string(e.Outcome)
		w.FinalScore = e.FinalScore
	case domain.PracticeSessionStartedEvent:
		w.PracticeSessionID = e.PracticeSessionID
		w.InstrumentID = e.InstrumentID
		w.Minutes = &e.Minutes
		w.PlannedItems = make([]plannedItemWire, 0, len(e.PlannedItems))
		for _, item := range e.PlannedItems {
			w.PlannedItems = append(w.PlannedItems, plannedItemWire{ItemKey: item.ItemKey, Reason: string(item.Reason)})
		}
	case domain.PracticeItemAnsweredEvent:
		w.PracticeSessionID = e.PracticeSessionID
		if e.TriggerContext != nil {
			w.TriggerContext = toTriggerContextWire(*e.TriggerContext)
		}
		w.ItemKey = e.ItemKey
		w.Response = toPracticeResponseWire(e.Response)
		w.TapMs = e.TapMs
	case domain.PracticeSessionEndedEvent:
		w.PracticeSessionID = e.PracticeSessionID
		w.AnsweredCount = &e.AnsweredCount
		w.LeftEarly = &e.LeftEarly
		felt := make([]feltRatingWire, 0, len(e.FeltRatings))
		for _, r := range e.FeltRatings {
			felt = append(felt, feltRatingWire{DrillTemplateKey: r.DrillTemplateKey, Felt: string(r.Felt)})
		}
		w.FeltRatings = &felt
	case domain.PracticeTapCheckCompletedEvent:
		w.MedianTapMs = &e.MedianTapMs
		w.TapCount = &e.TapCount
	}

	return w
}

func toPracticeResponseWire(r domain.PracticeResponse) *practiceResponseWire {
	return &practiceResponseWire{
		ResponseType:     string(r.Type),
		NoteName:         r.NoteName,
		String:           r.String,
		Fret:             r.Fret,
		OptionIDs:        r.OptionIDs,
		LatencyMs:        r.LatencyMs,
		AudioMs:          r.AudioMs,
		Rating:           string(r.Rating),
		TempoBPM:         r.TempoBPM,
		ChangesPerMinute: r.ChangesPerMinute,
	}
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
