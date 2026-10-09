package kafka

import (
	"encoding/json"
	"time"
)

// wireEvent decodes only the fields this worker needs from a motifpath.events
// message. It intentionally does not model the full event schema — see the
// Event Ingestion Service's own wireEvent for that — because this worker only
// derives state from lesson events (content_context.content_node_id) and
// practice events (sessions, the item answered and the raw response), and the song
// chart a song_chart.completed event names.
type wireEvent struct {
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	StudentID      string    `json:"student_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	ContentContext *struct {
		ContentNodeID string `json:"content_node_id"`
	} `json:"content_context,omitempty"`

	PracticeSessionID string                `json:"practice_session_id,omitempty"`
	TriggerContext    *triggerContextWire   `json:"trigger_context,omitempty"`
	ItemKey           string                `json:"item_key,omitempty"`
	Response          *practiceResponseWire `json:"response,omitempty"`
	TapMs             *int                  `json:"tap_ms,omitempty"`

	InstrumentID  string            `json:"instrument_id,omitempty"`
	Minutes       int               `json:"minutes,omitempty"`
	PlannedItems  []plannedItemWire `json:"planned_items,omitempty"`
	AnsweredCount int               `json:"answered_count,omitempty"`
	LeftEarly     bool              `json:"left_early,omitempty"`
	FeltRatings   []feltRatingWire  `json:"felt_ratings,omitempty"`

	MedianTapMs int `json:"median_tap_ms,omitempty"`
	TapCount    int `json:"tap_count,omitempty"`

	SongChartContext *struct {
		SongChartID string `json:"song_chart_id"`
	} `json:"song_chart_context,omitempty"`
}

// feltRatingWire is how a timed drill felt to the student in a session.
type feltRatingWire struct {
	DrillTemplateKey string `json:"drill_template_key"`
	Felt             string `json:"felt"`
}

// triggerContextWire is where an answer outside a practice session was given.
type triggerContextWire struct {
	Source        string `json:"source"`
	ContentNodeID string `json:"content_node_id,omitempty"`
	ChallengeID   string `json:"challenge_id,omitempty"`
}

type plannedItemWire struct {
	ItemKey string `json:"item_key"`
	Reason  string `json:"reason"`
}

// practiceResponseWire is the raw response, flat, with only its shape's fields.
type practiceResponseWire struct {
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

func decodeWireEvent(payload []byte) (wireEvent, error) {
	var w wireEvent
	err := json.Unmarshal(payload, &w)
	return w, err
}
