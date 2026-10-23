package domain

import (
	"fmt"
	"slices"
)

// PracticeItemKindSong is a song: a published song chart played whole on the
// instrument in hand, as written, and rated once.
const PracticeItemKindSong PracticeItemKind = "song"

const (
	// MaxSongsPerSession caps the songs one session plays: a song is only
	// ever its ending.
	MaxSongsPerSession = 1
	// songPlaySeconds is the time a song is estimated to take to play, and
	// songRateSeconds the time to rate it after.
	songPlaySeconds = 120
	songRateSeconds = 10
	// songWay is the one way a song is answered: rated by the student.
	songWay = "self_rating"
)

// SongPlays is what the student plays from a song's chart.
type SongPlays string

const (
	// SongPlaysChords plays the chords as written, on guitar.
	SongPlaysChords SongPlays = "chords"
	// SongPlaysRoots plays the root of each chord, on bass.
	SongPlaysRoots SongPlays = "roots"
)

// PlannedSong is a song as a session plays it: the chart to load as learners
// read it, and why it was picked.
type PlannedSong struct {
	SongChartID        string
	LayoutInstrumentID string
	Title              string
	Artist             string
	// ProgressionKey is the progression the chart plays that practises the
	// session's subject.
	ProgressionKey string
	Plays          SongPlays
}

// SongItem is a published song chart on a fretboard layout, as a drill item.
// On a bass layout (Bass) the student plays the root of each chord.
type SongItem struct {
	LayoutInstrumentID string
	Bass               bool
	ChartID            string
	Title              string
	Artist             string
	// Progressions are the progressions the chart's latest published revision
	// is tagged with, in tagged order.
	Progressions []Progression
	// subject is the node the session practises through the song; empty
	// until ForSubject sets it.
	subject string
}

func (i SongItem) ItemKey() string { return SongItemKey(i.LayoutInstrumentID, i.ChartID) }

// SongItemKey is the item key of the song of songChartID on the layout
// layoutInstrumentID.
func SongItemKey(layoutInstrumentID, songChartID string) string {
	return fmt.Sprintf("%s:%s:%s", PracticeItemKindSong, layoutInstrumentID, songChartID)
}

func (SongItem) Kind() PracticeItemKind { return PracticeItemKindSong }
func (SongItem) Ways() []string         { return []string{songWay} }

// Seconds is the time to play the song, then rate it.
func (SongItem) Seconds() int { return songPlaySeconds + songRateSeconds }

// SuitsSession holds in hand only: a song is played.
func (SongItem) SuitsSession(inHand bool) bool { return inHand }

// NodeIDs are the nodes of its progressions, each once, in order.
func (i SongItem) NodeIDs() []string {
	var nodes []string
	for _, p := range i.Progressions {
		for _, n := range p.NodeIDs {
			if !slices.Contains(nodes, n) {
				nodes = append(nodes, n)
			}
		}
	}
	return nodes
}

// Serves reports whether one of its progressions practises nodeID.
func (i SongItem) Serves(nodeID string) bool {
	return slices.Contains(i.NodeIDs(), nodeID)
}

// ForSubject is the song as a session on nodeID plays it.
func (i SongItem) ForSubject(nodeID string) SongItem {
	i.subject = nodeID
	return i
}

// progressionKey is the first progression, in tagged order, that practises
// the subject; with none, the first.
func (i SongItem) progressionKey() string {
	for _, p := range i.Progressions {
		if slices.Contains(p.NodeIDs, i.subject) {
			return p.Key
		}
	}
	if len(i.Progressions) == 0 {
		return ""
	}
	return i.Progressions[0].Key
}

// Plan plays the chart as written: its chords on guitar, their roots on bass.
func (i SongItem) Plan(_ string, _ DrillPlanning) PlannedDrill {
	plays := SongPlaysChords
	if i.Bass {
		plays = SongPlaysRoots
	}
	return plannedSong{planned: &PlannedSong{
		SongChartID: i.ChartID, LayoutInstrumentID: i.LayoutInstrumentID, Title: i.Title, Artist: i.Artist,
		ProgressionKey: i.progressionKey(), Plays: plays,
	}}
}

type plannedSong struct {
	planned *PlannedSong
}

func (plannedSong) Way() string             { return songWay }
func (plannedSong) Stimulus() DrillStimulus { return DrillStimulus{} }
func (p plannedSong) AttachTo(item *PracticeSessionItem) {
	item.Kind = PracticeItemKindSong
	item.Song = p.planned
}

// EndsSessionOnly reports whether items of kind are only ever a session's
// application ending: never its warm-up, a focus item, a review or a stretch.
func EndsSessionOnly(kind PracticeItemKind) bool {
	return kind == PracticeItemKindSong
}
