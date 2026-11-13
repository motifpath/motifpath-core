package domain

import (
	"slices"
	"strings"
)

// The ear drills: a numeral heard after the key is set, a progression heard
// in a key, and a chord's quality heard alone. They are generated from the
// major keys, the progression catalog and the qualities, and recognised in
// the head on any instrument.
const (
	PracticeItemKindHeardChord        PracticeItemKind = "heard_chord"
	PracticeItemKindHeardProgression  PracticeItemKind = "heard_progression"
	PracticeItemKindHeardChordQuality PracticeItemKind = "heard_chord_quality"
)

// HeardProgressionBars is how many bars a heard progression has: a longer
// one is too long to hold in mind before answering.
const HeardProgressionBars = 4

// IsHeardProgressionLength says whether a drill progression of bars is asked
// by ear: 2 to HeardProgressionBars, since a longer one is too long to hold
// before answering.
func IsHeardProgressionLength(bars int) bool {
	return bars >= minDrillBars && bars <= HeardProgressionBars
}

// HearItemSeconds is how long a question asked by ear takes in a session:
// up to five bars heard at the slowest starting tempo, then the answer.
const HearItemSeconds = 20

// GeneratedPlayback is what a question asked by ear plays before it can be
// answered: bars in 4/4, one chord a bar, from TempoBPM in NoteValue, in the
// chords' voicings, with each chord's root under it in the voice of the bass
// BassInstrumentID names; no root line when it is empty.
type GeneratedPlayback struct {
	TempoBPM         int
	NoteValue        TempoNoteValue
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

// A question asked by ear starts at HearTempoBPM in HearNoteValue, two
// chords a bar, whatever the student's level: its tempo is never graded, so
// a faster start would not make it harder.
const (
	HearTempoBPM  = 70
	HearNoteValue = TempoNoteValueEighths
)

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
// offers it among its confusables, each named by every bar's numeral so the
// numerals shown always match the chords heard.
func (i HeardProgressionItem) Plan(way string, planning DrillPlanning) PlannedDrill {
	keys := MusicKeysOf(ProgressionMode(i.Progression.Mode))
	key := keys[planning.Pick(len(keys))]
	q := &PlannedGeneratedChoice{AskedWay: way, MusicKey: key, Answer: i.Progression.Key, itemKind: PracticeItemKindHeardProgression}
	var bars []PlaybackBar
	for _, chord := range ProgressionChart(i.Progression.Bars, key) {
		bars = append(bars, PlaybackBar{Chord: chord})
	}
	q.play(bars, planning)
	for _, p := range confusableProgressions(i.Progression, i.Catalog, planning.Pick) {
		q.Options = append(q.Options, GeneratedOption{Value: p.Key, Names: barNames(p)})
	}
	q.Options = shuffled(q.Options, planning.Pick)
	return q
}

// barNames names p by every bar's numeral in order, in each of its names'
// languages and in English.
func barNames(p Progression) LocalizedText {
	name := strings.Join(p.Bars, "–")
	names := LocalizedText{"en": name}
	for language := range p.Names {
		names[language] = name
	}
	return names
}

// HeardChordQualities are the qualities a heard chord quality asks, in the
// order its options show them. The first heardQualityOptions of them are asked
// first, among themselves until the student is fluent on the item; the others
// wait until the student is fluent on those, and are always asked among all.
var HeardChordQualities = []string{"major", "minor", "dominant7", "diminished", "augmented"}

const heardQualityOptions = 3

// heardQualityChord is the chord quality each heard quality sounds.
var heardQualityChord = map[string]ChordQuality{
	"major":      ChordQualityMajor,
	"minor":      ChordQualityMinor,
	"dominant7":  ChordQualityDominant7,
	"diminished": ChordQualityDiminished,
	"augmented":  ChordQualityAugmented,
}

// HeardChordQualityItem is a chord's quality heard alone, on a root picked
// each time it is asked, in no key.
type HeardChordQualityItem struct {
	Quality string
}

// ItemKey is heard_chord_quality:<quality>.
func (i HeardChordQualityItem) ItemKey() string {
	return string(PracticeItemKindHeardChordQuality) + ":" + i.Quality
}

// Kind is a heard chord quality's.
func (HeardChordQualityItem) Kind() PracticeItemKind { return PracticeItemKindHeardChordQuality }

// Ways hears the chord: it has no second way, and comes back later in the
// session instead.
func (HeardChordQualityItem) Ways() []string { return []string{"chord_quality_by_ear"} }

// Seconds is HearItemSeconds.
func (HeardChordQualityItem) Seconds() int { return HearItemSeconds }

// SuitsSession holds in the head only.
func (HeardChordQualityItem) SuitsSession(inHand bool) bool { return !inHand }

// WaitsOnItems holds a quality past the first heardQualityOptions back until
// the student is fluent on each of those: before then, it could only be asked
// among options that leave it out.
func (i HeardChordQualityItem) WaitsOnItems() ([]string, KnowledgeLevel) {
	if slices.Index(HeardChordQualities, i.Quality) < heardQualityOptions {
		return nil, ""
	}
	keys := make([]string, heardQualityOptions)
	for n, quality := range HeardChordQualities[:heardQualityOptions] {
		keys[n] = HeardChordQualityItem{Quality: quality}.ItemKey()
	}
	return keys, KnowledgeLevelFluent
}

// Plan picks the root and plays the chord alone for one bar, nothing setting
// a key. It offers every quality, always in the same order so where the answer
// sits says nothing; one of the first heardQualityOptions qualities is offered
// among those only until the student is fluent on it.
func (i HeardChordQualityItem) Plan(way string, planning DrillPlanning) PlannedDrill {
	root := MusicKeys[planning.Pick(len(MusicKeys))]
	q := &PlannedGeneratedChoice{AskedWay: way, Answer: i.Quality, itemKind: PracticeItemKindHeardChordQuality}
	q.play([]PlaybackBar{{Chord: root + heardQualityChord[i.Quality].CanonicalSuffix()}}, planning)
	qualities := HeardChordQualities
	if !fluent(planning.Level) && slices.Index(qualities, i.Quality) < heardQualityOptions {
		qualities = qualities[:heardQualityOptions]
	}
	q.Options = valueOptions(qualities)
	return q
}

// ParseHeardChordQualityItemKey reads a heard_chord_quality item key; false
// for any other key, or one whose quality isn't a heard one.
func ParseHeardChordQualityItemKey(itemKey string) (HeardChordQualityItem, bool) {
	quality, found := strings.CutPrefix(itemKey, string(PracticeItemKindHeardChordQuality)+":")
	if !found || !slices.Contains(HeardChordQualities, quality) {
		return HeardChordQualityItem{}, false
	}
	return HeardChordQualityItem{Quality: quality}, true
}

// play sets the question's playback to bars, at the tempo of the student's
// level, with the voicing of every chord it sounds whatever the session's
// reading. Nothing is drawn, so its chords count as symbols.
func (q *PlannedGeneratedChoice) play(bars []PlaybackBar, planning DrillPlanning) {
	q.Playback = &GeneratedPlayback{TempoBPM: HearTempoBPM, NoteValue: HearNoteValue, BassInstrumentID: planning.BassInstrumentID, Bars: bars}
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
