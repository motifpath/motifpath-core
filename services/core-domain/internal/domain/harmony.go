package domain

import "slices"

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

// keyPitchClass is the pitch class of key's tonic; false for a key that
// isn't one of MusicKeys.
func keyPitchClass(key string) (int, bool) {
	i := slices.Index(MusicKeys, key)
	return i, i >= 0
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
