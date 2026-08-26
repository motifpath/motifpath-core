package kafka

import "encoding/json"

// wireEvent decodes only the fields this worker needs from a motifpath.events
// message. It intentionally does not model the full seven-event-type schema —
// see the Event Ingestion Service's own wireEvent for that — because this
// worker only derives state from event_type, student_id, and (for lesson
// events) content_context.content_node_id.
type wireEvent struct {
	EventType      string `json:"event_type"`
	StudentID      string `json:"student_id"`
	ContentContext *struct {
		ContentNodeID string `json:"content_node_id"`
	} `json:"content_context,omitempty"`
}

func decodeWireEvent(payload []byte) (wireEvent, error) {
	var w wireEvent
	err := json.Unmarshal(payload, &w)
	return w, err
}
