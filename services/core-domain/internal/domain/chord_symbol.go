package domain

import (
	"strings"
	"unicode"
)

// ChordQuality is the kind of chord, independent of its root. Each quality
// has one formula and one canonical suffix in a chord symbol.
type ChordQuality string

const (
	ChordQualityMajor           ChordQuality = "major"
	ChordQualityMinor           ChordQuality = "minor"
	ChordQualityPower           ChordQuality = "power"
	ChordQualityDiminished      ChordQuality = "diminished"
	ChordQualityAugmented       ChordQuality = "augmented"
	ChordQualitySus2            ChordQuality = "sus2"
	ChordQualitySus4            ChordQuality = "sus4"
	ChordQualityMajor6          ChordQuality = "major_6"
	ChordQualityMinor6          ChordQuality = "minor_6"
	ChordQualityDominant7       ChordQuality = "dominant_7"
	ChordQualityMajor7          ChordQuality = "major_7"
	ChordQualityMinor7          ChordQuality = "minor_7"
	ChordQualityMinorMajor7     ChordQuality = "minor_major_7"
	ChordQualityHalfDiminished7 ChordQuality = "half_diminished_7"
	ChordQualityDiminished7     ChordQuality = "diminished_7"
	ChordQualityDominant7Sus4   ChordQuality = "dominant_7_sus4"
	ChordQualityAdd9            ChordQuality = "add_9"
	ChordQualityMinorAdd9       ChordQuality = "minor_add_9"
	ChordQualityDominant9       ChordQuality = "dominant_9"
	ChordQualityMajor9          ChordQuality = "major_9"
	ChordQualityMinor9          ChordQuality = "minor_9"
	ChordQualityDominant11      ChordQuality = "dominant_11"
	ChordQualityMinor11         ChordQuality = "minor_11"
	ChordQualityDominant13      ChordQuality = "dominant_13"
	ChordQualityDominant7Flat5  ChordQuality = "dominant_7_flat_5"
	ChordQualityDominant7Sharp5 ChordQuality = "dominant_7_sharp_5"
	ChordQualityDominant7Flat9  ChordQuality = "dominant_7_flat_9"
	ChordQualityDominant7Sharp9 ChordQuality = "dominant_7_sharp_9"
)

// chordSuffixes lists every quality's suffixes, the canonical one first. A
// suffix is matched exactly and case-sensitively, after ♭ and ♯ are read as b
// and #. "o" and "o7" are deliberately absent: in Brazilian Portuguese "Do" is
// the note C, and reading it as D diminished would show a learner the wrong
// chord.
var chordSuffixes = []struct {
	quality  ChordQuality
	suffixes []string
}{
	{ChordQualityMajor, []string{"", "M", "maj"}},
	{ChordQualityMinor, []string{"m", "min", "-"}},
	{ChordQualityPower, []string{"5"}},
	{ChordQualityDiminished, []string{"dim", "°"}},
	{ChordQualityAugmented, []string{"aug", "+"}},
	{ChordQualitySus2, []string{"sus2"}},
	{ChordQualitySus4, []string{"sus4", "sus"}},
	{ChordQualityMajor6, []string{"6", "M6", "maj6"}},
	{ChordQualityMinor6, []string{"m6", "min6", "-6"}},
	{ChordQualityDominant7, []string{"7"}},
	{ChordQualityMajor7, []string{"maj7", "M7", "Δ7", "Δ", "ma7"}},
	{ChordQualityMinor7, []string{"m7", "min7", "-7"}},
	{ChordQualityMinorMajor7, []string{"mMaj7", "mM7", "m(maj7)", "minMaj7", "mΔ7", "-Δ7"}},
	{ChordQualityHalfDiminished7, []string{"m7b5", "m7(b5)", "-7b5", "ø", "ø7"}},
	{ChordQualityDiminished7, []string{"dim7", "°7"}},
	{ChordQualityDominant7Sus4, []string{"7sus4", "7sus"}},
	{ChordQualityAdd9, []string{"add9", "(add9)"}},
	{ChordQualityMinorAdd9, []string{"madd9", "m(add9)"}},
	{ChordQualityDominant9, []string{"9"}},
	{ChordQualityMajor9, []string{"maj9", "M9", "Δ9"}},
	{ChordQualityMinor9, []string{"m9", "min9", "-9"}},
	{ChordQualityDominant11, []string{"11"}},
	{ChordQualityMinor11, []string{"m11", "min11", "-11"}},
	{ChordQualityDominant13, []string{"13"}},
	{ChordQualityDominant7Flat5, []string{"7b5", "7(b5)"}},
	{ChordQualityDominant7Sharp5, []string{"7#5", "7(#5)", "7+5", "+7", "aug7"}},
	{ChordQualityDominant7Flat9, []string{"7b9", "7(b9)"}},
	{ChordQualityDominant7Sharp9, []string{"7#9", "7(#9)"}},
}

// qualityBySuffix and canonicalSuffix index chordSuffixes both ways.
var (
	qualityBySuffix = map[string]ChordQuality{}
	canonicalSuffix = map[ChordQuality]string{}
)

func init() {
	for _, q := range chordSuffixes {
		canonicalSuffix[q.quality] = q.suffixes[0]
		for _, s := range q.suffixes {
			qualityBySuffix[s] = q.quality
		}
	}
}

// Valid reports whether q is a chord quality this service knows.
func (q ChordQuality) Valid() bool {
	_, ok := canonicalSuffix[q]
	return ok
}

// CanonicalSuffix is what follows the root in q's canonical chord symbol:
// "" for major, "m7b5" for half-diminished seventh.
func (q ChordQuality) CanonicalSuffix() string {
	return canonicalSuffix[q]
}

// ChordSymbolStatus says whether a chord symbol names a chord.
type ChordSymbolStatus string

const (
	ChordSymbolParsed   ChordSymbolStatus = "parsed"
	ChordSymbolUnparsed ChordSymbolStatus = "unparsed"
	ChordSymbolNoChord  ChordSymbolStatus = "no_chord"
)

// ChordSymbolWarning says why a symbol didn't parse.
type ChordSymbolWarning string

const (
	// ChordSymbolWarningUnparsed: no root, a malformed bass, or whitespace.
	ChordSymbolWarningUnparsed ChordSymbolWarning = "unparsed_symbol"
	// ChordSymbolWarningUnsupportedQuality: the root reads as a note, but
	// what follows it is none of the supported qualities.
	ChordSymbolWarningUnsupportedQuality ChordSymbolWarning = "unsupported_quality"
)

// ParsedChordSymbol is what a chord symbol means. Root and Bass are spelled
// as written, in ASCII; Bass and BassPitchClass are nil together, also when
// the written bass was the root itself.
type ParsedChordSymbol struct {
	Root            string
	RootPitchClass  int
	Quality         ChordQuality
	Bass            *string
	BassPitchClass  *int
	CanonicalSymbol string
}

// ChordSymbolReading is the result of reading one chord symbol: Parsed is
// set exactly when Status is ChordSymbolParsed, Warning exactly when it is
// ChordSymbolUnparsed.
type ChordSymbolReading struct {
	Status  ChordSymbolStatus
	Parsed  *ParsedChordSymbol
	Warning ChordSymbolWarning
}

var letterPitchClass = map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}

// ParseChordSymbol reads raw exactly as written: nothing is trimmed, and a
// symbol that isn't a supported chord is reported, never reinterpreted as
// something close to it.
func ParseChordSymbol(raw string) ChordSymbolReading {
	if raw == "N.C." || raw == "NC" {
		return ChordSymbolReading{Status: ChordSymbolNoChord}
	}
	unparsed := func(w ChordSymbolWarning) ChordSymbolReading {
		return ChordSymbolReading{Status: ChordSymbolUnparsed, Warning: w}
	}
	if strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return unparsed(ChordSymbolWarningUnparsed)
	}
	text := strings.NewReplacer("♭", "b", "♯", "#").Replace(raw)

	root, rootPC, rest, ok := leadingNote(text)
	if !ok {
		return unparsed(ChordSymbolWarningUnparsed)
	}
	var bass *string
	var bassPC *int
	if suffix, written, found := strings.Cut(rest, "/"); found {
		note, pc, after, ok := leadingNote(written)
		if !ok || after != "" {
			return unparsed(ChordSymbolWarningUnparsed)
		}
		rest = suffix
		if pc != rootPC {
			bass, bassPC = &note, &pc
		}
	}
	quality, ok := qualityBySuffix[rest]
	if !ok {
		return unparsed(ChordSymbolWarningUnsupportedQuality)
	}
	canonical := root + quality.CanonicalSuffix()
	if bass != nil {
		canonical += "/" + *bass
	}
	return ChordSymbolReading{Status: ChordSymbolParsed, Parsed: &ParsedChordSymbol{
		Root: root, RootPitchClass: rootPC, Quality: quality, Bass: bass, BassPitchClass: bassPC, CanonicalSymbol: canonical,
	}}
}

// leadingNote reads an uppercase letter A–G and at most one accidental from
// the start of text, returning the note, its pitch class and what follows.
func leadingNote(text string) (note string, pitchClass int, rest string, ok bool) {
	if text == "" {
		return "", 0, "", false
	}
	pc, isLetter := letterPitchClass[text[0]]
	if !isLetter {
		return "", 0, "", false
	}
	end := 1
	if len(text) > 1 {
		switch text[1] {
		case 'b':
			pc, end = pc+11, 2
		case '#':
			pc, end = pc+1, 2
		}
	}
	return text[:end], pc % 12, text[end:], true
}
