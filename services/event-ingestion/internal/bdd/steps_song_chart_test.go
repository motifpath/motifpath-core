//go:build integration

package bdd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
)

// songChartEvent is a song_chart.* event as the reader sends it. Omitted
// fields are left out of the JSON, so a step can send an event without them.
type songChartEvent struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	StudentID  string `json:"student_id"`
	SessionID  string `json:"session_id"`
	OccurredAt string `json:"occurred_at"`

	SongChartContext  *songChartContext `json:"song_chart_context,omitempty"`
	AnchorID          string            `json:"anchor_id,omitempty"`
	ChordDefinitionID string            `json:"chord_definition_id,omitempty"`
	ChordVoicingID    string            `json:"chord_voicing_id,omitempty"`
}

type songChartContext struct {
	SongChartID    string `json:"song_chart_id"`
	RevisionNumber int    `json:"revision_number"`
}

func registerSongChartSteps(sc *godog.ScenarioContext, w *world) {
	// Given
	sc.Step(`^"([^"]+)" has already submitted a song_chart\.opened event with identifier "([^"]+)"$`, w.hasSubmittedChartOpened)

	// When
	sc.Step(`^"([^"]+)" submits a song_chart\.opened event for revision (\d+) of song chart "([^"]+)"$`, func(name string, revision int, chart string) error {
		e := newSongChartEvent(name, "song_chart.opened", chart, revision)
		return w.submitSongChart(e)
	})
	sc.Step(`^"([^"]+)" submits a song_chart\.chord_viewed event for anchor "([^"]+)" of revision (\d+) of song chart "([^"]+)", resolving to chord "([^"]+)" and opening on voicing "([^"]+)"$`,
		func(name, anchor string, revision int, chart, chord, voicing string) error {
			e := newSongChartEvent(name, "song_chart.chord_viewed", chart, revision)
			e.AnchorID = anchor
			e.ChordDefinitionID = deterministicUUID("chord", chord).String()
			e.ChordVoicingID = deterministicUUID("voicing", voicing).String()
			return w.submitSongChart(e)
		})
	sc.Step(`^"([^"]+)" submits a song_chart\.completed event for revision (\d+) of song chart "([^"]+)"$`, func(name string, revision int, chart string) error {
		return w.submitSongChart(newSongChartEvent(name, "song_chart.completed", chart, revision))
	})
	sc.Step(`^"([^"]+)" submits a song_chart\.completed event with the song chart context omitted$`, func(name string) error {
		e := newSongChartEvent(name, "song_chart.completed", "asa-branca", 2)
		e.SongChartContext = nil
		return w.submitSongChart(e)
	})
	sc.Step(`^"([^"]+)" submits the same song_chart\.opened event again with identifier "([^"]+)"$`, func(string, string) error {
		if w.lastSubmittedBody == nil {
			return fmt.Errorf("no previously submitted event to resubmit")
		}
		w.submit(w.lastSubmittedBody)
		return nil
	})
	sc.Step(`^"([^"]+)" submits a song_chart\.opened event with the song chart context omitted$`, func(name string) error {
		e := newSongChartEvent(name, "song_chart.opened", "asa-branca", 2)
		e.SongChartContext = nil
		return w.submitSongChart(e)
	})
	sc.Step(`^"([^"]+)" submits a song_chart\.chord_viewed event with the voicing omitted$`, func(name string) error {
		e := newSongChartEvent(name, "song_chart.chord_viewed", "asa-branca", 2)
		e.AnchorID = "a3"
		e.ChordDefinitionID = deterministicUUID("chord", "G").String()
		return w.submitSongChart(e)
	})
}

func newSongChartEvent(name, eventType, chart string, revision int) songChartEvent {
	return songChartEvent{
		EventID:          deterministicUUID("event", name, eventType, chart).String(),
		EventType:        eventType,
		StudentID:        studentID(name),
		SessionID:        fixedSessionID.String(),
		OccurredAt:       fixedOccurredAt.Format(time.RFC3339),
		SongChartContext: &songChartContext{SongChartID: deterministicUUID("song-chart", chart).String(), RevisionNumber: revision},
	}
}

func (w *world) submitSongChart(e songChartEvent) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	body := &generated.TrackingEvent{}
	if err := body.UnmarshalJSON(raw); err != nil {
		return err
	}
	w.submit(body)
	return nil
}

func (w *world) hasSubmittedChartOpened(name, identifier string) error {
	e := newSongChartEvent(name, "song_chart.opened", "asa-branca", 2)
	e.EventID = deterministicUUID("event", identifier).String()
	if err := w.submitSongChart(e); err != nil {
		return err
	}
	if _, ok := w.ingestResp.(generated.IngestTrackingEvent202JSONResponse); !ok {
		return fmt.Errorf("setup: expected the first submission to be accepted, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	return nil
}
