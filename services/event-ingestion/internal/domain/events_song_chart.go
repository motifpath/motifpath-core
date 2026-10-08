package domain

// MaxSongChartAnchorIDLength is the longest anchor id a song chart holds.
const MaxSongChartAnchorIDLength = 64

// SongChartContext is the published song chart revision a song_chart event
// happened in, so a reading is tied to the exact lyrics and chords the
// student saw. Required on every song_chart event.
type SongChartContext struct {
	SongChartID    string
	RevisionNumber int
}

// SongChartOpenedEvent is emitted each time a student opens a published song
// chart in the reader.
type SongChartOpenedEvent struct {
	TrackingEventBase
	SongChartContext SongChartContext
}

func (e SongChartOpenedEvent) Base() TrackingEventBase { return e.TrackingEventBase }

// SongChartChordViewedEvent is emitted when a student taps a chord in a song
// chart and its voicing sheet opens: the anchor tapped, the catalog chord it
// resolves to and the voicing the sheet opened on.
type SongChartChordViewedEvent struct {
	TrackingEventBase
	SongChartContext  SongChartContext
	AnchorID          string
	ChordDefinitionID string
	ChordVoicingID    string
}

func (e SongChartChordViewedEvent) Base() TrackingEventBase { return e.TrackingEventBase }

// SongChartCompletedEvent is emitted when a student marks a song chart as
// played, once per opening.
type SongChartCompletedEvent struct {
	TrackingEventBase
	SongChartContext SongChartContext
}

func (e SongChartCompletedEvent) Base() TrackingEventBase { return e.TrackingEventBase }
