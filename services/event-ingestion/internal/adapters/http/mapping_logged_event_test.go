package http

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoggedEvent_ReturnsEachEventAsSubmitted takes a submitted body of every
// event type through the domain and back: the event log must return every
// field it was submitted with, and nothing else.
func TestLoggedEvent_ReturnsEachEventAsSubmitted(t *testing.T) {
	content := fmt.Sprintf(`"content_context":{"content_node_id":%q,"content_type":"article","teacher_id":%q}`, testNodeID, testChallengeID)
	trigger := fmt.Sprintf(`"trigger_context":{"source":"challenge","challenge_id":%q}`, testChallengeID)
	song := fmt.Sprintf(`"song_chart_context":{"song_chart_id":%q,"revision_number":3}`, testDiagramID)
	cases := map[string]string{
		"lesson.started":               "," + content,
		"lesson.resumed":               "," + content,
		"lesson.completed":             "," + content + `,"duration_seconds":90`,
		"exercise.started":             fmt.Sprintf(`,"exercise_id":%q,%s`, testOptionID, trigger),
		"exercise.progress":            fmt.Sprintf(`,"exercise_id":%q,%s,"elapsed_seconds":30`, testOptionID, trigger),
		"exercise.ended":               fmt.Sprintf(`,"exercise_id":%q,%s,"outcome":"completed","final_score":80`, testOptionID, trigger),
		"practice.session_started":     fmt.Sprintf(`,"practice_session_id":%q,"instrument_id":%q,"minutes":10,"planned_items":[{"item_key":%q,"reason":"new"}]`, testPracticeSessionID, testInstrumentID, testCellKey),
		"practice.item_answered":       answeredFields(testCellKey, `{"response_type":"name_the_note","note_name":"C","latency_ms":1200}`),
		"practice.session_ended":       fmt.Sprintf(`,"practice_session_id":%q,"answered_count":9,"left_early":false,"felt_ratings":[]`, testPracticeSessionID),
		"practice.tap_check_completed": `,"median_tap_ms":320,"tap_count":24`,
		"song_chart.opened":            "," + song,
		"song_chart.chord_viewed":      fmt.Sprintf(`,%s,"source":"lyrics","anchor_id":"a1","chord_definition_id":%q,"chord_voicing_id":%q`, song, testNodeID, testOptionID),
		"song_chart.completed":         "," + song,
	}
	for eventType, fields := range cases {
		t.Run(eventType, func(t *testing.T) {
			body := practiceBody(t, eventType, fields)
			submitted, err := body.MarshalJSON()
			require.NoError(t, err)
			event, err := toDomainEvent(body)
			require.NoError(t, err)

			logged, err := toGeneratedTrackingEvent(event)
			require.NoError(t, err)

			returned, err := json.Marshal(logged)
			require.NoError(t, err)
			assert.JSONEq(t, string(submitted), string(returned))
		})
	}
}
