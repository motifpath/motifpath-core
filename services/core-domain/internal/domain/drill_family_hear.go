package domain

import (
	"slices"
	"strings"
)

// The ear drills: a numeral heard after the key is set, and a progression
// heard in a key. They are generated from the major keys and the
// progression catalog, and recognised in the head on any instrument.
const (
	PracticeItemKindHeardChord       PracticeItemKind = "heard_chord"
	PracticeItemKindHeardProgression PracticeItemKind = "heard_progression"
)

// HeardProgressionBars is how many bars a heard progression has: a longer
// one is too long to hold in mind before answering.
const HeardProgressionBars = 4

// HearItemSeconds is how long a question asked by ear takes in a session:
// up to five bars heard at the slowest starting tempo, then the answer.
const HearItemSeconds = 20

// GeneratedPlayback is what a question asked by ear plays before it can be
// answered: bars in 4/4, one chord a bar, from TempoBPM, in the chords'
// voicings, with each chord's root under it in the voice of the bass
// BassInstrumentID names; no root line when it is empty.
type GeneratedPlayback struct {
	TempoBPM         int
	BassInstrumentID string
	Bars             []PlaybackBar
}

// PlaybackBar is one bar of a playback: the chord sounded, never shown,
// and the numeral shown on the bar, empty for a bar whose chord is asked
// or a progression's bar.
type PlaybackBar struct {
	Chord   string
	Numeral string
}

// HearTempoBPM is the tempo a question asked by ear starts at, by the
// student's level on the item: slow while it is new or being learnt,
// faster once accurate, and again once fluent.
func HearTempoBPM(level KnowledgeLevel) int {
	switch level {
	case KnowledgeLevelAccurate:
		return 90
	case KnowledgeLevelFluent, KnowledgeLevelRetained:
		return 110
	case KnowledgeLevelNew, KnowledgeLevelLearning:
		return 70
	}
	return 70
}

// cadenceNumerals are the numerals of the cadence that sets the key.
var cadenceNumerals = []string{"I", "IV", "V", "I"}

// HeardChordItem is a numeral heard in a key picked each time it is asked,
// once the key is set; or, its second way, the numeral of the key's chord
// drawn on the screen.
type HeardChordItem struct {
	Numeral string
}

// ItemKey is heard_chord:<numeral>.
func (i HeardChordItem) ItemKey() string { return string(PracticeItemKindHeardChord) + ":" + i.Numeral }

// Kind is a heard chord's.
func (HeardChordItem) Kind() PracticeItemKind { return PracticeItemKindHeardChord }

// Ways hears the chord first, then sees it.
func (HeardChordItem) Ways() []string { return []string{"cadence_then_chord", "see_then_name"} }

// Seconds is HearItemSeconds.
func (HeardChordItem) Seconds() int { return HearItemSeconds }

// SuitsSession holds in the head only.
func (HeardChordItem) SuitsSession(inHand bool) bool { return !inHand }

// Plan picks the key, then plays the cadence (the tonic alone once the
// student is accurate on the item) and the chord, or draws the chord; it
// offers the numeral among the three nearest it, or all seven in scale
// order once the student is fluent on the item.
func (i HeardChordItem) Plan(way string, planning DrillPlanning) PlannedDrill {
	key := MusicKeys[planning.Pick(len(MusicKeys))]
	q := &PlannedGeneratedChoice{AskedWay: way, MusicKey: key, Answer: i.Numeral, itemKind: PracticeItemKindHeardChord}
	if way == "see_then_name" {
		q.Chord = NumeralChord(key, i.Numeral)
		q.draw([]string{q.Chord}, planning)
	} else {
		var bars []PlaybackBar
		for _, n := range keySetting(planning.Level) {
			bars = append(bars, PlaybackBar{Chord: NumeralChord(key, n), Numeral: n})
		}
		q.play(append(bars, PlaybackBar{Chord: NumeralChord(key, i.Numeral)}), planning)
	}
	if fluent(planning.Level) {
		q.Options = valueOptions(ChordNumerals)
		return q
	}
	q.Options = shuffled(valueOptions(nearestNumerals(i.Numeral, planning.Pick)), planning.Pick)
	return q
}

// keySetting is what sets the key before a heard chord: the cadence, or
// the tonic alone once the student is accurate on it.
func keySetting(level KnowledgeLevel) []string {
	if level == KnowledgeLevelAccurate || fluent(level) {
		return cadenceNumerals[:1]
	}
	return cadenceNumerals
}

func fluent(level KnowledgeLevel) bool {
	return level == KnowledgeLevelFluent || level == KnowledgeLevelRetained
}

// ParseHeardChordItemKey reads a heard_chord item key; false for any other
// key, or one whose numeral isn't a heard chord's.
func ParseHeardChordItemKey(itemKey string) (HeardChordItem, bool) {
	numeral, found := strings.CutPrefix(itemKey, string(PracticeItemKindHeardChord)+":")
	if !found || !slices.Contains(HarmonyItemNumerals, numeral) {
		return HeardChordItem{}, false
	}
	return HeardChordItem{Numeral: numeral}, true
}

// HeardProgressionItem is a 4-bar progression of the catalog heard in a key
// picked each time it is asked. Catalog is every progression, the
// confusables offered as options among them.
type HeardProgressionItem struct {
	Progression Progression
	Catalog     []Progression
}

// ItemKey is heard_progression:<progression key>.
func (i HeardProgressionItem) ItemKey() string { return HeardProgressionItemKey(i.Progression.Key) }

// HeardProgressionItemKey is the heard progression item of the progression
// with progressionKey.
func HeardProgressionItemKey(progressionKey string) string {
	return string(PracticeItemKindHeardProgression) + ":" + progressionKey
}

// Kind is a heard progression's.
func (HeardProgressionItem) Kind() PracticeItemKind { return PracticeItemKindHeardProgression }

// Ways hears the progression: it has no second way, and comes back later
// in the session instead.
func (HeardProgressionItem) Ways() []string { return []string{"progression_by_number"} }

// Seconds is HearItemSeconds.
func (HeardProgressionItem) Seconds() int { return HearItemSeconds }

// SuitsSession holds in the head only.
func (HeardProgressionItem) SuitsSession(inHand bool) bool { return !inHand }

// ConfusableWith lists the items of the progressions it is mistaken for.
func (i HeardProgressionItem) ConfusableWith() []string {
	keys := make([]string, len(i.Progression.Confusable))
	for n, key := range i.Progression.Confusable {
		keys[n] = HeardProgressionItemKey(key)
	}
	return keys
}

// Plan picks the key, plays the progression's bars with no numeral, and
// offers it among its confusables.
func (i HeardProgressionItem) Plan(way string, planning DrillPlanning) PlannedDrill {
	key := MusicKeys[planning.Pick(len(MusicKeys))]
	q := &PlannedGeneratedChoice{AskedWay: way, MusicKey: key, Answer: i.Progression.Key, itemKind: PracticeItemKindHeardProgression}
	var bars []PlaybackBar
	for _, chord := range ProgressionChart(i.Progression.Bars, key) {
		bars = append(bars, PlaybackBar{Chord: chord})
	}
	q.play(bars, planning)
	for _, p := range confusableProgressions(i.Progression, i.Catalog, planning.Pick) {
		q.Options = append(q.Options, GeneratedOption{Value: p.Key, Names: p.Names})
	}
	q.Options = shuffled(q.Options, planning.Pick)
	return q
}

// play sets the question's playback to bars, at the tempo of the student's
// level, with the voicing of every chord it sounds whatever the session's
// reading. Nothing is drawn, so its chords count as symbols.
func (q *PlannedGeneratedChoice) play(bars []PlaybackBar, planning DrillPlanning) {
	q.Playback = &GeneratedPlayback{TempoBPM: HearTempoBPM(planning.Level), BassInstrumentID: planning.BassInstrumentID, Bars: bars}
	q.DrawnAs, q.Voicings = GeneratedChordSymbols, nil
	if planning.Voicing == nil {
		return
	}
	for _, bar := range bars {
		if slices.ContainsFunc(q.Voicings, func(b ChordBox) bool { return b.Chord == bar.Chord }) {
			continue
		}
		if id, ok := planning.Voicing(bar.Chord); ok {
			q.Voicings = append(q.Voicings, ChordBox{Chord: bar.Chord, DiagramID: id})
		}
	}
}
