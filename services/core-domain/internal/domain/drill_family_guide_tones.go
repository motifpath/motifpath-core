package domain

import (
	"fmt"
	"slices"
)

// PracticeItemKindGuideTone is a guide tone: a progression of the progression
// catalog played once in a key while the student taps a chord tone of each
// chord on the fretboard as it sounds.
const PracticeItemKindGuideTone PracticeItemKind = "guide_tone"

const (
	// GuideToneTargetTempoBPM is the tempo, in quarters, a right take's
	// effective speed is measured against for fluency.
	GuideToneTargetTempoBPM = 90
	// MaxGuideTonesPerSession caps the guide tones one session asks, and
	// guideToneTakes the takes a session gives one.
	MaxGuideTonesPerSession = 1
	guideToneTakes          = 3
	// guideToneResultSeconds is the time to see a take's result.
	guideToneResultSeconds = 5
	// guideToneWay is the one way a guide tone is asked, its drill template's.
	guideToneWay = "tap_the_third"
)

// guideToneWindow is the frets every guide tone is tapped in: the open string
// and frets 1 to 5, which hold every pitch class on guitar and on bass.
var guideToneWindow = FretWindow{FromFret: 0}

// GuideToneTempoBPM is the tempo a guide tone's first take starts at, by the
// student's level on the item: slow while it is new or being learnt, faster
// once accurate (as fast as the fluency fluent needs), the target once fluent.
func GuideToneTempoBPM(level KnowledgeLevel) int {
	switch level {
	case KnowledgeLevelAccurate:
		return 72
	case KnowledgeLevelFluent, KnowledgeLevelRetained:
		return GuideToneTargetTempoBPM
	case KnowledgeLevelNew, KnowledgeLevelLearning:
		return 60
	}
	return 60
}

// guideToneTone is the chord tone asked of every chord: the third until the
// student is accurate on the item, then the seventh.
func guideToneTone(level KnowledgeLevel) string {
	if level.Rank() >= KnowledgeLevelAccurate.Rank() {
		return "7"
	}
	return "3"
}

// PlannedGuideTone is a guide tone as a session asks it: the progression in a
// key, the tone asked, the tempo its first take starts at and the frets it is
// tapped in. It names the chords, never the notes to tap or their places.
type PlannedGuideTone struct {
	LayoutInstrumentID string
	ProgressionKey     string
	ProgressionNames   LocalizedText
	MusicKey           string
	Tone               string
	TempoBPM           int
	TargetTempoBPM     int
	Takes              int
	FretWindow         FretWindow
	Bars               []GuideToneBar
	Voicings           []ChordBox
}

// GuideToneBar is one bar of a guide tone: its numeral and its chord in the key.
type GuideToneBar struct {
	Numeral string
	Chord   string
}

// GuideToneItem is a progression whose guide tones are tapped on a layout, as a
// drill item.
type GuideToneItem struct {
	Layout      Instrument
	Progression Progression
}

func (i GuideToneItem) ItemKey() string {
	return GuideToneItemKey(i.Layout.ID, i.Progression.Key)
}

// GuideToneItemKey is the item key of progressionKey's guide tone on the layout
// layoutInstrumentID.
func GuideToneItemKey(layoutInstrumentID, progressionKey string) string {
	return fmt.Sprintf("%s:%s:%s", PracticeItemKindGuideTone, layoutInstrumentID, progressionKey)
}

func (GuideToneItem) Kind() PracticeItemKind { return PracticeItemKindGuideTone }
func (GuideToneItem) Ways() []string         { return []string{guideToneWay} }

// Seconds sizes a guide tone by its takes at the slowest start tempo: each a bar
// of count-in and the progression once, then its result. A faster start takes
// less, so the estimate never runs short.
func (i GuideToneItem) Seconds() int {
	beats := (len(i.Progression.Bars) + 1) * tourBeatsPerBar
	take := beats*60/GuideToneTempoBPM(KnowledgeLevelNew) + guideToneResultSeconds
	return guideToneTakes * take
}

// SuitsSession holds in the head only: the tones are tapped on the screen,
// nothing is played on the instrument.
func (GuideToneItem) SuitsSession(inHand bool) bool { return !inHand }

// guideToneTwelveBarNode is the node a 12-bar guide tone waits on: the form is
// planned only once the student is accurate on targeting chord tones.
const guideToneTwelveBarNode = "target-chord-tones"

// WaitsOn holds a 12-bar form back until the student is accurate on targeting
// chord tones; a four-bar progression waits on nothing.
func (i GuideToneItem) WaitsOn() (string, KnowledgeLevel, bool) {
	if len(i.Progression.Bars) > HeardProgressionBars {
		return guideToneTwelveBarNode, KnowledgeLevelAccurate, true
	}
	return "", "", false
}

// Plan picks what this time asks: the tone and tempo by the student's level on
// the item, and a major key at random, never the key of its last take.
func (i GuideToneItem) Plan(_ string, planning DrillPlanning) PlannedDrill {
	keys := MusicKeys
	if planning.State != nil && planning.State.LastMusicKey != "" {
		keys = slices.DeleteFunc(slices.Clone(MusicKeys), func(k string) bool { return k == planning.State.LastMusicKey })
	}
	key := keys[planning.Pick(len(keys))]
	planned := &PlannedGuideTone{
		LayoutInstrumentID: i.Layout.ID,
		ProgressionKey:     i.Progression.Key,
		ProgressionNames:   i.Progression.Names,
		MusicKey:           key,
		Tone:               guideToneTone(planning.Level),
		TempoBPM:           GuideToneTempoBPM(planning.Level),
		TargetTempoBPM:     GuideToneTargetTempoBPM,
		Takes:              guideToneTakes,
		FretWindow:         guideToneWindow,
	}
	chart := ProgressionChart(i.Progression.Bars, key)
	for n, numeral := range i.Progression.Bars {
		planned.Bars = append(planned.Bars, GuideToneBar{Numeral: numeral, Chord: chart[n]})
	}
	for _, chord := range distinct(chart) {
		if planning.Voicing == nil {
			break
		}
		if id, ok := planning.Voicing(chord); ok {
			planned.Voicings = append(planned.Voicings, ChordBox{Chord: chord, DiagramID: id})
		}
	}
	return plannedGuideTone{planned: planned}
}

// plannedGuideTone is a guide tone as a session asks it. Its stimulus is empty:
// the board shows nothing, and the answer is every bar's tone, never planned.
type plannedGuideTone struct {
	planned *PlannedGuideTone
}

func (plannedGuideTone) Way() string             { return guideToneWay }
func (plannedGuideTone) Stimulus() DrillStimulus { return DrillStimulus{} }
func (p plannedGuideTone) AttachTo(item *PracticeSessionItem) {
	item.Kind = PracticeItemKindGuideTone
	item.GuideTone = p.planned
}

// fixedRung is the rung of a kind that isn't timed, so its rung doesn't come
// from a timed template: the self-rated chord drills, tours and songs, and a
// guide tone, measured by tempo. False for any other kind.
func fixedRung(kind PracticeItemKind) (PracticeRung, bool) {
	if rung, rated := selfRatedChordKinds[kind]; rated {
		return rung, true
	}
	switch kind {
	case PracticeItemKindGuideTone:
		return PracticeRungTarget, true
	case PracticeItemKindSong:
		return PracticeRungUse, true
	}
	return "", false
}

// kindSessionCap is the most items of kind a session takes, for the kinds
// capped by kind rather than by template; false for any other.
func kindSessionCap(kind PracticeItemKind) (int, bool) {
	if most, ok := chordSessionCaps[kind]; ok {
		return most, true
	}
	switch kind {
	case PracticeItemKindGuideTone:
		return MaxGuideTonesPerSession, true
	case PracticeItemKindSong:
		return MaxSongsPerSession, true
	}
	return 0, false
}
