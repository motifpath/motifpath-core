package http

import (
	"encoding/json"
	"time"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

// loggedEventJSON is a stored event in the shape it was submitted in, for the
// event log. It mirrors the Kafka adapter's wire format on purpose rather than
// sharing it: the topic and the HTTP contract may evolve separately, even
// though they carry the same fields today.
type loggedEventJSON struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	StudentID  string    `json:"student_id"`
	SessionID  string    `json:"session_id"`
	OccurredAt time.Time `json:"occurred_at"`

	ContentContext *contentContextJSON `json:"content_context,omitempty"`
	ExerciseID     string              `json:"exercise_id,omitempty"`
	TriggerContext *triggerContextJSON `json:"trigger_context,omitempty"`
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
	PlannedItems      []plannedItemJSON     `json:"planned_items,omitempty"`
	ItemKey           string                `json:"item_key,omitempty"`
	Response          *practiceResponseJSON `json:"response,omitempty"`
	TapMs             *int                  `json:"tap_ms,omitempty"`
	AnsweredCount     *int                  `json:"answered_count,omitempty"`
	LeftEarly         *bool                 `json:"left_early,omitempty"`
	FeltRatings       *[]feltRatingJSON     `json:"felt_ratings,omitempty"`
	MedianTapMs       *int                  `json:"median_tap_ms,omitempty"`
	TapCount          *int                  `json:"tap_count,omitempty"`

	// song_chart.* fields.
	SongChartContext  *songChartContextJSON `json:"song_chart_context,omitempty"`
	Source            string                `json:"source,omitempty"`
	AnchorID          string                `json:"anchor_id,omitempty"`
	ChordDefinitionID string                `json:"chord_definition_id,omitempty"`
	ChordVoicingID    string                `json:"chord_voicing_id,omitempty"`
}

type songChartContextJSON struct {
	SongChartID    string `json:"song_chart_id"`
	RevisionNumber int    `json:"revision_number"`
}

type plannedItemJSON struct {
	ItemKey string `json:"item_key"`
	Reason  string `json:"reason"`
}

// practiceResponseJSON carries the raw response with only its shape's properties.
type practiceResponseJSON struct {
	ResponseType     string   `json:"response_type"`
	NoteName         string   `json:"note_name,omitempty"`
	Shape            string   `json:"shape,omitempty"`
	Interval         string   `json:"interval,omitempty"`
	String           *int     `json:"string,omitempty"`
	Fret             *int     `json:"fret,omitempty"`
	OptionIDs        []string `json:"option_ids,omitempty"`
	LatencyMs        *int     `json:"latency_ms,omitempty"`
	AudioMs          *int     `json:"audio_ms,omitempty"`
	Rating           string   `json:"rating,omitempty"`
	TempoBPM         *int     `json:"tempo_bpm,omitempty"`
	ChangesPerMinute *int     `json:"changes_per_minute,omitempty"`
}

type feltRatingJSON struct {
	DrillTemplateKey string `json:"drill_template_key"`
	Felt             string `json:"felt"`
}

type contentContextJSON struct {
	ContentNodeID string `json:"content_node_id"`
	ContentType   string `json:"content_type,omitempty"`
	TeacherID     string `json:"teacher_id,omitempty"`
	Review        bool   `json:"review,omitempty"`
}

type triggerContextJSON struct {
	Source        string `json:"source"`
	ContentNodeID string `json:"content_node_id,omitempty"`
	ChallengeID   string `json:"challenge_id,omitempty"`
	Review        bool   `json:"review,omitempty"`
}

func toLoggedEventJSON(event domain.TrackingEvent) loggedEventJSON {
	base := event.Base()
	w := loggedEventJSON{
		EventID:    base.EventID,
		EventType:  string(base.EventType),
		StudentID:  base.StudentID,
		SessionID:  base.SessionID,
		OccurredAt: base.OccurredAt,
	}

	switch e := event.(type) {
	case domain.LessonStartedEvent:
		w.ContentContext = toContentContextJSON(e.ContentContext)
	case domain.LessonResumedEvent:
		w.ContentContext = toContentContextJSON(e.ContentContext)
	case domain.LessonCompletedEvent:
		w.ContentContext = toContentContextJSON(e.ContentContext)
		w.DurationSeconds = e.DurationSeconds
	case domain.ExerciseStartedEvent:
		w.ExerciseID = e.ExerciseID
		w.TriggerContext = toTriggerContextJSON(e.TriggerContext)
	case domain.ExerciseProgressEvent:
		w.ExerciseID = e.ExerciseID
		w.TriggerContext = toTriggerContextJSON(e.TriggerContext)
		w.ElapsedSeconds = e.ElapsedSeconds
	case domain.ExerciseEndedEvent:
		w.ExerciseID = e.ExerciseID
		w.TriggerContext = toTriggerContextJSON(e.TriggerContext)
		w.Outcome = string(e.Outcome)
		w.FinalScore = e.FinalScore
	case domain.PracticeSessionStartedEvent:
		w.PracticeSessionID = e.PracticeSessionID
		w.InstrumentID = e.InstrumentID
		w.Minutes = &e.Minutes
		w.PlannedItems = make([]plannedItemJSON, 0, len(e.PlannedItems))
		for _, item := range e.PlannedItems {
			w.PlannedItems = append(w.PlannedItems, plannedItemJSON{ItemKey: item.ItemKey, Reason: string(item.Reason)})
		}
	case domain.PracticeItemAnsweredEvent:
		w.PracticeSessionID = e.PracticeSessionID
		if e.TriggerContext != nil {
			w.TriggerContext = toTriggerContextJSON(*e.TriggerContext)
		}
		w.ItemKey = e.ItemKey
		w.Response = toPracticeResponseJSON(e.Response)
		w.TapMs = e.TapMs
	case domain.PracticeSessionEndedEvent:
		w.PracticeSessionID = e.PracticeSessionID
		w.AnsweredCount = &e.AnsweredCount
		w.LeftEarly = &e.LeftEarly
		felt := make([]feltRatingJSON, 0, len(e.FeltRatings))
		for _, r := range e.FeltRatings {
			felt = append(felt, feltRatingJSON{DrillTemplateKey: r.DrillTemplateKey, Felt: string(r.Felt)})
		}
		w.FeltRatings = &felt
	case domain.PracticeTapCheckCompletedEvent:
		w.MedianTapMs = &e.MedianTapMs
		w.TapCount = &e.TapCount
	case domain.SongChartOpenedEvent, domain.SongChartChordViewedEvent, domain.SongChartCompletedEvent:
		addSongChartJSON(&w, event)
	}

	return w
}

func toPracticeResponseJSON(r domain.PracticeResponse) *practiceResponseJSON {
	return &practiceResponseJSON{
		ResponseType:     string(r.Type),
		NoteName:         r.NoteName,
		Shape:            r.Shape,
		Interval:         r.Interval,
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

func toContentContextJSON(cc domain.ContentContext) *contentContextJSON {
	return &contentContextJSON{
		ContentNodeID: cc.ContentNodeID,
		ContentType:   string(cc.ContentType),
		TeacherID:     cc.TeacherID,
		Review:        cc.Review,
	}
}

func toTriggerContextJSON(tc domain.TriggerContext) *triggerContextJSON {
	return &triggerContextJSON{
		Source:        string(tc.Source),
		ContentNodeID: tc.ContentNodeID,
		ChallengeID:   tc.ChallengeID,
		Review:        tc.Review,
	}
}

// toGeneratedTrackingEvent is event as the client submitted it, in the
// generated union the event log returns it in.
func toGeneratedTrackingEvent(event domain.TrackingEvent) (generated.TrackingEvent, error) {
	raw, err := json.Marshal(toLoggedEventJSON(event))
	if err != nil {
		return generated.TrackingEvent{}, err
	}
	var out generated.TrackingEvent
	if err := out.UnmarshalJSON(raw); err != nil {
		return generated.TrackingEvent{}, err
	}
	return out, nil
}

// addSongChartJSON sets the fields of a song_chart.* event.
func addSongChartJSON(w *loggedEventJSON, event domain.TrackingEvent) {
	switch e := event.(type) {
	case domain.SongChartOpenedEvent:
		w.SongChartContext = toSongChartContextJSON(e.SongChartContext)
	case domain.SongChartChordViewedEvent:
		w.SongChartContext = toSongChartContextJSON(e.SongChartContext)
		w.Source = string(e.Source)
		w.AnchorID = e.AnchorID
		w.ChordDefinitionID = e.ChordDefinitionID
		w.ChordVoicingID = e.ChordVoicingID
	case domain.SongChartCompletedEvent:
		w.SongChartContext = toSongChartContextJSON(e.SongChartContext)
	}
}

func toSongChartContextJSON(c domain.SongChartContext) *songChartContextJSON {
	return &songChartContextJSON{SongChartID: c.SongChartID, RevisionNumber: c.RevisionNumber}
}
