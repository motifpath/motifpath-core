package kafka

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

func intPtr(v int) *int { return &v }

// The payloads are the ones event-ingestion publishes (its wire_event_practice_test).
func TestToDomainEvent_PracticeItemAnswered(t *testing.T) {
	const base = `"event_id":"11111111-1111-4111-8111-111111111111","student_id":"22222222-2222-4222-8222-222222222222","session_id":"33333333-3333-4333-8333-333333333333","occurred_at":"2026-10-04T10:00:00Z","event_type":"practice.item_answered","practice_session_id":"44444444-4444-4444-8444-444444444444"`
	occurredAt := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		payload string
		want    domain.PracticeAnswer
	}{
		{
			name:    "self-rated take",
			payload: `{` + base + `,"item_key":"play_along:55555555-5555-4555-8555-555555555555","response":{"response_type":"self_rating","rating":"clean","tempo_bpm":90}}`,
			want: domain.PracticeAnswer{
				ItemKey:  "play_along:55555555-5555-4555-8555-555555555555",
				Response: domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: domain.SelfRatingClean, TempoBPM: intPtr(90)},
			},
		},
		{
			name:    "timed answer with its tap",
			payload: `{` + base + `,"item_key":"fretboard_cell:6ea2d087-ab9c-59dc-9657-8546025414d2:5:3","response":{"response_type":"name_the_note","note_name":"C","latency_ms":1800},"tap_ms":350}`,
			want: domain.PracticeAnswer{
				ItemKey:  "fretboard_cell:6ea2d087-ab9c-59dc-9657-8546025414d2:5:3",
				Response: domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: intPtr(1800)},
				TapMs:    intPtr(350),
			},
		},
		{
			name:    "found note and chosen options",
			payload: `{` + base + `,"item_key":"exercise:55555555-5555-4555-8555-555555555555","response":{"response_type":"option_choice","option_ids":["a","b"],"latency_ms":900,"string":6,"fret":0}}`,
			want: domain.PracticeAnswer{
				ItemKey:  "exercise:55555555-5555-4555-8555-555555555555",
				Response: domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice, OptionIDs: []string{"a", "b"}, LatencyMs: intPtr(900), String: intPtr(6), Fret: intPtr(0)},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wire, err := decodeWireEvent([]byte(c.payload))
			require.NoError(t, err)

			event := toDomainEvent(wire)

			assert.Equal(t, domain.EventTypePracticeItemAnswered, event.EventType)
			assert.Equal(t, "22222222-2222-4222-8222-222222222222", event.StudentID)
			require.NotNil(t, event.PracticeAnswer)
			want := c.want
			want.EventID = "11111111-1111-4111-8111-111111111111"
			want.StudentID = "22222222-2222-4222-8222-222222222222"
			want.OccurredAt = occurredAt
			want.PracticeSessionID = "44444444-4444-4444-8444-444444444444"
			assert.Equal(t, want, *event.PracticeAnswer)
		})
	}
}

func TestToDomainEvent_PracticeItemAnsweredWithoutAResponseCarriesNoAnswer(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_type":"practice.item_answered","student_id":"s","item_key":"k"}`))
	require.NoError(t, err)

	assert.Nil(t, toDomainEvent(wire).PracticeAnswer)
}

func TestToDomainEvent_OtherEventsCarryNoPracticeAnswer(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_type":"lesson.started","student_id":"s","content_context":{"content_node_id":"n"}}`))
	require.NoError(t, err)

	event := toDomainEvent(wire)
	assert.Equal(t, "n", event.ContentNodeID)
	assert.Nil(t, event.PracticeAnswer)
}

func TestToDomainEvent_CarriesTheEnvelope(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_id":"11111111-1111-4111-8111-111111111111","event_type":"lesson.completed","student_id":"s","occurred_at":"2026-10-05T19:30:00Z","content_context":{"content_node_id":"n"},"duration_seconds":30}`))
	require.NoError(t, err)

	event := toDomainEvent(wire)

	assert.Equal(t, domain.EventTypeLessonCompleted, event.EventType)
	assert.Equal(t, "11111111-1111-4111-8111-111111111111", event.EventID)
	assert.Equal(t, time.Date(2026, 10, 5, 19, 30, 0, 0, time.UTC), event.OccurredAt)
	assert.Equal(t, "n", event.ContentNodeID)
}

func TestToDomainEvent_PracticeSessionStarted(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_id":"11111111-1111-4111-8111-111111111111","event_type":"practice.session_started","student_id":"s","occurred_at":"2026-10-05T18:00:00Z","practice_session_id":"p","instrument_id":"g","minutes":10,"planned_items":[{"item_key":"play_along:d1","reason":"due"},{"item_key":"play_along:d2","reason":"new"}]}`))
	require.NoError(t, err)

	event := toDomainEvent(wire)

	assert.Equal(t, "p", event.PracticeSessionID)
	assert.Equal(t, &domain.PracticeSessionStart{
		EventID:           "11111111-1111-4111-8111-111111111111",
		StudentID:         "s",
		PracticeSessionID: "p",
		OccurredAt:        time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC),
		InstrumentID:      "g",
		Minutes:           10,
		PlannedItems: []domain.PlannedPracticeItem{
			{ItemKey: "play_along:d1", Reason: "due"},
			{ItemKey: "play_along:d2", Reason: "new"},
		},
	}, event.SessionStart)
	assert.Nil(t, event.SessionEnd)
}

func TestToDomainEvent_PracticeSessionEnded(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_id":"11111111-1111-4111-8111-111111111111","event_type":"practice.session_ended","student_id":"s","occurred_at":"2026-10-05T18:11:00Z","practice_session_id":"p","answered_count":6,"left_early":true,"felt_ratings":[]}`))
	require.NoError(t, err)

	event := toDomainEvent(wire)

	assert.Equal(t, &domain.PracticeSessionEnd{
		EventID:           "11111111-1111-4111-8111-111111111111",
		StudentID:         "s",
		PracticeSessionID: "p",
		OccurredAt:        time.Date(2026, 10, 5, 18, 11, 0, 0, time.UTC),
		LeftEarly:         true,
		AnsweredCount:     6,
	}, event.SessionEnd)
	assert.Nil(t, event.SessionStart)
}

func TestToDomainEvent_AChallengeAnswerCarriesItsTriggerContextAndAudio(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_type":"practice.item_answered","student_id":"s","trigger_context":{"source":"challenge_sequence","challenge_id":"c","content_node_id":"n"},"item_key":"exercise:e","response":{"response_type":"option_choice","option_ids":["a"],"latency_ms":9500,"audio_ms":5000}}`))
	require.NoError(t, err)

	event := toDomainEvent(wire)

	require.NotNil(t, event.PracticeAnswer)
	assert.Equal(t, &domain.TriggerContext{Source: "challenge_sequence", ChallengeID: "c", ContentNodeID: "n"}, event.PracticeAnswer.TriggerContext)
	assert.Equal(t, intPtr(5000), event.PracticeAnswer.Response.AudioMs)
}

func TestToDomainEvent_AnAnswerInASessionHasNoTriggerContext(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_type":"practice.item_answered","student_id":"s","practice_session_id":"p","item_key":"exercise:e","response":{"response_type":"option_choice","option_ids":["a"],"latency_ms":900}}`))
	require.NoError(t, err)

	event := toDomainEvent(wire)

	require.NotNil(t, event.PracticeAnswer)
	assert.Nil(t, event.PracticeAnswer.TriggerContext)
}

func TestToDomainEvent_AnAnswerOutsideASessionHasNoSession(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_type":"practice.item_answered","student_id":"s","trigger_context":{"source":"challenge_sequence"},"item_key":"exercise:e","response":{"response_type":"option_choice","option_ids":["a"]}}`))
	require.NoError(t, err)

	event := toDomainEvent(wire)

	assert.Empty(t, event.PracticeSessionID)
	require.NotNil(t, event.PracticeAnswer)
	assert.Empty(t, event.PracticeAnswer.PracticeSessionID)
}

func TestToDomainEvent_APracticeSessionEndCarriesItsFeltRatings(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_id":"11111111-1111-4111-8111-111111111111","event_type":"practice.session_ended","student_id":"s","occurred_at":"2026-10-05T18:11:00Z","practice_session_id":"p","answered_count":6,"left_early":false,"felt_ratings":[{"drill_template_key":"fretboard_cell:name_the_note","felt":"hard"},{"drill_template_key":"exercise:text_response","felt":"about_right"}]}`))
	require.NoError(t, err)

	event := toDomainEvent(wire)

	require.NotNil(t, event.SessionEnd)
	assert.Equal(t, []domain.FeltRating{
		{DrillTemplateKey: "fretboard_cell:name_the_note", Felt: domain.FeltHard},
		{DrillTemplateKey: "exercise:text_response", Felt: domain.FeltAboutRight},
	}, event.SessionEnd.FeltRatings)
}

func TestToDomainEvent_ATapCheckCarriesItsMedianAndTapCount(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_id":"11111111-1111-4111-8111-111111111111","event_type":"practice.tap_check_completed","student_id":"s","occurred_at":"2026-10-06T09:00:00Z","median_tap_ms":320,"tap_count":24}`))
	require.NoError(t, err)

	event := toDomainEvent(wire)

	assert.Equal(t, &domain.TapCheck{
		EventID:     "11111111-1111-4111-8111-111111111111",
		StudentID:   "s",
		DoneAt:      time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC),
		MedianTapMs: 320,
		TapCount:    24,
	}, event.TapCheck)
}

func TestToDomainEvent_OtherEventsCarryNoTapCheck(t *testing.T) {
	wire, err := decodeWireEvent([]byte(`{"event_type":"practice.session_ended","student_id":"s","practice_session_id":"p"}`))
	require.NoError(t, err)

	assert.Nil(t, toDomainEvent(wire).TapCheck)
}
