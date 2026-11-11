package domain

import (
	"slices"
	"strings"
)

// ProgressionMode is the mode a progression's numerals are read in.
type ProgressionMode string

const (
	ProgressionModeMajor ProgressionMode = "major"
	ProgressionModeMinor ProgressionMode = "minor"
)

// ProgressionNumeral is one bar of a progression, read in its mode.
type ProgressionNumeral struct {
	// Numeral is the stored form, with ♭, ♯, ° and ø in place of b, #, o and h.
	Numeral string
	// RootSemitones is how far the root stands above the tonic, in either mode.
	RootSemitones int
	Quality       ChordQuality
	// Borrowed marks a chord whose root and triad are outside the mode. A
	// seventh doesn't change the triad, so V7 counts as V.
	Borrowed bool
}

// numeralRoots lists each root with its degree in the major scale, longer
// spellings before the ones they start with.
var numeralRoots = []struct {
	upper, lower string
	semitones    int
}{
	{"VII", "vii", 11},
	{"III", "iii", 4},
	{"VI", "vi", 9},
	{"IV", "iv", 5},
	{"II", "ii", 2},
	{"V", "v", 7},
	{"I", "i", 0},
}

// numeralSuffix is how a suffix is stored and the chord quality it gives.
type numeralSuffix struct {
	stored  string
	quality ChordQuality
}

var upperCaseNumeralSuffixes = map[string]numeralSuffix{
	"":     {"", ChordQualityMajor},
	"7":    {"7", ChordQualityDominant7},
	"maj7": {"maj7", ChordQualityMajor7},
}

var lowerCaseNumeralSuffixes = map[string]numeralSuffix{
	"":   {"", ChordQualityMinor},
	"7":  {"7", ChordQualityMinor7},
	"°":  {"°", ChordQualityDiminished},
	"o":  {"°", ChordQualityDiminished},
	"ø7": {"ø7", ChordQualityHalfDiminished7},
	"h7": {"ø7", ChordQualityHalfDiminished7},
	"°7": {"°7", ChordQualityDiminished7},
	"o7": {"°7", ChordQualityDiminished7},
}

// diatonicChord is a chord of a mode: its root above the tonic and its triad.
type diatonicChord struct {
	semitones int
	triad     triad
}

// diatonicChords are each mode's chords. Minor includes harmonic minor's V and
// vii° alongside natural minor's v.
var diatonicChords = map[ProgressionMode][]diatonicChord{
	ProgressionModeMajor: {
		{0, triadMajor}, {2, triadMinor}, {4, triadMinor}, {5, triadMajor},
		{7, triadMajor}, {9, triadMinor}, {11, triadDiminished},
	},
	ProgressionModeMinor: {
		{0, triadMinor}, {2, triadDiminished}, {3, triadMajor}, {5, triadMinor},
		{7, triadMinor}, {8, triadMajor}, {10, triadMajor},
		{7, triadMajor}, {11, triadDiminished},
	},
}

// ParseProgressionNumeral reads input in mode exactly as sent: nothing is
// trimmed, and any other character makes it no numeral. False for an input that
// isn't a numeral or a mode that isn't one.
func ParseProgressionNumeral(input string, mode ProgressionMode) (ProgressionNumeral, bool) {
	diatonic, ok := diatonicChords[mode]
	if !ok {
		return ProgressionNumeral{}, false
	}

	rest, accidental, shift := cutNumeralAccidental(input)
	for _, root := range numeralRoots {
		suffixes := upperCaseNumeralSuffixes
		spelled := root.upper
		after, found := strings.CutPrefix(rest, root.upper)
		if !found {
			suffixes = lowerCaseNumeralSuffixes
			spelled = root.lower
			if after, found = strings.CutPrefix(rest, root.lower); !found {
				continue
			}
		}
		suffix, ok := suffixes[after]
		if !ok {
			return ProgressionNumeral{}, false
		}
		semitones := (root.semitones + shift + 12) % 12
		chord := diatonicChord{semitones: semitones, triad: triadOf[suffix.quality]}
		return ProgressionNumeral{
			Numeral:       accidental + spelled + suffix.stored,
			RootSemitones: semitones,
			Quality:       suffix.quality,
			Borrowed:      !slices.Contains(diatonic, chord),
		}, true
	}
	return ProgressionNumeral{}, false
}

// cutNumeralAccidental takes the leading accidental off input, returning the
// rest, its stored symbol and how many semitones it moves the root.
func cutNumeralAccidental(input string) (rest, accidental string, shift int) {
	for _, flat := range []string{"♭", "b"} {
		if after, ok := strings.CutPrefix(input, flat); ok {
			return after, "♭", -1
		}
	}
	for _, sharp := range []string{"♯", "#"} {
		if after, ok := strings.CutPrefix(input, sharp); ok {
			return after, "♯", 1
		}
	}
	return input, "", 0
}
