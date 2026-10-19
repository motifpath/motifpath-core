package domain

import "time"

// EventTypeSongChartCompleted is sent when a student marks a song chart as played
// in its reader.
const EventTypeSongChartCompleted EventType = "song_chart.completed"

// SongChartCompletion is one time a student marked a song chart as played, kept
// raw with when it happened: marking the same chart again is another completion.
// How many songs a student has played, and since when, is evaluated from these
// when read.
type SongChartCompletion struct {
	// EventID is the id of the song_chart.completed event, so an event counts
	// once.
	EventID     string
	StudentID   string
	SongChartID string
	// CompletedAt is when the student marked it, on their own clock.
	CompletedAt time.Time
}
