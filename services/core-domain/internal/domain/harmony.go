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

// diatonicTriad is a scale degree's triad in a major key: semitones above
// the tonic, the chord symbol's suffix and its function.
type diatonicTriad struct {
	semitones int
	suffix    string
	function  ChordFunction
}

// diatonicTriads are the major key's triads by numeral, viio being vii°.
var diatonicTriads = map[string]diatonicTriad{
	"I":    {0, "", ChordFunctionTonic},
	"ii":   {2, "m", ChordFunctionSubdominant},
	"iii":  {4, "m", ChordFunctionTonic},
	"IV":   {5, "", ChordFunctionSubdominant},
	"V":    {7, "", ChordFunctionDominant},
	"vi":   {9, "m", ChordFunctionTonic},
	"viio": {11, "dim", ChordFunctionDominant},
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

// keyPitchClass is the pitch class of key's tonic; false for a key that
// isn't one of MusicKeys.
func keyPitchClass(key string) (int, bool) {
	i := slices.Index(MusicKeys, key)
	return i, i >= 0
}

// NumeralChord is the chord symbol of numeral in the major key: its triad
// (Em, Bdim), or a dominant seventh for a progression's I7, IV7 or V7 (G7).
// Empty for a key or numeral it doesn't know.
func NumeralChord(key, numeral string) string {
	tonic, ok := keyPitchClass(key)
	if !ok {
		return ""
	}
	base, seventh := strings.CutSuffix(numeral, "7")
	triad, ok := diatonicTriads[base]
	if !ok {
		return ""
	}
	root := MusicKeys[(tonic+triad.semitones)%12]
	if seventh {
		return root + "7"
	}
	return root + triad.suffix
}

// NumeralFunction is numeral's function in a major key; empty for a
// numeral it doesn't know.
func NumeralFunction(numeral string) ChordFunction {
	return diatonicTriads[numeral].function
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
