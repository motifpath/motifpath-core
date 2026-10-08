package http

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/event-ingestion/internal/domain"
)

const (
	testSongChartID = "6f1c1d1e-9999-4999-8999-999999999999"
	testChordID     = "6f1c1d1e-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testVoicingID   = "6f1c1d1e-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

var (
	testChartContext   = fmt.Sprintf(`,"song_chart_context":{"song_chart_id":%q,"revision_number":2}`, testSongChartID)
	testChartReference = domain.SongChartContext{SongChartID: testSongChartID, RevisionNumber: 2}
)

func TestToDomainEvent_SongChartEvents(t *testing.T) {
	cases := []struct {
		name      string
		eventType string
		fields    string
		want      domain.TrackingEvent
	}{
		{
			name:      "opened",
			eventType: "song_chart.opened",
			fields:    testChartContext,
			want:      domain.SongChartOpenedEvent{SongChartContext: testChartReference},
		},
		{
			name:      "chord viewed",
			eventType: "song_chart.chord_viewed",
			fields:    testChartContext + fmt.Sprintf(`,"anchor_id":"a3","chord_definition_id":%q,"chord_voicing_id":%q`, testChordID, testVoicingID),
			want: domain.SongChartChordViewedEvent{
				SongChartContext: testChartReference, AnchorID: "a3", ChordDefinitionID: testChordID, ChordVoicingID: testVoicingID,
			},
		},
		{
			name:      "completed",
			eventType: "song_chart.completed",
			fields:    testChartContext,
			want:      domain.SongChartCompletedEvent{SongChartContext: testChartReference},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			event, err := toDomainEvent(practiceBody(t, tt.eventType, tt.fields))

			require.NoError(t, err)
			assert.Equal(t, domain.EventType(tt.eventType), event.Base().EventType)
			assert.Equal(t, testEventID, event.Base().EventID)
			assert.Equal(t, tt.want, withoutBase(event))
		})
	}
}

// withoutBase clears the envelope, which every event maps the same way.
func withoutBase(event domain.TrackingEvent) domain.TrackingEvent {
	switch e := event.(type) {
	case domain.SongChartOpenedEvent:
		e.TrackingEventBase = domain.TrackingEventBase{}
		return e
	case domain.SongChartChordViewedEvent:
		e.TrackingEventBase = domain.TrackingEventBase{}
		return e
	case domain.SongChartCompletedEvent:
		e.TrackingEventBase = domain.TrackingEventBase{}
		return e
	}
	return event
}

func TestToDomainEvent_SongChartEventsRejected(t *testing.T) {
	viewed := func(omit string) string {
		fields := map[string]string{
			"anchor_id":           `"a3"`,
			"chord_definition_id": fmt.Sprintf("%q", testChordID),
			"chord_voicing_id":    fmt.Sprintf("%q", testVoicingID),
		}
		delete(fields, omit)
		out := testChartContext
		for _, key := range []string{"anchor_id", "chord_definition_id", "chord_voicing_id"} {
			if v, ok := fields[key]; ok {
				out += fmt.Sprintf(`,%q:%s`, key, v)
			}
		}
		return out
	}
	cases := []struct {
		name      string
		eventType string
		fields    string
		wantErr   error
		wantField string
	}{
		{name: "opened without a song chart context", eventType: "song_chart.opened", fields: "", wantErr: domain.ErrMissingRequiredField, wantField: "song_chart_context"},
		{name: "a context without the chart", eventType: "song_chart.opened", fields: `,"song_chart_context":{"revision_number":2}`, wantErr: domain.ErrMissingRequiredField, wantField: "song_chart_context.song_chart_id"},
		{name: "a context at revision 0", eventType: "song_chart.opened", fields: fmt.Sprintf(`,"song_chart_context":{"song_chart_id":%q,"revision_number":0}`, testSongChartID), wantErr: domain.ErrInvalidField, wantField: "song_chart_context.revision_number"},
		{name: "chord viewed without the anchor", eventType: "song_chart.chord_viewed", fields: viewed("anchor_id"), wantErr: domain.ErrMissingRequiredField, wantField: "anchor_id"},
		{name: "chord viewed with an anchor id too long", eventType: "song_chart.chord_viewed", fields: testChartContext + fmt.Sprintf(`,"anchor_id":%q,"chord_definition_id":%q,"chord_voicing_id":%q`, strings.Repeat("a", 65), testChordID, testVoicingID), wantErr: domain.ErrInvalidField, wantField: "anchor_id"},
		{name: "chord viewed without the chord", eventType: "song_chart.chord_viewed", fields: viewed("chord_definition_id"), wantErr: domain.ErrMissingRequiredField, wantField: "chord_definition_id"},
		{name: "chord viewed without the voicing", eventType: "song_chart.chord_viewed", fields: viewed("chord_voicing_id"), wantErr: domain.ErrMissingRequiredField, wantField: "chord_voicing_id"},
		{name: "completed without a song chart context", eventType: "song_chart.completed", fields: "", wantErr: domain.ErrMissingRequiredField, wantField: "song_chart_context"},
		{name: "a section_completed event, no longer an event type", eventType: "song_chart.section_completed", fields: testChartContext + `,"section_index":0`, wantErr: domain.ErrInvalidEventType, wantField: "song_chart.section_completed"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := toDomainEvent(practiceBody(t, tt.eventType, tt.fields))

			require.Error(t, err)
			assert.True(t, errors.Is(err, tt.wantErr), "got %v", err)
			assert.Contains(t, err.Error(), tt.wantField)
		})
	}
}
