package domain

import (
	"regexp"
	"slices"
	"strings"
)

// catalogRoots spells each pitch class as the chord catalog spells roots, C first.
var catalogRoots = [12]string{"C", "Db", "D", "Eb", "E", "F", "F#", "G", "Ab", "A", "Bb", "B"}

// chordQuality is a chord's quality, as far as harmony drills tell them apart.
type chordQuality string

const (
	qualityMajor           chordQuality = "major"
	qualityMinor           chordQuality = "minor"
	qualityDiminished      chordQuality = "diminished"
	qualityDominant7th     chordQuality = "dominant_7"
	qualityMajor7th        chordQuality = "major_7"
	qualityMinor7th        chordQuality = "minor_7"
	qualityHalfDiminished7 chordQuality = "half_diminished_7"
	qualityDiminished7th   chordQuality = "diminished_7"
)

// qualitySuffix is the symbol suffix of each quality.
var qualitySuffix = map[chordQuality]string{
	qualityMajor: "", qualityMinor: "m", qualityDiminished: "dim", qualityDominant7th: "7",
	qualityMajor7th: "maj7", qualityMinor7th: "m7", qualityHalfDiminished7: "m7b5", qualityDiminished7th: "dim7",
}

// suffixQuality reads a symbol suffix back into its quality.
var suffixQuality = func() map[string]chordQuality {
	qualities := make(map[string]chordQuality, len(qualitySuffix))
	for quality, suffix := range qualitySuffix {
		qualities[suffix] = quality
	}
	return qualities
}()

// chord is a root's pitch class and a quality. Two spellings of one root are one chord.
type chord struct {
	root    int
	quality chordQuality
}

// symbol spells the chord as the chord catalog does.
func (c chord) symbol() string { return catalogRoots[c.root] + qualitySuffix[c.quality] }

// third is the pitch class of the chord's third: minor on every chord over a minor
// or diminished triad.
func (c chord) third() int {
	switch c.quality {
	case qualityMinor, qualityDiminished, qualityMinor7th, qualityHalfDiminished7, qualityDiminished7th:
		return (c.root + 3) % 12
	case qualityMajor, qualityDominant7th, qualityMajor7th:
	}
	return (c.root + 4) % 12
}

// numeralRoots lists each root with its degree in the major scale, longer spellings
// before the ones they start with.
var numeralRoots = []struct {
	upper, lower string
	semitones    int
}{
	{"VII", "vii", 11}, {"III", "iii", 4}, {"VI", "vi", 9}, {"IV", "iv", 5},
	{"II", "ii", 2}, {"V", "v", 7}, {"I", "i", 0},
}

// Upper case is a major chord, lower case a minor or diminished one; viio is the
// catalog's spelling of vii°.
var (
	upperNumeralSuffixes = map[string]chordQuality{"": qualityMajor, "7": qualityDominant7th, "maj7": qualityMajor7th}
	lowerNumeralSuffixes = map[string]chordQuality{
		"": qualityMinor, "7": qualityMinor7th, "°": qualityDiminished, "o": qualityDiminished,
		"ø7": qualityHalfDiminished7, "°7": qualityDiminished7th,
	}
)

// readNumeral is a numeral's root above the tonic and its chord's quality, in
// either accidental; false for a numeral that isn't one.
func readNumeral(numeral string) (int, chordQuality, bool) {
	rest, shift := numeral, 0
	if after, ok := strings.CutPrefix(rest, "♭"); ok {
		rest, shift = after, -1
	} else if after, ok := strings.CutPrefix(rest, "♯"); ok {
		rest, shift = after, 1
	}
	for _, root := range numeralRoots {
		if suffix, ok := strings.CutPrefix(rest, root.upper); ok {
			quality, known := upperNumeralSuffixes[suffix]
			return (root.semitones + shift + 12) % 12, quality, known
		}
		if suffix, ok := strings.CutPrefix(rest, root.lower); ok {
			quality, known := lowerNumeralSuffixes[suffix]
			return (root.semitones + shift + 12) % 12, quality, known
		}
	}
	return 0, "", false
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
	semitones, quality, ok := readNumeral(numeral)
	if !ok {
		return chord{}, false
	}
	return chord{root: (key + semitones) % 12, quality: quality}, true
}

// seventhOf is the pitch class of numeral's seventh in the key. A written suffix
// decides: maj7 major, 7 and ø7 minor, °7 diminished. A chord with no written
// seventh takes the major seventh on I and IV and the minor seventh on any other
// numeral, in or out of the key.
func seventhOf(key int, numeral string) int {
	c, _ := chordOf(key, numeral)
	switch {
	case c.quality == qualityMajor7th, numeral == "I", numeral == "IV":
		return (c.root + 11) % 12
	case c.quality == qualityDiminished7th:
		return (c.root + 9) % 12
	}
	return (c.root + 10) % 12
}

// minorMode is the mode a minor progression's numerals are read in; any other
// progression's are read in major.
const minorMode = "minor"

// modeOf is p's mode: minor, or major when it says nothing else.
func modeOf(p ProgressionReference) string {
	if p.Mode == minorMode {
		return minorMode
	}
	return "major"
}

// musicKeyOf reads a key a response names: a major key, its tonic (G), or a minor
// key, its tonic chord (Gm). It reports the tonic's pitch class, the key's mode,
// and false for anything else.
func musicKeyOf(key string) (int, string, bool) {
	if tonic, ok := strings.CutSuffix(key, "m"); ok {
		pc, ok := pitchClass(tonic)
		return pc, minorMode, ok
	}
	pc, ok := pitchClass(key)
	return pc, "major", ok
}

// asciiNumerals turns a created progression's key back into its numerals' symbols.
var asciiNumerals = strings.NewReplacer("b", "♭", "#", "♯", "o", "°", "h", "ø")

// minorBareSevenths is how far above its root the seventh of a minor key's chord
// with no written seventh is: natural minor's, with harmonic minor's on V and vii°.
var minorBareSevenths = map[string]int{
	"i": 10, "ii°": 10, "iio": 10, "♭III": 11, "iv": 10, "v": 10, "♭VI": 11, "♭VII": 10,
	"V": 10, "vii°": 9, "viio": 9,
}

// minorSeventhOf is the pitch class of numeral's seventh in a minor key. A
// written suffix decides as in a major key; a chord with no written seventh
// takes minorBareSevenths, or on any other numeral the seventh of its quality:
// major on a major chord, minor on a minor or diminished one.
func minorSeventhOf(key int, numeral string) int {
	c, _ := chordOf(key, numeral)
	if !slices.Contains([]chordQuality{qualityMajor, qualityMinor, qualityDiminished}, c.quality) {
		return seventhOf(key, numeral)
	}
	if above, ok := minorBareSevenths[numeral]; ok {
		return (c.root + above) % 12
	}
	if c.quality == qualityMajor {
		return (c.root + 11) % 12
	}
	return (c.root + 10) % 12
}

// chordSymbol is a root in either spelling, then a suffix of qualitySuffix.
var chordSymbol = regexp.MustCompile(`^([A-G][#b]?)(maj7|m7b5|m7|dim7|m|dim|7)?$`)

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
	quality := suffixQuality[m[2]]
	seventh := strings.Contains(m[2], "7")
	return chord{root: root, quality: quality}, !seventh || allowSevenths
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
