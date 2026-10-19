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

// ChordViewSource is where in a song chart a student tapped the chord they viewed.
type ChordViewSource string

const (
	// ChordViewSourceLyrics is a chord over the lyrics: one anchor was tapped.
	ChordViewSourceLyrics ChordViewSource = "lyrics"
	// ChordViewSourceStrip is a chord in the song's chord strip, which stands
	// for every anchor of that chord, so no single anchor was tapped.
	ChordViewSourceStrip ChordViewSource = "strip"
)

// SongChartChordViewedEvent is emitted when a student taps a chord in a song
// chart and its chord card opens: where the tap was, the anchor tapped (only
// from the lyrics), the catalog chord it resolves to and the voicing the card
// opened on.
type SongChartChordViewedEvent struct {
	TrackingEventBase
	SongChartContext SongChartContext
	Source           ChordViewSource
	// AnchorID is empty when Source is ChordViewSourceStrip.
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
