package kafka

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/domain"
)

func practiceWireBase(eventType domain.EventType) domain.TrackingEventBase {
	return domain.TrackingEventBase{
		EventID:    "11111111-1111-4111-8111-111111111111",
		EventType:  eventType,
		StudentID:  "22222222-2222-4222-8222-222222222222",
		SessionID:  "33333333-3333-4333-8333-333333333333",
		OccurredAt: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC),
	}
}

const wireBaseJSON = `"event_id":"11111111-1111-4111-8111-111111111111","student_id":"22222222-2222-4222-8222-222222222222","session_id":"33333333-3333-4333-8333-333333333333","occurred_at":"2026-10-04T10:00:00Z"`

func wireJSON(t *testing.T, event domain.TrackingEvent) string {
	t.Helper()
	b, err := json.Marshal(toWireEvent(event))
	require.NoError(t, err)
	return string(b)
}

func TestToWireEvent_PracticeEvents(t *testing.T) {
	latency, tap, tempo, str, fret, audio := 1800, 350, 90, 6, 0, 5000

	cases := []struct {
		name  string
		event domain.TrackingEvent
		want  string
	}{
		{
			name: "session started with an instrument",
			event: domain.PracticeSessionStartedEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypePracticeSessionStarted),
				PracticeSessionID: "44444444-4444-4444-8444-444444444444",
				InstrumentID:      "6ea2d087-ab9c-59dc-9657-8546025414d2",
				Minutes:           10,
				PlannedItems:      []domain.PlannedPracticeItem{{ItemKey: "play_along:55555555-5555-4555-8555-555555555555", Reason: domain.PracticePickReasonWarmUp}},
			},
			want: `{` + wireBaseJSON + `,"event_type":"practice.session_started","practice_session_id":"44444444-4444-4444-8444-444444444444","instrument_id":"6ea2d087-ab9c-59dc-9657-8546025414d2","minutes":10,"planned_items":[{"item_key":"play_along:55555555-5555-4555-8555-555555555555","reason":"warm_up"}]}`,
		},
		{
			name: "timed answer with its tap",
			event: domain.PracticeItemAnsweredEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypePracticeItemAnswered),
				PracticeSessionID: "44444444-4444-4444-8444-444444444444",
				ItemKey:           "fretboard_cell:6ea2d087-ab9c-59dc-9657-8546025414d2:5:3",
				Response:          domain.PracticeResponse{Type: domain.PracticeResponseNameTheNote, NoteName: "C", LatencyMs: &latency},
				TapMs:             &tap,
			},
			want: `{` + wireBaseJSON + `,"event_type":"practice.item_answered","practice_session_id":"44444444-4444-4444-8444-444444444444","item_key":"fretboard_cell:6ea2d087-ab9c-59dc-9657-8546025414d2:5:3","response":{"response_type":"name_the_note","note_name":"C","latency_ms":1800},"tap_ms":350}`,
		},
		{
			name: "named shape",
			event: domain.PracticeItemAnsweredEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypePracticeItemAnswered),
				PracticeSessionID: "44444444-4444-4444-8444-444444444444",
				ItemKey:           "diagram_shape:55555555-5555-4555-8555-555555555555",
				Response:          domain.PracticeResponse{Type: domain.PracticeResponseNameTheShape, Shape: "A", LatencyMs: &latency},
			},
			want: `{` + wireBaseJSON + `,"event_type":"practice.item_answered","practice_session_id":"44444444-4444-4444-8444-444444444444","item_key":"diagram_shape:55555555-5555-4555-8555-555555555555","response":{"response_type":"name_the_shape","shape":"A","latency_ms":1800}}`,
		},
		{
			name: "found degree",
			event: domain.PracticeItemAnsweredEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypePracticeItemAnswered),
				PracticeSessionID: "44444444-4444-4444-8444-444444444444",
				ItemKey:           "diagram_shape:55555555-5555-4555-8555-555555555555",
				Response:          domain.PracticeResponse{Type: domain.PracticeResponseFindTheDegree, Interval: "b3", String: &str, Fret: &fret, LatencyMs: &latency},
			},
			want: `{` + wireBaseJSON + `,"event_type":"practice.item_answered","practice_session_id":"44444444-4444-4444-8444-444444444444","item_key":"diagram_shape:55555555-5555-4555-8555-555555555555","response":{"response_type":"find_the_degree","interval":"b3","string":6,"fret":0,"latency_ms":1800}}`,
		},
		{
			name: "find the note on an open string",
			event: domain.PracticeItemAnsweredEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypePracticeItemAnswered),
				PracticeSessionID: "44444444-4444-4444-8444-444444444444",
				ItemKey:           "fretboard_cell:6ea2d087-ab9c-59dc-9657-8546025414d2:6:0",
				Response:          domain.PracticeResponse{Type: domain.PracticeResponseFindTheNote, String: &str, Fret: &fret, LatencyMs: &latency},
			},
			want: `{` + wireBaseJSON + `,"event_type":"practice.item_answered","practice_session_id":"44444444-4444-4444-8444-444444444444","item_key":"fretboard_cell:6ea2d087-ab9c-59dc-9657-8546025414d2:6:0","response":{"response_type":"find_the_note","string":6,"fret":0,"latency_ms":1800}}`,
		},
		{
			name: "challenge answer with audio",
			event: domain.PracticeItemAnsweredEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypePracticeItemAnswered),
				TriggerContext: &domain.TriggerContext{
					Source:        domain.TriggerSourceChallengeSequence,
					ChallengeID:   "77777777-7777-4777-8777-777777777777",
					ContentNodeID: "88888888-8888-4888-8888-888888888888",
				},
				ItemKey: "exercise:55555555-5555-4555-8555-555555555555",
				Response: domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice,
					OptionIDs: []string{"66666666-6666-4666-8666-666666666666"}, LatencyMs: &latency, AudioMs: &audio},
			},
			want: `{` + wireBaseJSON + `,"event_type":"practice.item_answered","trigger_context":{"source":"challenge_sequence","content_node_id":"88888888-8888-4888-8888-888888888888","challenge_id":"77777777-7777-4777-8777-777777777777"},"item_key":"exercise:55555555-5555-4555-8555-555555555555","response":{"response_type":"option_choice","option_ids":["66666666-6666-4666-8666-666666666666"],"latency_ms":1800,"audio_ms":5000}}`,
		},
		{
			name: "self-rated take",
			event: domain.PracticeItemAnsweredEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypePracticeItemAnswered),
				PracticeSessionID: "44444444-4444-4444-8444-444444444444",
				ItemKey:           "play_along:55555555-5555-4555-8555-555555555555",
				Response:          domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: domain.SelfRatingClean, TempoBPM: &tempo},
			},
			want: `{` + wireBaseJSON + `,"event_type":"practice.item_answered","practice_session_id":"44444444-4444-4444-8444-444444444444","item_key":"play_along:55555555-5555-4555-8555-555555555555","response":{"response_type":"self_rating","rating":"clean","tempo_bpm":90}}`,
		},
		{
			name: "session ended early with no felt ratings",
			event: domain.PracticeSessionEndedEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypePracticeSessionEnded),
				PracticeSessionID: "44444444-4444-4444-8444-444444444444",
				AnsweredCount:     0,
				LeftEarly:         true,
				FeltRatings:       []domain.FeltRating{},
			},
			want: `{` + wireBaseJSON + `,"event_type":"practice.session_ended","practice_session_id":"44444444-4444-4444-8444-444444444444","answered_count":0,"left_early":true,"felt_ratings":[]}`,
		},
		{
			name: "tap check",
			event: domain.PracticeTapCheckCompletedEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypePracticeTapCheckCompleted),
				MedianTapMs:       350,
				TapCount:          24,
			},
			want: `{` + wireBaseJSON + `,"event_type":"practice.tap_check_completed","median_tap_ms":350,"tap_count":24}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.JSONEq(t, tc.want, wireJSON(t, tc.event))
		})
	}
}

func TestToWireEvent_SongChartEvents(t *testing.T) {
	chart := domain.SongChartContext{SongChartID: "44444444-4444-4444-8444-444444444444", RevisionNumber: 2}
	const chartJSON = `"song_chart_context":{"song_chart_id":"44444444-4444-4444-8444-444444444444","revision_number":2}`

	cases := []struct {
		name  string
		event domain.TrackingEvent
		want  string
	}{
		{
			name:  "opened",
			event: domain.SongChartOpenedEvent{TrackingEventBase: practiceWireBase(domain.EventTypeSongChartOpened), SongChartContext: chart},
			want:  `{` + wireBaseJSON + `,"event_type":"song_chart.opened",` + chartJSON + `}`,
		},
		{
			name: "chord viewed",
			event: domain.SongChartChordViewedEvent{
				TrackingEventBase: practiceWireBase(domain.EventTypeSongChartChordViewed), SongChartContext: chart,
				AnchorID: "a3", ChordDefinitionID: "55555555-5555-4555-8555-555555555555", ChordVoicingID: "66666666-6666-4666-8666-666666666666",
			},
			want: `{` + wireBaseJSON + `,"event_type":"song_chart.chord_viewed",` + chartJSON + `,"anchor_id":"a3","chord_definition_id":"55555555-5555-4555-8555-555555555555","chord_voicing_id":"66666666-6666-4666-8666-666666666666"}`,
		},
		{
			name:  "section completed on the first section keeps its index",
			event: domain.SongChartSectionCompletedEvent{TrackingEventBase: practiceWireBase(domain.EventTypeSongChartSectionCompleted), SongChartContext: chart, SectionIndex: 0},
			want:  `{` + wireBaseJSON + `,"event_type":"song_chart.section_completed",` + chartJSON + `,"section_index":0}`,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert.JSONEq(t, tt.want, wireJSON(t, tt.event))
		})
	}
}
