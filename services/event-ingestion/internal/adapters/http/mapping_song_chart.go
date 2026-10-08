package http

import (
	"fmt"
	"unicode/utf8"

	"github.com/motifpath/event-ingestion/internal/adapters/http/generated"
	"github.com/motifpath/event-ingestion/internal/domain"
)

// toSongChartEvent converts any song_chart.* event; the caller has checked
// that eventType is one of them.
func toSongChartEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	if eventType == domain.EventTypeSongChartChordViewed {
		return toSongChartChordViewedEvent(eventType, body)
	}
	if eventType == domain.EventTypeSongChartCompleted {
		return toSongChartCompletedEvent(eventType, body)
	}
	return toSongChartOpenedEvent(eventType, body)
}

func toSongChartOpenedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsSongChartOpenedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	chart, err := toDomainSongChartContext(body, v.SongChartContext)
	if err != nil {
		return nil, err
	}
	return domain.SongChartOpenedEvent{TrackingEventBase: base, SongChartContext: chart}, nil
}

func toSongChartChordViewedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsSongChartChordViewedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	chart, err := toDomainSongChartContext(body, v.SongChartContext)
	if err != nil {
		return nil, err
	}
	if err := requirePresent(body, "anchor_id"); err != nil {
		return nil, err
	}
	if n := utf8.RuneCountInString(v.AnchorId); n == 0 || n > domain.MaxSongChartAnchorIDLength {
		return nil, fmt.Errorf("%w: anchor_id", domain.ErrInvalidField)
	}
	chordID, err := requireUUID(v.ChordDefinitionId, "chord_definition_id")
	if err != nil {
		return nil, err
	}
	voicingID, err := requireUUID(v.ChordVoicingId, "chord_voicing_id")
	if err != nil {
		return nil, err
	}
	return domain.SongChartChordViewedEvent{
		TrackingEventBase: base,
		SongChartContext:  chart,
		AnchorID:          v.AnchorId,
		ChordDefinitionID: chordID,
		ChordVoicingID:    voicingID,
	}, nil
}

func toSongChartCompletedEvent(eventType domain.EventType, body *generated.TrackingEvent) (domain.TrackingEvent, error) {
	v, err := body.AsSongChartCompletedEvent()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, err)
	}
	base, err := toDomainBase(eventType, v.EventId, v.StudentId, v.SessionId, v.OccurredAt)
	if err != nil {
		return nil, err
	}
	chart, err := toDomainSongChartContext(body, v.SongChartContext)
	if err != nil {
		return nil, err
	}
	return domain.SongChartCompletedEvent{TrackingEventBase: base, SongChartContext: chart}, nil
}

// toDomainSongChartContext reads the context every song_chart event requires.
// An absent context decodes to its zero value, so its presence is checked on
// the raw body.
func toDomainSongChartContext(body *generated.TrackingEvent, c generated.SongChartContext) (domain.SongChartContext, error) {
	if err := requirePresent(body, "song_chart_context"); err != nil {
		return domain.SongChartContext{}, err
	}
	chartID, err := requireUUID(c.SongChartId, "song_chart_context.song_chart_id")
	if err != nil {
		return domain.SongChartContext{}, err
	}
	if c.RevisionNumber < 1 {
		return domain.SongChartContext{}, fmt.Errorf("%w: song_chart_context.revision_number", domain.ErrInvalidField)
	}
	return domain.SongChartContext{SongChartID: chartID, RevisionNumber: c.RevisionNumber}, nil
}
