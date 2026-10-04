package kafka

import (
	"encoding/json"
	"time"
)

// wireEvent decodes only the fields this worker needs from a motifpath.events
// message. It intentionally does not model the full event schema — see the
// Event Ingestion Service's own wireEvent for that — because this worker only
// derives state from lesson events (content_context.content_node_id) and
// practice answers (the item and the raw response).
type wireEvent struct {
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	StudentID      string    `json:"student_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	ContentContext *struct {
		ContentNodeID string `json:"content_node_id"`
	} `json:"content_context,omitempty"`

	PracticeSessionID string                `json:"practice_session_id,omitempty"`
	ItemKey           string                `json:"item_key,omitempty"`
	Response          *practiceResponseWire `json:"response,omitempty"`
	TapMs             *int                  `json:"tap_ms,omitempty"`
}

// practiceResponseWire is the raw response, flat, with only its shape's fields.
type practiceResponseWire struct {
	ResponseType     string   `json:"response_type"`
	NoteName         string   `json:"note_name,omitempty"`
	String           *int     `json:"string,omitempty"`
	Fret             *int     `json:"fret,omitempty"`
	OptionIDs        []string `json:"option_ids,omitempty"`
	LatencyMs        *int     `json:"latency_ms,omitempty"`
	Rating           string   `json:"rating,omitempty"`
	TempoBPM         *int     `json:"tempo_bpm,omitempty"`
	ChangesPerMinute *int     `json:"changes_per_minute,omitempty"`
}

func decodeWireEvent(payload []byte) (wireEvent, error) {
	var w wireEvent
	err := json.Unmarshal(payload, &w)
	return w, err
}
