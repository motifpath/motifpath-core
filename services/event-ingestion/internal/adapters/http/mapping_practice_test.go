package http

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

const (
	testEventID           = "6f1c1d1e-1111-4111-8111-111111111111"
	testStudentID         = "6f1c1d1e-2222-4222-8222-222222222222"
	testSessionID         = "6f1c1d1e-3333-4333-8333-333333333333"
	testPracticeSessionID = "6f1c1d1e-4444-4444-8444-444444444444"
	testInstrumentID      = "6ea2d087-ab9c-59dc-9657-8546025414d2"
	testDiagramID         = "6f1c1d1e-5555-4555-8555-555555555555"
	testOptionID          = "6f1c1d1e-6666-4666-8666-666666666666"
	testChallengeID       = "6f1c1d1e-7777-4777-8777-777777777777"
	testNodeID            = "6f1c1d1e-8888-4888-8888-888888888888"
	testOccurredAt        = "2026-10-04T10:00:00Z"
)

var testCellKey = "fretboard_cell:" + testInstrumentID + ":5:3"

// practiceBody builds a raw practice.* event, so the tests exercise the same JSON
// decoding the router performs — including fields the generated structs drop.
func practiceBody(t *testing.T, eventType, fields string) *generated.TrackingEvent {
	t.Helper()
	raw := fmt.Sprintf(`{"event_id":%q,"event_type":%q,"student_id":%q,"session_id":%q,"occurred_at":%q%s}`,
		testEventID, eventType, testStudentID, testSessionID, testOccurredAt, fields)
	body := &generated.TrackingEvent{}
	require.NoError(t, body.UnmarshalJSON([]byte(raw)))
	return body
}

func answeredFields(itemKey, response string) string {
	return fmt.Sprintf(`,"practice_session_id":%q,"item_key":%q,"response":%s`, testPracticeSessionID, itemKey, response)
}

func intPtr(v int) *int { return &v }

func TestToDomainEvent_PracticeSessionStarted(t *testing.T) {
	body := practiceBody(t, "practice.session_started", fmt.Sprintf(
		`,"practice_session_id":%q,"instrument_id":%q,"minutes":10,"planned_items":[{"item_key":%q,"reason":"warm_up"},{"item_key":%q,"reason":"new"}]`,
		testPracticeSessionID, testInstrumentID, "play_along:"+testDiagramID, testCellKey))

	event, err := toDomainEvent(body)

	require.NoError(t, err)
	started, ok := event.(domain.PracticeSessionStartedEvent)
	require.True(t, ok, "got %T", event)
	assert.Equal(t, domain.EventTypePracticeSessionStarted, started.EventType)
	assert.Equal(t, testPracticeSessionID, started.PracticeSessionID)
	assert.Equal(t, testInstrumentID, started.InstrumentID)
	assert.Equal(t, 10, started.Minutes)
	assert.Equal(t, []domain.PlannedPracticeItem{
		{ItemKey: "play_along:" + testDiagramID, Reason: domain.PracticePickReasonWarmUp},
		{ItemKey: testCellKey, Reason: domain.PracticePickReasonNew},
	}, started.PlannedItems)
}

func TestToDomainEvent_PracticeSessionStartedInTheHead(t *testing.T) {
	body := practiceBody(t, "practice.session_started", fmt.Sprintf(
		`,"practice_session_id":%q,"minutes":5,"planned_items":[{"item_key":%q,"reason":"due"}]`,
		testPracticeSessionID, testCellKey))

	event, err := toDomainEvent(body)

	require.NoError(t, err)
	assert.Empty(t, event.(domain.PracticeSessionStartedEvent).InstrumentID)
}

func TestToDomainEvent_PracticeItemAnsweredResponses(t *testing.T) {
	cases := []struct {
		name     string
		itemKey  string
		response string
		want     domain.PracticeResponse
	}{
		{
			name:     "name the note",
			itemKey:  testCellKey,
			response: `{"response_type":"name_the_note","note_name":"C","latency_ms":1800}`,
			want:     domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: intPtr(1800)},
		},
		{
			name:     "find the note",
			itemKey:  testCellKey,
			response: `{"response_type":"find_the_note","string":5,"fret":3,"latency_ms":2100}`,
			want:     domain.PracticeResponse{Type: domain.PracticeResponseFindTheNote, String: intPtr(5), Fret: intPtr(3), LatencyMs: intPtr(2100)},
		},
		{
			name:     "find the note on an open string",
			itemKey:  testCellKey,
			response: `{"response_type":"find_the_note","string":6,"fret":0,"latency_ms":900}`,
			want:     domain.PracticeResponse{Type: domain.PracticeResponseFindTheNote, String: intPtr(6), Fret: intPtr(0), LatencyMs: intPtr(900)},
		},
		{
			name:     "option choice",
			itemKey:  "exercise:" + testDiagramID,
			response: fmt.Sprintf(`{"response_type":"option_choice","option_ids":[%q],"latency_ms":4000}`, testOptionID),
			want:     domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice, OptionIDs: []string{testOptionID}, LatencyMs: intPtr(4000)},
		},
		{
			name:     "option choice with audio",
			itemKey:  "exercise:" + testDiagramID,
			response: fmt.Sprintf(`{"response_type":"option_choice","option_ids":[%q],"latency_ms":9500,"audio_ms":5000}`, testOptionID),
			want:     domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice, OptionIDs: []string{testOptionID}, LatencyMs: intPtr(9500), AudioMs: intPtr(5000)},
		},
		{
			name:     "self rating of a play-along",
			itemKey:  "play_along:" + testDiagramID,
			response: `{"response_type":"self_rating","rating":"clean","tempo_bpm":90}`,
			want:     domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: domain.SelfRatingClean, TempoBPM: intPtr(90)},
		},
		{
			name:     "self rating of a chord change",
			itemKey:  "chord_change:" + testDiagramID + ":" + testOptionID,
			response: `{"response_type":"self_rating","rating":"almost","changes_per_minute":42}`,
			want:     domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: domain.SelfRatingAlmost, ChangesPerMinute: intPtr(42)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event, err := toDomainEvent(practiceBody(t, "practice.item_answered", answeredFields(tc.itemKey, tc.response)))

			require.NoError(t, err)
			answered, ok := event.(domain.PracticeItemAnsweredEvent)
			require.True(t, ok, "got %T", event)
			assert.Equal(t, domain.EventTypePracticeItemAnswered, answered.EventType)
			assert.Equal(t, testPracticeSessionID, answered.PracticeSessionID)
			assert.Equal(t, tc.itemKey, answered.ItemKey)
			assert.Equal(t, tc.want, answered.Response)
			assert.Nil(t, answered.TapMs)
		})
	}
}

func TestToDomainEvent_PracticeItemAnsweredInAChallenge(t *testing.T) {
	fields := fmt.Sprintf(`,"trigger_context":{"source":"challenge_sequence","challenge_id":%q,"content_node_id":%q},"item_key":%q,"response":{"response_type":"option_choice","option_ids":[%q],"latency_ms":3500}`,
		testChallengeID, testNodeID, "exercise:"+testDiagramID, testOptionID)

	event, err := toDomainEvent(practiceBody(t, "practice.item_answered", fields))

	require.NoError(t, err)
	answered, ok := event.(domain.PracticeItemAnsweredEvent)
	require.True(t, ok, "got %T", event)
	assert.Empty(t, answered.PracticeSessionID)
	assert.Equal(t, &domain.TriggerContext{
		Source:        domain.TriggerSourceChallengeSequence,
		ChallengeID:   testChallengeID,
		ContentNodeID: testNodeID,
	}, answered.TriggerContext)
}

func TestToDomainEvent_PracticeItemAnsweredIgnoresClientTapTime(t *testing.T) {
	fields := answeredFields(testCellKey, `{"response_type":"name_the_note","note_name":"C","latency_ms":1800}`) + `,"tap_ms":5`

	event, err := toDomainEvent(practiceBody(t, "practice.item_answered", fields))

	require.NoError(t, err)
	assert.Nil(t, event.(domain.PracticeItemAnsweredEvent).TapMs, "tap_ms is server-set; a client value must be dropped")
}

func TestToDomainEvent_PracticeSessionEnded(t *testing.T) {
	body := practiceBody(t, "practice.session_ended", fmt.Sprintf(
		`,"practice_session_id":%q,"answered_count":6,"left_early":false,"felt_ratings":[{"drill_template_key":"fretboard_cell:name_the_note","felt":"about_right"}]`,
		testPracticeSessionID))

	event, err := toDomainEvent(body)

	require.NoError(t, err)
	ended, ok := event.(domain.PracticeSessionEndedEvent)
	require.True(t, ok, "got %T", event)
	assert.Equal(t, testPracticeSessionID, ended.PracticeSessionID)
	assert.Equal(t, 6, ended.AnsweredCount)
	assert.False(t, ended.LeftEarly)
	assert.Equal(t, []domain.FeltRating{{DrillTemplateKey: "fretboard_cell:name_the_note", Felt: domain.FeltAboutRight}}, ended.FeltRatings)
}

func TestToDomainEvent_PracticeSessionEndedEarlyWithoutFeltRatings(t *testing.T) {
	body := practiceBody(t, "practice.session_ended", fmt.Sprintf(
		`,"practice_session_id":%q,"answered_count":2,"left_early":true,"felt_ratings":[]`, testPracticeSessionID))

	event, err := toDomainEvent(body)

	require.NoError(t, err)
	ended := event.(domain.PracticeSessionEndedEvent)
	assert.True(t, ended.LeftEarly)
	assert.Empty(t, ended.FeltRatings)
}

func TestToDomainEvent_PracticeTapCheckCompleted(t *testing.T) {
	event, err := toDomainEvent(practiceBody(t, "practice.tap_check_completed", `,"median_tap_ms":350,"tap_count":24`))

	require.NoError(t, err)
	tap, ok := event.(domain.PracticeTapCheckCompletedEvent)
	require.True(t, ok, "got %T", event)
	assert.Equal(t, domain.EventTypePracticeTapCheckCompleted, tap.EventType)
	assert.Equal(t, 350, tap.MedianTapMs)
	assert.Equal(t, 24, tap.TapCount)
}

func TestToDomainEvent_PracticeValidationFailures(t *testing.T) {
	nameTheNote := `{"response_type":"name_the_note","note_name":"C","latency_ms":1800}`
	planned := fmt.Sprintf(`[{"item_key":%q,"reason":"due"}]`, testCellKey)

	cases := []struct {
		name      string
		eventType string
		fields    string
		wantErr   error
		wantField string
	}{
		// practice.session_started
		{"started without a practice session id", "practice.session_started",
			`,"minutes":10,"planned_items":` + planned, domain.ErrMissingRequiredField, "practice_session_id"},
		{"started without minutes", "practice.session_started",
			fmt.Sprintf(`,"practice_session_id":%q,"planned_items":%s`, testPracticeSessionID, planned), domain.ErrMissingRequiredField, "minutes"},
		{"started with zero minutes", "practice.session_started",
			fmt.Sprintf(`,"practice_session_id":%q,"minutes":0,"planned_items":%s`, testPracticeSessionID, planned), domain.ErrInvalidField, "minutes"},
		{"started with more than 60 minutes", "practice.session_started",
			fmt.Sprintf(`,"practice_session_id":%q,"minutes":61,"planned_items":%s`, testPracticeSessionID, planned), domain.ErrInvalidField, "minutes"},
		{"started with no planned items", "practice.session_started",
			fmt.Sprintf(`,"practice_session_id":%q,"minutes":10,"planned_items":[]`, testPracticeSessionID), domain.ErrMissingRequiredField, "planned_items"},
		{"started with a planned item key of no kind", "practice.session_started",
			fmt.Sprintf(`,"practice_session_id":%q,"minutes":10,"planned_items":[{"item_key":"fretboard:x","reason":"due"}]`, testPracticeSessionID), domain.ErrInvalidField, "planned_items"},
		{"started with an unknown pick reason", "practice.session_started",
			fmt.Sprintf(`,"practice_session_id":%q,"minutes":10,"planned_items":[{"item_key":%q,"reason":"because"}]`, testPracticeSessionID, testCellKey), domain.ErrInvalidField, "planned_items"},

		// practice.item_answered
		{"answered without a practice session id or a trigger context", "practice.item_answered",
			fmt.Sprintf(`,"item_key":%q,"response":%s`, testCellKey, nameTheNote), domain.ErrMissingRequiredField, "practice_session_id"},
		{"answered with both a practice session id and a trigger context", "practice.item_answered",
			answeredFields(testCellKey, nameTheNote) + fmt.Sprintf(`,"trigger_context":{"source":"challenge_sequence","challenge_id":%q}`, testChallengeID),
			domain.ErrInvalidField, "trigger_context"},
		{"answered with a trigger context without a source", "practice.item_answered",
			fmt.Sprintf(`,"trigger_context":{},"item_key":%q,"response":%s`, testCellKey, nameTheNote), domain.ErrMissingRequiredField, "trigger_context"},
		{"answered with a negative audio length", "practice.item_answered",
			answeredFields("exercise:"+testDiagramID, fmt.Sprintf(`{"response_type":"option_choice","option_ids":[%q],"latency_ms":900,"audio_ms":-1}`, testOptionID)), domain.ErrInvalidField, "response"},
		{"answered naming a note with an audio length", "practice.item_answered",
			answeredFields(testCellKey, `{"response_type":"name_the_note","note_name":"C","latency_ms":1800,"audio_ms":500}`), domain.ErrInvalidField, "response"},
		{"answered with an item key of no kind", "practice.item_answered",
			answeredFields("fretboard:"+testInstrumentID+":5:3", nameTheNote), domain.ErrInvalidField, "item_key"},
		{"answered with a cell key on string 0", "practice.item_answered",
			answeredFields("fretboard_cell:"+testInstrumentID+":0:3", nameTheNote), domain.ErrInvalidField, "item_key"},
		{"answered without a response", "practice.item_answered",
			fmt.Sprintf(`,"practice_session_id":%q,"item_key":%q`, testPracticeSessionID, testCellKey), domain.ErrMissingRequiredField, "response"},
		{"answered with an unknown response type", "practice.item_answered",
			answeredFields(testCellKey, `{"response_type":"hum_it"}`), domain.ErrInvalidField, "response"},
		{"answered naming a note that isn't a note", "practice.item_answered",
			answeredFields(testCellKey, `{"response_type":"name_the_note","note_name":"H","latency_ms":1800}`), domain.ErrInvalidField, "response"},
		{"answered naming a note with an octave", "practice.item_answered",
			answeredFields(testCellKey, `{"response_type":"name_the_note","note_name":"C4","latency_ms":1800}`), domain.ErrInvalidField, "response"},
		{"answered naming a note without a latency", "practice.item_answered",
			answeredFields(testCellKey, `{"response_type":"name_the_note","note_name":"C"}`), domain.ErrInvalidField, "response"},
		{"answered with a negative latency", "practice.item_answered",
			answeredFields(testCellKey, `{"response_type":"name_the_note","note_name":"C","latency_ms":-1}`), domain.ErrInvalidField, "response"},
		{"answered with a response that claims it is correct", "practice.item_answered",
			answeredFields(testCellKey, `{"response_type":"name_the_note","note_name":"C","latency_ms":1800,"correct":true}`), domain.ErrInvalidField, "response"},
		{"answered finding a note on string 0", "practice.item_answered",
			answeredFields(testCellKey, `{"response_type":"find_the_note","string":0,"fret":3,"latency_ms":900}`), domain.ErrInvalidField, "response"},
		{"answered finding a note without a fret", "practice.item_answered",
			answeredFields(testCellKey, `{"response_type":"find_the_note","string":5,"latency_ms":900}`), domain.ErrInvalidField, "response"},
		{"answered choosing no option", "practice.item_answered",
			answeredFields("exercise:"+testDiagramID, `{"response_type":"option_choice","option_ids":[],"latency_ms":900}`), domain.ErrInvalidField, "response"},
		{"answered choosing the same option twice", "practice.item_answered",
			answeredFields("exercise:"+testDiagramID, fmt.Sprintf(`{"response_type":"option_choice","option_ids":[%q,%q],"latency_ms":900}`, testOptionID, testOptionID)), domain.ErrInvalidField, "response"},
		{"answered with an unknown rating", "practice.item_answered",
			answeredFields("play_along:"+testDiagramID, `{"response_type":"self_rating","rating":"perfect","tempo_bpm":90}`), domain.ErrInvalidField, "response"},
		{"answered with a tempo above 400", "practice.item_answered",
			answeredFields("play_along:"+testDiagramID, `{"response_type":"self_rating","rating":"clean","tempo_bpm":401}`), domain.ErrInvalidField, "response"},
		{"answered with a self rating claiming a latency", "practice.item_answered",
			answeredFields("play_along:"+testDiagramID, `{"response_type":"self_rating","rating":"clean","tempo_bpm":90,"latency_ms":10}`), domain.ErrInvalidField, "response"},

		// practice.session_ended
		{"ended without a practice session id", "practice.session_ended",
			`,"answered_count":2,"left_early":true,"felt_ratings":[]`, domain.ErrMissingRequiredField, "practice_session_id"},
		{"ended with a negative answered count", "practice.session_ended",
			fmt.Sprintf(`,"practice_session_id":%q,"answered_count":-1,"left_early":true,"felt_ratings":[]`, testPracticeSessionID), domain.ErrInvalidField, "answered_count"},
		{"ended with three felt ratings", "practice.session_ended",
			fmt.Sprintf(`,"practice_session_id":%q,"answered_count":6,"left_early":false,"felt_ratings":[%s,%s,%s]`, testPracticeSessionID,
				`{"drill_template_key":"fretboard_cell:name_the_note","felt":"easy"}`,
				`{"drill_template_key":"fretboard_cell:find_the_note","felt":"hard"}`,
				`{"drill_template_key":"exercise:interval_recognition","felt":"easy"}`), domain.ErrInvalidField, "felt_ratings"},
		{"ended with an unknown felt answer", "practice.session_ended",
			fmt.Sprintf(`,"practice_session_id":%q,"answered_count":6,"left_early":false,"felt_ratings":[{"drill_template_key":"fretboard_cell:name_the_note","felt":"meh"}]`, testPracticeSessionID), domain.ErrInvalidField, "felt_ratings"},
		{"ended with a malformed drill template key", "practice.session_ended",
			fmt.Sprintf(`,"practice_session_id":%q,"answered_count":6,"left_early":false,"felt_ratings":[{"drill_template_key":"Name The Note","felt":"easy"}]`, testPracticeSessionID), domain.ErrInvalidField, "felt_ratings"},

		{"ended without an answered count", "practice.session_ended",
			fmt.Sprintf(`,"practice_session_id":%q,"left_early":true,"felt_ratings":[]`, testPracticeSessionID), domain.ErrMissingRequiredField, "answered_count"},
		{"ended without saying whether it was left early", "practice.session_ended",
			fmt.Sprintf(`,"practice_session_id":%q,"answered_count":2,"felt_ratings":[]`, testPracticeSessionID), domain.ErrMissingRequiredField, "left_early"},
		{"ended without felt ratings", "practice.session_ended",
			fmt.Sprintf(`,"practice_session_id":%q,"answered_count":2,"left_early":true`, testPracticeSessionID), domain.ErrMissingRequiredField, "felt_ratings"},
		{"ended with null felt ratings", "practice.session_ended",
			fmt.Sprintf(`,"practice_session_id":%q,"answered_count":2,"left_early":true,"felt_ratings":null`, testPracticeSessionID), domain.ErrMissingRequiredField, "felt_ratings"},

		// practice.tap_check_completed
		{"tap check without a median", "practice.tap_check_completed",
			`,"tap_count":24`, domain.ErrMissingRequiredField, "median_tap_ms"},
		{"tap check without a tap count", "practice.tap_check_completed",
			`,"median_tap_ms":350`, domain.ErrMissingRequiredField, "tap_count"},
		{"tap check with a negative median", "practice.tap_check_completed",
			`,"median_tap_ms":-5,"tap_count":24`, domain.ErrInvalidField, "median_tap_ms"},
		{"tap check over no taps", "practice.tap_check_completed",
			`,"median_tap_ms":350,"tap_count":0`, domain.ErrInvalidField, "tap_count"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := toDomainEvent(practiceBody(t, tc.eventType, tc.fields))

			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.wantErr), "want %v, got %v", tc.wantErr, err)
			assert.True(t, strings.Contains(err.Error(), tc.wantField), "error %q should name %q", err, tc.wantField)
		})
	}
}
