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
	// ChordChangeSeconds is how long a take of changes takes: a bar of
	// count-in, the cycles at the first tempo and the share picked.
	ChordChangeSeconds = 45
	// ChangeTakeCycles is the cycles one take of changes plays, one cycle
	// being from, to and back.
	ChangeTakeCycles = 10
	// RootFifthStartTempoBPM is where a root and fifth's take starts, in
	// quarters, each note a half note.
	RootFifthStartTempoBPM = 60
	// MaxChordVoicingsPerSession and MaxChordChangesPerSession cap the
	// single chords and the changes one session plays.
	MaxChordVoicingsPerSession = 4
	MaxChordChangesPerSession  = 2

	// The change ladder, in BPM, one change on every beat: a change never
	// played starts at changeFirstTempoBPM; two clean takes in a row raise
	// the next by changeTempoStepBPM and a struggled one lowers it as much,
	// never below changeMinTempoBPM.
	changeFirstTempoBPM = 40
	changeTempoStepBPM  = 5
	changeMinTempoBPM   = 20
	// changeCleanStreak is the clean takes in a row that raise the tempo.
	changeCleanStreak = 2

	// chordWay is the one way a chord item is answered: rated by the student
	// (a change's rating follows from its clean share).
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
// bass layout, inside the five-fret window starting at WindowStartFret: the
// root and the fifth above it, played, and the fifth below it, marked only
// (nil when no string reaches it in the window).
type PlannedRootFifth struct {
	LayoutInstrumentID string
	ChordSymbol        string
	WindowStartFret    int
	Root, Fifth        Place
	FifthBelow         *Place
	StartTempoBPM      int
}

// PlannedChangeTake is a take of changes as a session plays it: its cycles,
// where its tempo ladder starts, the target tempo, and the student's best
// clean tempo (nil before a clean take).
type PlannedChangeTake struct {
	Cycles            int
	StartTempoBPM     int
	TargetTempoBPM    int
	BestCleanTempoBPM *int
}

// PlannedChordChange is a take of changes between two chords' boxes.
type PlannedChordChange struct {
	FromDiagramID, ToDiagramID     string
	FromChordSymbol, ToChordSymbol string
	Take                           PlannedChangeTake
}

// PlannedRootChange is a take of changes between two chords' roots on a bass
// layout, both inside the five-fret window starting at WindowStartFret.
type PlannedRootChange struct {
	LayoutInstrumentID             string
	FromChordSymbol, ToChordSymbol string
	WindowStartFret                int
	FromRoot, ToRoot               Place
	Take                           PlannedChangeTake
}

// planChangeTake starts a change's take on its ladder from state, the
// student's state on it (nil when never played): the last take's tempo,
// raised after two clean takes in a row, lowered after a struggled one,
// never below the floor.
func planChangeTake(state *PracticeItemState, target int) PlannedChangeTake {
	take := PlannedChangeTake{Cycles: ChangeTakeCycles, StartTempoBPM: changeFirstTempoBPM, TargetTempoBPM: target}
	if state == nil {
		return take
	}
	take.BestCleanTempoBPM = state.BestCleanBPM
	last := state.LastChangeTake
	if last == nil {
		return take
	}
	tempo := last.TempoBPM
	switch {
	case last.Struggled:
		tempo -= changeTempoStepBPM
	case last.CleanInARow >= changeCleanStreak:
		tempo += changeTempoStepBPM
	}
	take.StartTempoBPM = max(tempo, changeMinTempoBPM)
	return take
}

// legacyStruggledPercent is the share of a minute's beats changed clean below
// which a minute counted before takes reads as a struggled take: below almost.
const legacyStruggledPercent = 75

// LegacyChangeTake reads a change's last minute, counted before ten-cycle
// takes, as a take: its clean share is the clean changes counted over the
// minute's beats, one change on every beat at tempoBPM, struggled below
// almost. The clean minutes in a row it ended carry over as clean takes.
func LegacyChangeTake(tempoBPM, cleanChanges, cleanMinutesInARow int) ChangeTakeState {
	return ChangeTakeState{
		TempoBPM:    tempoBPM,
		Struggled:   tempoBPM <= 0 || cleanChanges*100/tempoBPM < legacyStruggledPercent,
		CleanInARow: cleanMinutesInARow,
	}
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

// ChordChangeItem is a pair of chord drills, changed between take after
// take, as a drill item. TargetTempoBPM is the change target in force.
type ChordChangeItem struct {
	From, To       ChordDrill
	TargetTempoBPM int
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
	take := planChangeTake(planning.State, c.TargetTempoBPM)
	return plannedChordItem{attach: func(item *PracticeSessionItem) {
		item.Kind = PracticeItemKindChordChange
		item.ChordChange = &PlannedChordChange{
			FromDiagramID: c.From.DiagramID, ToDiagramID: c.To.DiagramID,
			FromChordSymbol: c.From.Chord, ToChordSymbol: c.To.Chord, Take: take,
		}
	}}
}

// UsesChord reports whether the change is to or from the voicing diagramID.
func (c ChordChangeItem) UsesChord(diagramID string) bool {
	return c.From.DiagramID == diagramID || c.To.DiagramID == diagramID
}

// RootFifthItem is a chord's root and fifth on a bass layout, as a drill
// item. Its places are picked when it is planned, in a window picked then.
type RootFifthItem struct {
	Layout Instrument
	Chord  string
	root   int
}

// NewRootFifthItem is chord's root and fifth on layout; it refuses a chord
// whose root it can't read.
func NewRootFifthItem(layout Instrument, chord string) (RootFifthItem, error) {
	root, err := chordRootPitchClass(chord)
	if err != nil {
		return RootFifthItem{}, err
	}
	return RootFifthItem{Layout: layout, Chord: chord, root: root}, nil
}

func (r RootFifthItem) ItemKey() string { return RootFifthItemKey(r.Layout.ID, r.Chord) }

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

// Plan places the root and fifth in a window the session picks: the root on
// the lowest string that holds it there with the fifth above it there too,
// and the fifth an octave below marked when a string reaches it.
func (r RootFifthItem) Plan(_ string, planning DrillPlanning) PlannedDrill {
	window := pickBassWindow(planning.Pick)
	planned := &PlannedRootFifth{
		LayoutInstrumentID: r.Layout.ID, ChordSymbol: r.Chord, WindowStartFret: window.start,
		StartTempoBPM: RootFifthStartTempoBPM,
	}
	var places []Place
	if board, err := newBassBoard(r.Layout, window); err == nil {
		if root, ok := board.root(r.root, []int{perfectFifth}); ok {
			planned.Root = board.placeOf(root)
			planned.Fifth = board.placeOf(root + perfectFifth)
			planned.FifthBelow = board.optionalPlace(root - perfectFourth)
			places = []Place{planned.Root, planned.Fifth}
		}
	}
	return plannedChordItem{places: places, attach: func(item *PracticeSessionItem) {
		item.Kind = PracticeItemKindRootFifth
		item.RootFifth = planned
	}}
}

// RootChangeItem is two chords' roots on a bass layout, changed between take
// after take, as a drill item. TargetTempoBPM is the change target in force.
type RootChangeItem struct {
	Layout           Instrument
	From, To         string
	TargetTempoBPM   int
	fromRoot, toRoot int
}

// NewRootChangeItem is the change between from's and to's roots on layout;
// it refuses a chord whose root it can't read.
func NewRootChangeItem(layout Instrument, from, to string, target int) (RootChangeItem, error) {
	fromRoot, err := chordRootPitchClass(from)
	if err != nil {
		return RootChangeItem{}, err
	}
	toRoot, err := chordRootPitchClass(to)
	if err != nil {
		return RootChangeItem{}, err
	}
	return RootChangeItem{Layout: layout, From: from, To: to, TargetTempoBPM: target, fromRoot: fromRoot, toRoot: toRoot}, nil
}

func (r RootChangeItem) ItemKey() string {
	return RootChangeItemKey(r.Layout.ID, r.From, r.To)
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

// Plan places both roots in a window the session picks, each on the lowest
// string that holds it there.
func (r RootChangeItem) Plan(_ string, planning DrillPlanning) PlannedDrill {
	window := pickBassWindow(planning.Pick)
	planned := &PlannedRootChange{
		LayoutInstrumentID: r.Layout.ID, FromChordSymbol: r.From, ToChordSymbol: r.To,
		WindowStartFret: window.start, Take: planChangeTake(planning.State, r.TargetTempoBPM),
	}
	var places []Place
	if board, err := newBassBoard(r.Layout, window); err == nil {
		from, fromOK := board.root(r.fromRoot, nil)
		to, toOK := board.root(r.toRoot, nil)
		if fromOK && toOK {
			planned.FromRoot, planned.ToRoot = board.placeOf(from), board.placeOf(to)
			places = []Place{planned.FromRoot, planned.ToRoot}
		}
	}
	return plannedChordItem{places: places, attach: func(item *PracticeSessionItem) {
		item.Kind = PracticeItemKindRootChange
		item.RootChange = planned
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

// chordRootPitchClass is the pitch class of chord's root.
func chordRootPitchClass(chord string) (int, error) {
	reading := ParseChordSymbol(chord)
	if reading.Parsed == nil || strings.Contains(chord, "/") {
		return 0, NewValidationError("chord", fmt.Sprintf("%q is not a chord symbol", chord))
	}
	return reading.Parsed.RootPitchClass, nil
}
