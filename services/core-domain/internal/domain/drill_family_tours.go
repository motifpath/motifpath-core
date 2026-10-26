package domain

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// PracticeItemKindProgressionKeys is a progression key tour: a progression of
// the progression catalog played along with, key after key around the cycle of
// fourths, and rated take by take.
const PracticeItemKindProgressionKeys PracticeItemKind = "progression_keys"

const (
	// ProgressionKeysTargetTempoBPM is what every tour aims for, in quarters.
	ProgressionKeysTargetTempoBPM = 100
	// MaxProgressionKeysPerSession caps the tours one session plays, and
	// tourTakes the takes a session gives one.
	MaxProgressionKeysPerSession = 1
	tourTakes                    = 3
	// tourFirstTempoShare starts a tour never played at this share of the
	// target, rounded down to a tourTempoStepBPM step; tourMinTempoShare is
	// the slowest a struggle lowers it to.
	tourFirstTempoShare = 0.6
	tourMinTempoShare   = 0.6
	tourTempoStepBPM    = 5
	// fourBarTimesThrough is how often a take plays a four-bar progression; a
	// longer form plays once.
	fourBarTimesThrough = 4
	// tourBeatsPerBar is the tour's time signature, 4/4.
	tourBeatsPerBar = 4
	// tourWay is the one way a tour is answered: rated by the student.
	tourWay = "self_rating"
)

// ProgressionKeyOrder is the tour's order, the cycle of fourths from C, spelled
// as the chord catalog spells roots.
var ProgressionKeyOrder = []string{"C", "F", "Bb", "Eb", "Ab", "Db", "F#", "B", "E", "A", "D", "G"}

// KeyTourState is a tour as the student left it: its tempo at effective speed,
// its keys clean at that tempo, and the note value of its last clean take (nil
// before any, which plays in quarters).
type KeyTourState struct {
	TempoBPM  int
	CleanKeys []string
	NoteValue *TempoNoteValue
}

// PlannedProgressionKeys is a tour as a session plays it: every key with its
// chart, and where the tour starts.
type PlannedProgressionKeys struct {
	LayoutInstrumentID string
	ProgressionKey     string
	ProgressionNames   LocalizedText
	TimesThrough       int
	Takes              int
	StartMusicKey      string
	// TourTempoBPM is in StartNoteValue: playing it is at least as fast as the
	// tour's tempo.
	TourTempoBPM   int
	StartNoteValue TempoNoteValue
	TargetTempoBPM int
	MinTempoBPM    int
	Keys           []PlannedTourKey
}

// PlannedTourKey is one key of a tour: whether it is clean at the tour tempo,
// the progression's chart in it, the voicing of each of its chords, and on bass
// each chord's root, third and fifth.
type PlannedTourKey struct {
	MusicKey   string
	Clean      bool
	Chart      []string
	Voicings   []ChordBox
	ChordTones []ChordTonePlaces
}

// ChordTonePlaces is where a chord's root, third and fifth are played on a
// bass layout.
type ChordTonePlaces struct {
	Chord              string
	Root, Third, Fifth Place
}

// ProgressionKeysItem is a progression toured key after key on a layout, as a
// drill item. On a bass layout (Bass) each chord's tones are marked on the
// board.
type ProgressionKeysItem struct {
	Layout      Instrument
	Bass        bool
	Progression Progression
}

func (i ProgressionKeysItem) ItemKey() string {
	return ProgressionKeysItemKey(i.Layout.ID, i.Progression.Key)
}

// ProgressionKeysItemKey is the item key of progressionKey's tour on the
// layout layoutInstrumentID.
func ProgressionKeysItemKey(layoutInstrumentID, progressionKey string) string {
	return fmt.Sprintf("%s:%s:%s", PracticeItemKindProgressionKeys, layoutInstrumentID, progressionKey)
}

func (ProgressionKeysItem) Kind() PracticeItemKind { return PracticeItemKindProgressionKeys }
func (ProgressionKeysItem) Ways() []string         { return []string{tourWay} }

// Seconds sizes the tour like a play-along: each take a bar of count-in and
// the progression's times through at the first tour's tempo, then rating it.
// A faster tour takes less, so the estimate never runs short.
func (i ProgressionKeysItem) Seconds() int {
	beats := (len(i.Progression.Bars)*i.timesThrough() + 1) * tourBeatsPerBar
	take := beats*60/tourFirstTempo() + rateSeconds
	return tourTakes * take
}

// SuitsSession holds in hand only: a tour is played.
func (ProgressionKeysItem) SuitsSession(inHand bool) bool { return inHand }

func (i ProgressionKeysItem) timesThrough() int {
	if len(i.Progression.Bars) == HeardProgressionBars {
		return fourBarTimesThrough
	}
	return 1
}

// Plan lays the tour out in every key, starting where planning.State left it:
// at the first key of the cycle not yet clean, at the tour's tempo, in the note
// value of the last clean take.
func (i ProgressionKeysItem) Plan(_ string, planning DrillPlanning) PlannedDrill {
	tour := KeyTourState{TempoBPM: tourFirstTempo()}
	if planning.State != nil && planning.State.KeyTour != nil {
		tour = *planning.State.KeyTour
	}
	noteValue := tempoNoteValueOr(tour.NoteValue)
	planned := &PlannedProgressionKeys{
		LayoutInstrumentID: i.Layout.ID,
		ProgressionKey:     i.Progression.Key,
		ProgressionNames:   i.Progression.Names,
		TimesThrough:       i.timesThrough(),
		Takes:              tourTakes,
		// Rounded up, so a take played at it is never slower than the tour.
		TourTempoBPM:   int(math.Ceil(float64(tour.TempoBPM) / noteValue.factor())),
		StartNoteValue: noteValue,
		TargetTempoBPM: ProgressionKeysTargetTempoBPM,
		MinTempoBPM:    tourMinTempo(),
	}
	for _, key := range ProgressionKeyOrder {
		clean := slices.Contains(tour.CleanKeys, key)
		if !clean && planned.StartMusicKey == "" {
			planned.StartMusicKey = key
		}
		planned.Keys = append(planned.Keys, i.planKey(key, clean, planning))
	}
	if planned.StartMusicKey == "" {
		planned.StartMusicKey = ProgressionKeyOrder[0]
	}
	return plannedTour{planned: planned}
}

// planKey draws the progression in key: its chart, a voicing for each chord
// the backing sounds, and on bass where each chord's root, third and fifth
// are. A chord without a voicing or a bass place is left out of that list.
func (i ProgressionKeysItem) planKey(key string, clean bool, planning DrillPlanning) PlannedTourKey {
	k := PlannedTourKey{MusicKey: key, Clean: clean, Chart: ProgressionChart(i.Progression.Bars, key)}
	for _, chord := range distinct(k.Chart) {
		if planning.Voicing != nil {
			if id, ok := planning.Voicing(chord); ok {
				k.Voicings = append(k.Voicings, ChordBox{Chord: chord, DiagramID: id})
			}
		}
		if i.Bass {
			if tones, err := chordTonePlaces(i.Layout, chord); err == nil {
				k.ChordTones = append(k.ChordTones, tones)
			}
		}
	}
	return k
}

// chordTonePlaces places chord's root, third and fifth on a bass layout: see
// bassPlace.
func chordTonePlaces(layout Instrument, chord string) (ChordTonePlaces, error) {
	root, err := chordRootPitchClass(chord)
	if err != nil {
		return ChordTonePlaces{}, err
	}
	third, fifth := chordThirdAndFifth(chord)
	places := make([]Place, 3)
	for n, interval := range []int{0, third, fifth} {
		if places[n], err = bassPlace(layout, (root+interval)%12); err != nil {
			return ChordTonePlaces{}, err
		}
	}
	return ChordTonePlaces{Chord: chord, Root: places[0], Third: places[1], Fifth: places[2]}, nil
}

// plannedTour is a tour as a session plays it: rated, never asked, so it
// marks no answer.
type plannedTour struct {
	planned *PlannedProgressionKeys
}

func (plannedTour) Way() string             { return tourWay }
func (plannedTour) Stimulus() DrillStimulus { return DrillStimulus{} }
func (p plannedTour) AttachTo(item *PracticeSessionItem) {
	item.Kind = PracticeItemKindProgressionKeys
	item.ProgressionKeys = p.planned
}

// tourFirstTempo is a tour never played's tempo: tourFirstTempoShare of the
// target, rounded down to a step.
func tourFirstTempo() int {
	return int(math.Floor(ProgressionKeysTargetTempoBPM*tourFirstTempoShare/tourTempoStepBPM)) * tourTempoStepBPM
}

// tourMinTempo is the slowest a struggle lowers a tour to.
func tourMinTempo() int {
	return int(math.Ceil(ProgressionKeysTargetTempoBPM * tourMinTempoShare))
}

// chordThirdAndFifth are the semitones from chord's root to its third and its
// fifth: minor and diminished chords have a minor third, a diminished one a
// flat fifth, and a seventh keeps its triad's.
func chordThirdAndFifth(chord string) (third, fifth int) {
	switch {
	case strings.HasSuffix(chord, "dim"):
		return 3, 6
	case strings.HasSuffix(chord, "m"):
		return 3, 7
	}
	return 4, 7
}

// distinct is chords, each once, in the order they first appear.
func distinct(chords []string) []string {
	var out []string
	for _, c := range chords {
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}
