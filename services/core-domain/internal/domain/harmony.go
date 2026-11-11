package domain

import (
	"slices"
	"strings"
)

// MusicKeys are the twelve major keys, spelled as the chord catalog spells
// roots, by pitch class from C: a chord's root is always written this way,
// even where its key would spell it otherwise (the IV of D♭ is F#).
var MusicKeys = []string{"C", "Db", "D", "Eb", "E", "F", "F#", "G", "Ab", "A", "Bb", "B"}

// ChordFunction is a chord's harmonic function in a major key.
type ChordFunction string

const (
	// ChordFunctionTonic is at rest: I, iii and vi.
	ChordFunctionTonic ChordFunction = "tonic"
	// ChordFunctionSubdominant moves away: ii and IV.
	ChordFunctionSubdominant ChordFunction = "subdominant"
	// ChordFunctionDominant pulls home: V and viio.
	ChordFunctionDominant ChordFunction = "dominant"
)

// ChordFunctions are the three functions, in the order a question offers
// them.
var ChordFunctions = []ChordFunction{ChordFunctionTonic, ChordFunctionSubdominant, ChordFunctionDominant}

// numeralFunctions are the major key's triads' functions by numeral, viio being
// vii°.
var numeralFunctions = map[string]ChordFunction{
	"I": ChordFunctionTonic, "iii": ChordFunctionTonic, "vi": ChordFunctionTonic,
	"ii": ChordFunctionSubdominant, "IV": ChordFunctionSubdominant,
	"V": ChordFunctionDominant, "viio": ChordFunctionDominant,
}

// ChordNumerals are the major key's numerals, in scale order.
var ChordNumerals = []string{"I", "ii", "iii", "IV", "V", "vi", "viio"}

// HarmonyItemNumerals are the numerals the numeral drills ask about. The
// tonic is left out: its chord is the key's own name, so a question about it
// would answer itself.
var HarmonyItemNumerals = ChordNumerals[1:]

// NewHarmonyKeyOrder is the order new numeral items are introduced in,
// around the circle of fifths from C, every numeral of a key before the
// next key.
var NewHarmonyKeyOrder = []string{"C", "G", "F", "D", "Bb", "A", "Eb", "E", "Ab", "B", "Db", "F#"}

// EasyChartKeys are the keys a chart question is asked in while the
// student is still learning it.
var EasyChartKeys = []string{"C", "G", "D", "F", "A"}

// minorKeySuffix ends a minor key's name: a minor key is written as its tonic
// chord, Am.
const minorKeySuffix = "m"

// keyPitchClass is the pitch class of key's tonic, a major key of MusicKeys or
// a minor one of MusicKeysOf(ProgressionModeMinor); false for any other key.
func keyPitchClass(key string) (int, bool) {
	i := slices.Index(MusicKeys, strings.TrimSuffix(key, minorKeySuffix))
	return i, i >= 0
}

// KeyMode is the mode of key: minor for a key written as its minor tonic chord.
func KeyMode(key string) ProgressionMode {
	if strings.HasSuffix(key, minorKeySuffix) {
		return ProgressionModeMinor
	}
	return ProgressionModeMajor
}

// MusicKeysOf are the 12 keys a progression of mode is asked in: its tonic
// spelled as the chord catalog spells roots, in minor keys too.
func MusicKeysOf(mode ProgressionMode) []string {
	if mode != ProgressionModeMinor {
		return slices.Clone(MusicKeys)
	}
	minor := make([]string, len(MusicKeys))
	for i, tonic := range MusicKeys {
		minor[i] = tonic + minorKeySuffix
	}
	return minor
}

// EasyChartKeysOf are the keys a chart question of mode is asked in while the
// student is still learning it: the easy major keys, or their relative minors.
func EasyChartKeysOf(mode ProgressionMode) []string {
	return keysOf(mode, EasyChartKeys)
}

// TourKeyOrderOf is the order a tour of a progression in mode goes through:
// the cycle of fourths from C, or each of its keys' relative minors.
func TourKeyOrderOf(mode ProgressionMode) []string {
	return keysOf(mode, ProgressionKeyOrder)
}

// relativeMinorSemitones is how far above a major key's tonic its relative
// minor's is.
const relativeMinorSemitones = 9

// keysOf is majorKeys for a major progression, and their relative minors, in
// the same order, for a minor one.
func keysOf(mode ProgressionMode, majorKeys []string) []string {
	if mode != ProgressionModeMinor {
		return slices.Clone(majorKeys)
	}
	minor := make([]string, len(majorKeys))
	for i, key := range majorKeys {
		tonic, _ := keyPitchClass(key)
		minor[i] = MusicKeys[(tonic+relativeMinorSemitones)%12] + minorKeySuffix
	}
	return minor
}

// NumeralChord is the chord symbol of numeral in the major key, for any numeral
// a progression can hold: Em for vi, Bb for ♭VII, Dm7b5 for iiø7. Empty for a key
// or numeral it doesn't know.
func NumeralChord(key, numeral string) string {
	tonic, ok := keyPitchClass(key)
	if !ok {
		return ""
	}
	parsed, ok := ParseProgressionNumeral(numeral, ProgressionModeMajor)
	if !ok {
		return ""
	}
	return MusicKeys[(tonic+parsed.RootSemitones)%12] + parsed.Quality.CanonicalSuffix()
}

// NumeralFunction is numeral's function in a major key; empty for a
// numeral it doesn't know.
func NumeralFunction(numeral string) ChordFunction {
	return numeralFunctions[numeral]
}

// ProgressionChart is a progression's chart in key: the chord symbol of each
// bar's numeral.
func ProgressionChart(bars []string, key string) []string {
	chart := make([]string, len(bars))
	for i, numeral := range bars {
		chart[i] = NumeralChord(key, numeral)
	}
	return chart
}
