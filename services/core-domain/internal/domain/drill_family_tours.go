package domain

import (
	"fmt"
	"math"
	"slices"
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
	// tourTakeBars is how many bars a take fills with whole times through the
	// progression: a four-bar progression plays four times, a 12-bar form once.
	tourTakeBars = 16
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
	// BassPattern is what every bar of a take plays on bass, by the student's
	// level on the tour; empty on guitar.
	BassPattern BassPattern
	Keys        []PlannedTourKey
}

// BassPattern is what a bass key tour plays on every bar of 4/4.
type BassPattern string

// The bass patterns, by the student's level on the tour: the root as a whole
// note; the root then the fifth above, a half note each; root, third, fifth,
// third in quarters; root, third, fifth, seventh in quarters.
const (
	BassPatternRoot                  BassPattern = "root"
	BassPatternRootFifth             BassPattern = "root_fifth"
	BassPatternRootThirdFifthThird   BassPattern = "root_third_fifth_third"
	BassPatternRootThirdFifthSeventh BassPattern = "root_third_fifth_seventh"
)

// BassPatternOf is the pattern a student at level plays a bass tour in.
func BassPatternOf(level KnowledgeLevel) BassPattern {
	switch level {
	case KnowledgeLevelAccurate:
		return BassPatternRootFifth
	case KnowledgeLevelFluent:
		return BassPatternRootThirdFifthThird
	case KnowledgeLevelRetained:
		return BassPatternRootThirdFifthSeventh
	case KnowledgeLevelNew, KnowledgeLevelLearning:
		return BassPatternRoot
	}
	return BassPatternRoot
}

// PlannedTourKey is one key of a tour: whether it is clean at the tour tempo,
// the progression's chart in it, the voicing of each of its chords, and on bass
// the window its tones are in (nil on guitar) and each chord's tones there.
type PlannedTourKey struct {
	MusicKey        string
	Clean           bool
	Chart           []string
	Voicings        []ChordBox
	WindowStartFret *int
	ChordTones      []BassChordTones
}

// BassChordTones is where a chord's tones are played on a bass layout inside
// a key's window: the root, on the lowest string holding it there, and its
// third and fifth above and an octave below, and with the seventh pattern its
// seventh above. A tone no string reaches in the window is nil.
type BassChordTones struct {
	Chord                  string
	Root                   Place
	ThirdAbove, FifthAbove *Place
	ThirdBelow, FifthBelow *Place
	Seventh                *Place
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

// timesThrough is how many whole times the progression fits in a take's
// tourTakeBars, and at least once.
func (i ProgressionKeysItem) timesThrough() int {
	return max(1, tourTakeBars/max(1, len(i.Progression.Bars)))
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
	if i.Bass {
		planned.BassPattern = BassPatternOf(planning.Level)
	}
	order := TourKeyOrderOf(ProgressionMode(i.Progression.Mode))
	for _, key := range order {
		clean := slices.Contains(tour.CleanKeys, key)
		if !clean && planned.StartMusicKey == "" {
			planned.StartMusicKey = key
		}
		planned.Keys = append(planned.Keys, i.planKey(key, clean, planned.BassPattern, planning))
	}
	if planned.StartMusicKey == "" {
		planned.StartMusicKey = order[0]
	}
	return plannedTour{planned: planned}
}

// planKey draws the progression in key: its chart, a voicing for each chord
// the backing sounds, and on bass a window and each chord's tones in it. A
// chord without a voicing or a readable root is left out of that list.
func (i ProgressionKeysItem) planKey(key string, clean bool, pattern BassPattern, planning DrillPlanning) PlannedTourKey {
	k := PlannedTourKey{MusicKey: key, Clean: clean, Chart: ProgressionChart(i.Progression.Bars, key)}
	chords := distinct(k.Chart)
	for _, chord := range chords {
		if planning.Voicing != nil {
			if id, ok := planning.Voicing(chord); ok {
				k.Voicings = append(k.Voicings, ChordBox{Chord: chord, DiagramID: id})
			}
		}
	}
	if !i.Bass {
		return k
	}
	board, ok := tourBassBoard(i.Layout, chords, pattern, planning.Pick)
	if !ok {
		return k
	}
	start := board.window.start
	k.WindowStartFret = &start
	for _, chord := range chords {
		if tones, ok := board.chordTones(chord, pattern); ok {
			k.ChordTones = append(k.ChordTones, tones)
		}
	}
	return k
}

// tourBassBoard is the layout in a window the session picks for a key: among
// the windows where every chord's pattern tones are held above its root, or
// among all of them when none is.
func tourBassBoard(layout Instrument, chords []string, pattern BassPattern, pick func(n int) int) (bassBoard, bool) {
	var fitting, all []bassBoard
	for start := range bassWindowStarts {
		board, err := newBassBoard(layout, bassWindow{start: start})
		if err != nil {
			return bassBoard{}, false
		}
		all = append(all, board)
		if board.holdsPatterns(chords, pattern) {
			fitting = append(fitting, board)
		}
	}
	if len(fitting) == 0 {
		fitting = all
	}
	if pick == nil {
		return fitting[0], true
	}
	return fitting[pick(len(fitting))], true
}

// holdsPatterns reports whether every chord's pattern tones are held above
// its root in b's window.
func (b bassBoard) holdsPatterns(chords []string, pattern BassPattern) bool {
	for _, chord := range chords {
		root, err := chordRootPitchClass(chord)
		if err != nil {
			continue
		}
		pitch, ok := b.root(root, nil)
		if !ok {
			return false
		}
		for _, interval := range patternIntervals(chord, pattern) {
			if !b.holds(pitch + interval) {
				return false
			}
		}
	}
	return true
}

// patternIntervals are the semitones above chord's root that pattern plays.
func patternIntervals(chord string, pattern BassPattern) []int {
	third, fifth := chordThirdAndFifth(chord)
	switch pattern {
	case BassPatternRootFifth:
		return []int{fifth}
	case BassPatternRootThirdFifthThird:
		return []int{third, fifth}
	case BassPatternRootThirdFifthSeventh:
		if seventh, ok := chordSeventh(chord); ok {
			return []int{third, fifth, seventh}
		}
		return []int{third, fifth}
	case BassPatternRoot:
	}
	return nil
}

// chordTones places chord's tones in b's window: false when its root can't
// be read or isn't there.
func (b bassBoard) chordTones(chord string, pattern BassPattern) (BassChordTones, bool) {
	rootClass, err := chordRootPitchClass(chord)
	if err != nil {
		return BassChordTones{}, false
	}
	root, ok := b.root(rootClass, nil)
	if !ok {
		return BassChordTones{}, false
	}
	third, fifth := chordThirdAndFifth(chord)
	tones := BassChordTones{
		Chord: chord, Root: b.placeOf(root),
		ThirdAbove: b.optionalPlace(root + third), FifthAbove: b.optionalPlace(root + fifth),
		ThirdBelow: b.optionalPlace(root + third - 12), FifthBelow: b.optionalPlace(root + fifth - 12),
	}
	if seventh, ok := chordSeventh(chord); ok && pattern == BassPatternRootThirdFifthSeventh {
		tones.Seventh = b.optionalPlace(root + seventh)
	}
	return tones, true
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
// fifth: its triad's, since a seventh keeps the triad under it. Minor and
// diminished triads have a minor third, a diminished one a flat fifth.
func chordThirdAndFifth(chord string) (third, fifth int) {
	reading := ParseChordSymbol(chord)
	if reading.Parsed == nil {
		return 4, 7
	}
	switch triadOf[reading.Parsed.Quality] {
	case triadDiminished:
		return 3, 6
	case triadMinor:
		return 3, 7
	case triadMajor, triadNone:
		return 4, 7
	}
	return 4, 7
}

// chordSeventh is the semitones from chord's root to its own seventh; false
// for a chord without one.
func chordSeventh(chord string) (int, bool) {
	reading := ParseChordSymbol(chord)
	if reading.Parsed == nil {
		return 0, false
	}
	switch reading.Parsed.Quality {
	case ChordQualityDominant7, ChordQualityMinor7, ChordQualityHalfDiminished7, ChordQualityDominant7Sus4,
		ChordQualityDominant9, ChordQualityMinor9, ChordQualityDominant11, ChordQualityMinor11, ChordQualityDominant13,
		ChordQualityDominant7Flat5, ChordQualityDominant7Sharp5, ChordQualityDominant7Flat9, ChordQualityDominant7Sharp9:
		return 10, true
	case ChordQualityMajor7, ChordQualityMinorMajor7, ChordQualityMajor9:
		return 11, true
	case ChordQualityDiminished7:
		return 9, true
	case ChordQualityMajor, ChordQualityMinor, ChordQualityPower, ChordQualityDiminished, ChordQualityAugmented,
		ChordQualitySus2, ChordQualitySus4, ChordQualityMajor6, ChordQualityMinor6, ChordQualityAdd9, ChordQualityMinorAdd9:
	}
	return 0, false
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
