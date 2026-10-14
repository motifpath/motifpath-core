package domain

import (
	"regexp"
	"strings"
)

// catalogRoots spells each pitch class as the chord catalog spells roots, C first.
var catalogRoots = [12]string{"C", "Db", "D", "Eb", "E", "F", "F#", "G", "Ab", "A", "Bb", "B"}

// chordQuality is a chord's quality, as far as harmony drills tell them apart.
type chordQuality string

const (
	qualityMajor       chordQuality = "major"
	qualityMinor       chordQuality = "minor"
	qualityDiminished  chordQuality = "diminished"
	qualityDominant7th chordQuality = "dominant_7"
)

// qualitySuffix is the symbol suffix of each quality.
var qualitySuffix = map[chordQuality]string{
	qualityMajor: "", qualityMinor: "m", qualityDiminished: "dim", qualityDominant7th: "7",
}

// chord is a root's pitch class and a quality. Two spellings of one root are one chord.
type chord struct {
	root    int
	quality chordQuality
}

// symbol spells the chord as the chord catalog does.
func (c chord) symbol() string { return catalogRoots[c.root] + qualitySuffix[c.quality] }

// third is the pitch class of the chord's third: minor on minor and diminished chords.
func (c chord) third() int {
	if c.quality == qualityMinor || c.quality == qualityDiminished {
		return (c.root + 3) % 12
	}
	return (c.root + 4) % 12
}

// numeralDegree is where each numeral of a major key stands: its semitones above
// the key's root and its chord's quality. I7, IV7 and V7 are the dominant sevenths
// a blues progression uses.
var numeralDegree = map[string]struct {
	semitones int
	quality   chordQuality
}{
	"I": {0, qualityMajor}, "ii": {2, qualityMinor}, "iii": {4, qualityMinor}, "IV": {5, qualityMajor},
	"V": {7, qualityMajor}, "vi": {9, qualityMinor}, "viio": {11, qualityDiminished},
	"I7": {0, qualityDominant7th}, "IV7": {5, qualityDominant7th}, "V7": {7, qualityDominant7th},
}

// keyNumerals are the seven numerals of a major key, its triads in order.
var keyNumerals = []string{"I", "ii", "iii", "IV", "V", "vi", "viio"}

// numeralFunction is the harmonic function of each triad of a major key.
var numeralFunction = map[string]string{
	"I": "tonic", "iii": "tonic", "vi": "tonic",
	"ii": "subdominant", "IV": "subdominant",
	"V": "dominant", "viio": "dominant",
}

// chordOf is the chord on numeral in the major key whose root is key, and false for
// a numeral that isn't one.
func chordOf(key int, numeral string) (chord, bool) {
	d, ok := numeralDegree[numeral]
	if !ok {
		return chord{}, false
	}
	return chord{root: (key + d.semitones) % 12, quality: d.quality}, true
}

// seventhOf is the pitch class of numeral's seventh in the key: major on I and IV,
// minor on every other numeral and on a dominant seventh.
func seventhOf(key int, numeral string) int {
	c, _ := chordOf(key, numeral)
	if numeral == "I" || numeral == "IV" {
		return (c.root + 11) % 12
	}
	return (c.root + 10) % 12
}

// chordSymbol is a root in either spelling, then m, dim, 7 or nothing.
var chordSymbol = regexp.MustCompile(`^([A-G][#b]?)(m|dim|7)?$`)

// parseChord reads a chord symbol; a seventh is read only when sevenths are allowed.
func parseChord(symbol string, allowSevenths bool) (chord, bool) {
	m := chordSymbol.FindStringSubmatch(symbol)
	if m == nil {
		return chord{}, false
	}
	root, ok := pitchClass(m[1])
	if !ok {
		return chord{}, false
	}
	switch m[2] {
	case "":
		return chord{root: root, quality: qualityMajor}, true
	case "m":
		return chord{root: root, quality: qualityMinor}, true
	case "dim":
		return chord{root: root, quality: qualityDiminished}, true
	default:
		return chord{root: root, quality: qualityDominant7th}, allowSevenths
	}
}

// chartOf is a progression's chords in the key, one per bar.
func chartOf(key int, bars []string) ([]chord, bool) {
	chart := make([]chord, len(bars))
	for i, numeral := range bars {
		c, ok := chordOf(key, numeral)
		if !ok {
			return nil, false
		}
		chart[i] = c
	}
	return chart, true
}

// chartSymbol spells a chart as one chord symbol per bar, separated by single spaces.
func chartSymbol(chart []chord) string {
	symbols := make([]string, len(chart))
	for i, c := range chart {
		symbols[i] = c.symbol()
	}
	return strings.Join(symbols, " ")
}
