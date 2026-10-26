package domain

import (
	"fmt"
	"strings"
)

// The chord drills' item kinds: chords played with the instrument in hand,
// rated rather than timed.
const (
	PracticeItemKindChordVoicing PracticeItemKind = "chord_voicing"
	PracticeItemKindChordChange  PracticeItemKind = "chord_change"
	PracticeItemKindRootFifth    PracticeItemKind = "root_fifth"
	PracticeItemKindRootChange   PracticeItemKind = "root_change"
)

const (
	// ChordVoicingSeconds is how long a single chord takes: fret it, strum
	// once, let it ring, rate it.
	ChordVoicingSeconds = 20
	// ChordChangeSeconds is how long a minute of changes takes: a bar of
	// count-in, the minute and the count.
	ChordChangeSeconds = 90
	// MaxChordVoicingsPerSession and MaxChordChangesPerSession cap the
	// single chords and the changes one session plays.
	MaxChordVoicingsPerSession = 4
	MaxChordChangesPerSession  = 2

	// The change ladder, in BPM, one change on every beat: a change never
	// played starts at changeFirstTempoBPM; two clean minutes in a row raise
	// the next by changeTempoStepBPM and a struggled one lowers it as much,
	// never below changeMinTempoBPM.
	changeFirstTempoBPM = 40
	changeTempoStepBPM  = 5
	changeMinTempoBPM   = 20
	// changeCleanStreak is the clean minutes in a row that raise the tempo.
	changeCleanStreak = 2

	// chordWay is the one way a chord item is answered: rated by the student
	// (a change's rating follows from its count).
	chordWay = "self_rating"
)

// chordSessionCaps are the most single chords, changes and progression key
// tours a session plays.
var chordSessionCaps = map[PracticeItemKind]int{
	PracticeItemKindChordVoicing:    MaxChordVoicingsPerSession,
	PracticeItemKindRootFifth:       MaxChordVoicingsPerSession,
	PracticeItemKindChordChange:     MaxChordChangesPerSession,
	PracticeItemKindRootChange:      MaxChordChangesPerSession,
	PracticeItemKindProgressionKeys: MaxProgressionKeysPerSession,
}

// selfRatedChordKinds are the kinds played and rated rather than timed: the
// chord drills and the progression key tour, with their rungs.
var selfRatedChordKinds = map[PracticeItemKind]PracticeRung{
	PracticeItemKindChordVoicing:    PracticeRungPlayClean,
	PracticeItemKindRootFifth:       PracticeRungPlayClean,
	PracticeItemKindChordChange:     PracticeRungPlayInTime,
	PracticeItemKindRootChange:      PracticeRungPlayInTime,
	PracticeItemKindProgressionKeys: PracticeRungMove,
}

// PlannedChordVoicing is a single chord as a session plays it: its voicing's
// chord box, named.
type PlannedChordVoicing struct {
	DiagramID   string
	ChordSymbol string
}

// PlannedRootFifth is a chord's root and fifth as a session plays them on a
// bass layout, both marked.
type PlannedRootFifth struct {
	LayoutInstrumentID string
	ChordSymbol        string
	Root, Fifth        Place
}

// PlannedChangeMinute is a minute of changes as a session plays it: where its
// tempo ladder starts, the target, and the student's best clean and last
// counts.
type PlannedChangeMinute struct {
	StartTempoBPM             int
	TargetChangesPerMinute    int
	BestCleanChangesPerMinute *int
	LastChangesPerMinute      *int
}

// PlannedChordChange is a minute of changes between two chords' boxes.
type PlannedChordChange struct {
	FromDiagramID, ToDiagramID     string
	FromChordSymbol, ToChordSymbol string
	Minute                         PlannedChangeMinute
}

// PlannedRootChange is a minute of changes between two chords' roots on a
// bass layout.
type PlannedRootChange struct {
	LayoutInstrumentID             string
	FromChordSymbol, ToChordSymbol string
	FromRoot, ToRoot               Place
	Minute                         PlannedChangeMinute
}

// planChangeMinute starts a change's minute on its ladder from state, the
// student's state on it (nil when never played): the last minute's tempo,
// raised after two clean minutes in a row, lowered after a struggled one,
// never below the floor.
func planChangeMinute(state *PracticeItemState, target int) PlannedChangeMinute {
	minute := PlannedChangeMinute{StartTempoBPM: changeFirstTempoBPM, TargetChangesPerMinute: target}
	if state == nil {
		return minute
	}
	minute.BestCleanChangesPerMinute = state.BestChangesPerMinute
	last := state.LastChangeMinute
	if last == nil {
		return minute
	}
	count := last.ChangesPerMinute
	minute.LastChangesPerMinute = &count
	tempo := last.TempoBPM
	switch {
	case last.Struggled:
		tempo -= changeTempoStepBPM
	case last.CleanInARow >= changeCleanStreak:
		tempo += changeTempoStepBPM
	}
	minute.StartTempoBPM = max(tempo, changeMinTempoBPM)
	return minute
}

// playedChord is a chord drill as played: nothing is asked, so nothing is
// marked as an answer. What it shows is all given.
func playedStimulus(places ...Place) DrillStimulus {
	s := DrillStimulus{}
	for _, p := range places {
		s.Places = append(s.Places, StimulusPlace{Place: p, Given: true})
	}
	return s
}

// ChordVoicingItem is a chord drill's voicing, fretted and strummed once, as a
// drill item.
type ChordVoicingItem struct {
	Drill ChordDrill
}

func (c ChordVoicingItem) ItemKey() string {
	return string(PracticeItemKindChordVoicing) + ":" + c.Drill.DiagramID
}
func (ChordVoicingItem) Kind() PracticeItemKind { return PracticeItemKindChordVoicing }
func (ChordVoicingItem) Ways() []string         { return []string{chordWay} }
func (ChordVoicingItem) Seconds() int           { return ChordVoicingSeconds }

// SuitsSession holds in hand only: a chord is played.
func (ChordVoicingItem) SuitsSession(inHand bool) bool { return inHand }

func (c ChordVoicingItem) Plan(string, DrillPlanning) PlannedDrill {
	return plannedChordItem{attach: func(item *PracticeSessionItem) {
		item.Kind = PracticeItemKindChordVoicing
		item.ChordVoicing = &PlannedChordVoicing{DiagramID: c.Drill.DiagramID, ChordSymbol: c.Drill.Chord}
	}}
}

// ChordChangeItem is a pair of chord drills, changed between for a minute, as
// a drill item. TargetChangesPerMinute is the change target in force.
type ChordChangeItem struct {
	From, To               ChordDrill
	TargetChangesPerMinute int
}

func (c ChordChangeItem) ItemKey() string {
	return fmt.Sprintf("%s:%s:%s", PracticeItemKindChordChange, c.From.DiagramID, c.To.DiagramID)
}
func (ChordChangeItem) Kind() PracticeItemKind { return PracticeItemKindChordChange }
func (ChordChangeItem) Ways() []string         { return []string{chordWay} }
func (ChordChangeItem) Seconds() int           { return ChordChangeSeconds }

// SuitsSession holds in hand only: a change is played.
func (ChordChangeItem) SuitsSession(inHand bool) bool { return inHand }

func (c ChordChangeItem) Plan(_ string, planning DrillPlanning) PlannedDrill {
	minute := planChangeMinute(planning.State, c.TargetChangesPerMinute)
	return plannedChordItem{attach: func(item *PracticeSessionItem) {
		item.Kind = PracticeItemKindChordChange
		item.ChordChange = &PlannedChordChange{
			FromDiagramID: c.From.DiagramID, ToDiagramID: c.To.DiagramID,
			FromChordSymbol: c.From.Chord, ToChordSymbol: c.To.Chord, Minute: minute,
		}
	}}
}

// UsesChord reports whether the change is to or from the voicing diagramID.
func (c ChordChangeItem) UsesChord(diagramID string) bool {
	return c.From.DiagramID == diagramID || c.To.DiagramID == diagramID
}

// RootFifthItem is a chord's root and fifth on a bass layout, as a drill
// item.
type RootFifthItem struct {
	LayoutInstrumentID string
	Chord              string
	Root, Fifth        Place
}

// NewRootFifthItem places chord's root and fifth on layout: see bassPlace.
func NewRootFifthItem(layout Instrument, chord string) (RootFifthItem, error) {
	root, err := chordRootPitchClass(chord)
	if err != nil {
		return RootFifthItem{}, err
	}
	rootPlace, err := bassPlace(layout, root)
	if err != nil {
		return RootFifthItem{}, err
	}
	fifthPlace, err := bassPlace(layout, (root+7)%12)
	if err != nil {
		return RootFifthItem{}, err
	}
	return RootFifthItem{LayoutInstrumentID: layout.ID, Chord: chord, Root: rootPlace, Fifth: fifthPlace}, nil
}

func (r RootFifthItem) ItemKey() string { return RootFifthItemKey(r.LayoutInstrumentID, r.Chord) }

// RootFifthItemKey is the item key of chord's root and fifth on the bass
// layout layoutInstrumentID.
func RootFifthItemKey(layoutInstrumentID, chord string) string {
	return fmt.Sprintf("%s:%s:%s", PracticeItemKindRootFifth, layoutInstrumentID, chord)
}
func (RootFifthItem) Kind() PracticeItemKind { return PracticeItemKindRootFifth }
func (RootFifthItem) Ways() []string         { return []string{chordWay} }
func (RootFifthItem) Seconds() int           { return ChordVoicingSeconds }

// SuitsSession holds in hand only: a root and fifth are played.
func (RootFifthItem) SuitsSession(inHand bool) bool { return inHand }

func (r RootFifthItem) Plan(string, DrillPlanning) PlannedDrill {
	return plannedChordItem{places: []Place{r.Root, r.Fifth}, attach: func(item *PracticeSessionItem) {
		item.Kind = PracticeItemKindRootFifth
		item.RootFifth = &PlannedRootFifth{LayoutInstrumentID: r.LayoutInstrumentID, ChordSymbol: r.Chord, Root: r.Root, Fifth: r.Fifth}
	}}
}

// RootChangeItem is two chords' roots on a bass layout, changed between for a
// minute, as a drill item. TargetChangesPerMinute is the change target in
// force.
type RootChangeItem struct {
	LayoutInstrumentID     string
	From, To               string
	FromRoot, ToRoot       Place
	TargetChangesPerMinute int
}

// NewRootChangeItem places from's and to's roots on layout: see bassPlace.
func NewRootChangeItem(layout Instrument, from, to string, target int) (RootChangeItem, error) {
	places := make([]Place, 2)
	for i, chord := range []string{from, to} {
		root, err := chordRootPitchClass(chord)
		if err != nil {
			return RootChangeItem{}, err
		}
		if places[i], err = bassPlace(layout, root); err != nil {
			return RootChangeItem{}, err
		}
	}
	return RootChangeItem{LayoutInstrumentID: layout.ID, From: from, To: to, FromRoot: places[0], ToRoot: places[1], TargetChangesPerMinute: target}, nil
}

func (r RootChangeItem) ItemKey() string {
	return RootChangeItemKey(r.LayoutInstrumentID, r.From, r.To)
}

// RootChangeItemKey is the item key of the changes between from's and to's
// roots on the bass layout layoutInstrumentID.
func RootChangeItemKey(layoutInstrumentID, from, to string) string {
	return fmt.Sprintf("%s:%s:%s:%s", PracticeItemKindRootChange, layoutInstrumentID, from, to)
}
func (RootChangeItem) Kind() PracticeItemKind { return PracticeItemKindRootChange }
func (RootChangeItem) Ways() []string         { return []string{chordWay} }
func (RootChangeItem) Seconds() int           { return ChordChangeSeconds }

// SuitsSession holds in hand only: a change is played.
func (RootChangeItem) SuitsSession(inHand bool) bool { return inHand }

func (r RootChangeItem) Plan(_ string, planning DrillPlanning) PlannedDrill {
	minute := planChangeMinute(planning.State, r.TargetChangesPerMinute)
	return plannedChordItem{places: []Place{r.FromRoot, r.ToRoot}, attach: func(item *PracticeSessionItem) {
		item.Kind = PracticeItemKindRootChange
		item.RootChange = &PlannedRootChange{
			LayoutInstrumentID: r.LayoutInstrumentID, FromChordSymbol: r.From, ToChordSymbol: r.To,
			FromRoot: r.FromRoot, ToRoot: r.ToRoot, Minute: minute,
		}
	}}
}

// plannedChordItem is a chord drill item as a session plays it: rated, never
// asked, so its places are all given.
type plannedChordItem struct {
	places []Place
	attach func(item *PracticeSessionItem)
}

func (plannedChordItem) Way() string                          { return chordWay }
func (p plannedChordItem) Stimulus() DrillStimulus            { return playedStimulus(p.places...) }
func (p plannedChordItem) AttachTo(item *PracticeSessionItem) { p.attach(item) }

// bassFretWindow is the frets a bass chord drill marks its places within:
// the open string and the first five frets, which hold every pitch class on
// a bass in fourths.
const bassFretWindow = 5

// bassPlace is where pitchClass is played on layout within the first frets:
// the lowest fret, then the lowest-pitched string.
func bassPlace(layout Instrument, pitchClass int) (Place, error) {
	count := len(layout.Tuning)
	for fret := 0; fret <= bassFretWindow; fret++ {
		// The tuning lists the lowest string first, which is the highest
		// string number.
		for i, open := range layout.Tuning {
			midi, ok := ParsePitch(open)
			if !ok {
				return Place{}, NewValidationError("tuning", "the layout's tuning is not in scientific pitch notation")
			}
			if (midi+fret)%12 == pitchClass {
				return Place{String: count - i, Fret: fret}, nil
			}
		}
	}
	return Place{}, NewValidationError("tuning", fmt.Sprintf("pitch class %d has no place within the first %d frets", pitchClass, bassFretWindow))
}

// chordRootPitchClass is the pitch class of chord's root.
func chordRootPitchClass(chord string) (int, error) {
	reading := ParseChordSymbol(chord)
	if reading.Parsed == nil || strings.Contains(chord, "/") {
		return 0, NewValidationError("chord", fmt.Sprintf("%q is not a chord symbol", chord))
	}
	return reading.Parsed.RootPitchClass, nil
}
