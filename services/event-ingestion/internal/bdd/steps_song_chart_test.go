//go:build integration

package bdd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
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
	Source            string            `json:"source,omitempty"`
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
	sc.Step(`^"([^"]+)" submits a song_chart\.chord_viewed event from the lyrics for anchor "([^"]+)" of revision (\d+) of song chart "([^"]+)", resolving to chord "([^"]+)" and opening on voicing "([^"]+)"$`,
		func(name, anchor string, revision int, chart, chord, voicing string) error {
			e := newChordViewedEvent(name, "lyrics", chart, revision, chord, voicing)
			e.AnchorID = anchor
			return w.submitSongChart(e)
		})
	sc.Step(`^"([^"]+)" submits a song_chart\.chord_viewed event from the strip of revision (\d+) of song chart "([^"]+)", resolving to chord "([^"]+)" and opening on voicing "([^"]+)"$`,
		func(name string, revision int, chart, chord, voicing string) error {
			return w.submitSongChart(newChordViewedEvent(name, "strip", chart, revision, chord, voicing))
		})
	sc.Step(`^"([^"]+)" submits a song_chart\.chord_viewed event with the source omitted$`, func(name string) error {
		e := newChordViewedEvent(name, "", "amazing-grace", 2, "G", "g-open")
		e.AnchorID = "a3"
		return w.submitSongChart(e)
	})
	sc.Step(`^"([^"]+)" submits a song_chart\.chord_viewed event with the source "([^"]+)"$`, func(name, source string) error {
		return w.submitSongChart(newChordViewedEvent(name, source, "amazing-grace", 2, "G", "g-open"))
	})
	sc.Step(`^"([^"]+)" submits a song_chart\.chord_viewed event from the lyrics with the anchor omitted$`, func(name string) error {
		return w.submitSongChart(newChordViewedEvent(name, "lyrics", "amazing-grace", 2, "G", "g-open"))
	})
	sc.Step(`^"([^"]+)" submits a song_chart\.completed event for revision (\d+) of song chart "([^"]+)"$`, func(name string, revision int, chart string) error {
		return w.submitSongChart(newSongChartEvent(name, "song_chart.completed", chart, revision))
	})
	sc.Step(`^"([^"]+)" submits a song_chart\.completed event with the song chart context omitted$`, func(name string) error {
		e := newSongChartEvent(name, "song_chart.completed", "amazing-grace", 2)
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
		e := newSongChartEvent(name, "song_chart.opened", "amazing-grace", 2)
		e.SongChartContext = nil
		return w.submitSongChart(e)
	})
	sc.Step(`^"([^"]+)" submits a song_chart\.chord_viewed event with the voicing omitted$`, func(name string) error {
		e := newChordViewedEvent(name, "lyrics", "amazing-grace", 2, "G", "g-open")
		e.AnchorID = "a3"
		e.ChordVoicingID = ""
		return w.submitSongChart(e)
	})

	// Then
	sc.Step(`^the stored event has the source "([^"]+)" and no anchor$`, w.storedChordViewHasSourceAndNoAnchor)
}

// newChordViewedEvent is a song_chart.chord_viewed event from source, without
// an anchor; an empty source leaves it out.
func newChordViewedEvent(name, source, chart string, revision int, chord, voicing string) songChartEvent {
	e := newSongChartEvent(name, "song_chart.chord_viewed", chart, revision)
	e.Source = source
	e.ChordDefinitionID = deterministicUUID("chord", chord).String()
	e.ChordVoicingID = deterministicUUID("voicing", voicing).String()
	return e
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
	e := newSongChartEvent(name, "song_chart.opened", "amazing-grace", 2)
	e.EventID = deterministicUUID("event", identifier).String()
	if err := w.submitSongChart(e); err != nil {
		return err
	}
	if _, ok := w.ingestResp.(generated.IngestTrackingEvent202JSONResponse); !ok {
		return fmt.Errorf("setup: expected the first submission to be accepted, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	return nil
}

func (w *world) storedChordViewHasSourceAndNoAnchor(source string) error {
	resp, ok := w.ingestResp.(generated.IngestTrackingEvent202JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 202 response, got %#v (err=%v)", w.ingestResp, w.ingestErr)
	}
	w.repo.mu.Lock()
	saved, found := w.repo.saved[resp.EventId.String()]
	w.repo.mu.Unlock()
	if !found {
		return fmt.Errorf("event %s was not stored", resp.EventId)
	}
	viewed, ok := saved.(domain.SongChartChordViewedEvent)
	if !ok {
		return fmt.Errorf("stored event is a %T", saved)
	}
	if string(viewed.Source) != source {
		return fmt.Errorf("stored source is %q, want %q", viewed.Source, source)
	}
	if viewed.AnchorID != "" {
		return fmt.Errorf("stored anchor is %q, want none", viewed.AnchorID)
	}
	return nil
}
