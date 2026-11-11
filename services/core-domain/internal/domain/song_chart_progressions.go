package domain

import "slices"

// MaxSongChartProgressions is how many progressions a section can be tagged
// with.
const MaxSongChartProgressions = 10

// SectionProgression is a progression a chart's section is tagged with: the
// section by its index in document order, the progression by its key in the
// progression catalog.
type SectionProgression struct {
	SectionIndex   int
	ProgressionKey string
}

// SectionProgressions are every section's tags, in document order, then in
// tagged order.
func (d SongChartDocument) SectionProgressions() []SectionProgression {
	var tags []SectionProgression
	for si, section := range d.Sections {
		for _, key := range section.ProgressionKeys {
			tags = append(tags, SectionProgression{SectionIndex: si, ProgressionKey: key})
		}
	}
	return tags
}

// ProgressionKeysOf are the progressions of tags, each once, in the order
// they first appear: the progressions a chart plays.
func ProgressionKeysOf(tags []SectionProgression) []string {
	var keys []string
	for _, t := range tags {
		if !slices.Contains(keys, t.ProgressionKey) {
			keys = append(keys, t.ProgressionKey)
		}
	}
	return keys
}

// triad is the triad a chord is heard as: major, minor or diminished. A chord
// without one of them (a power chord, a suspended or augmented chord) has none.
type triad string

const (
	triadNone       triad = ""
	triadMajor      triad = "major"
	triadMinor      triad = "minor"
	triadDiminished triad = "diminished"
)

// triadOf maps each quality to its triad: a seventh or an extension keeps the
// triad under it, so G7 is heard as G and Am7 as Am.
var triadOf = map[ChordQuality]triad{
	ChordQualityMajor: triadMajor, ChordQualityMajor6: triadMajor, ChordQualityDominant7: triadMajor,
	ChordQualityMajor7: triadMajor, ChordQualityAdd9: triadMajor, ChordQualityDominant9: triadMajor,
	ChordQualityMajor9: triadMajor, ChordQualityDominant11: triadMajor, ChordQualityDominant13: triadMajor,
	ChordQualityDominant7Flat9: triadMajor, ChordQualityDominant7Sharp9: triadMajor,
	ChordQualityMinor: triadMinor, ChordQualityMinor6: triadMinor, ChordQualityMinor7: triadMinor,
	ChordQualityMinorMajor7: triadMinor, ChordQualityMinorAdd9: triadMinor, ChordQualityMinor9: triadMinor,
	ChordQualityMinor11:    triadMinor,
	ChordQualityDiminished: triadDiminished, ChordQualityDiminished7: triadDiminished, ChordQualityHalfDiminished7: triadDiminished,
}

// heardChord is a chord as it sounds: its root's pitch class and its triad.
// A chord with no triad, or one that can't be read, matches nothing.
type heardChord struct {
	root  int
	triad triad
}

// UnplayedProgressions lists the sections' tags their section doesn't play,
// by section, then in tagged order. A progression is played by a section when
// its chords appear in a row in that section, in the chart's concert key:
//   - the section's chords are taken in document order, as they sound: moved
//     up by the capo, each counted by its root and triad. N.C. is skipped, and
//     a chord repeated in a row counts once. Another section's chords never
//     count;
//   - the progression's numerals are spelled in the concert key when their modes
//     match and in its relative key when they don't, a dominant seventh counting
//     as major, and a numeral repeated in a row counts once;
//   - the concert key must be stated; otherwise every tag is unplayed.
//
// A tag missing from catalog is unplayed.
func UnplayedProgressions(doc SongChartDocument, concertKey *string, capoFret int, catalog []Progression) []SectionProgression {
	unplayed := []SectionProgression{}
	for si, section := range doc.Sections {
		chords := sectionChords(section, capoFret)
		for _, key := range section.ProgressionKeys {
			i := slices.IndexFunc(catalog, func(p Progression) bool { return p.Key == key })
			if i >= 0 {
				if run := progressionChords(catalog[i], concertKey); len(run) > 0 && containsRun(chords, run) {
					continue
				}
			}
			unplayed = append(unplayed, SectionProgression{SectionIndex: si, ProgressionKey: key})
		}
	}
	return unplayed
}

// spellingTonic is the pitch class a progression in mode is spelled on: the
// concert key's tonic when their modes match, its relative key's when they don't.
// False for a key that isn't stated or can't be read.
func spellingTonic(concertKey *string, mode ProgressionMode) (int, bool) {
	if concertKey == nil {
		return 0, false
	}
	_, tonic, rest, ok := leadingNote(*concertKey)
	if !ok {
		return 0, false
	}
	switch {
	case rest == "" && mode == ProgressionModeMajor, rest == "m" && mode == ProgressionModeMinor:
		return tonic, true
	case rest == "" && mode == ProgressionModeMinor:
		return (tonic + 9) % 12, true
	case rest == "m" && mode == ProgressionModeMajor:
		return (tonic + 3) % 12, true
	}
	return 0, false
}

// sectionChords is the section's chords as they sound, in document order,
// with N.C. skipped and a chord repeated in a row kept once.
func sectionChords(section SongChartSection, capoFret int) []heardChord {
	var chords []heardChord
	for _, a := range (SongChartDocument{Sections: []SongChartSection{section}}).Anchors() {
		reading := ParseChordSymbol(a.Anchor.WrittenSymbol)
		if reading.Status == ChordSymbolNoChord {
			continue
		}
		chord := heardChord{root: -1}
		if p := reading.Parsed; p != nil && triadOf[p.Quality] != triadNone {
			chord = heardChord{root: (p.RootPitchClass + capoFret) % 12, triad: triadOf[p.Quality]}
		}
		chords = appendChanged(chords, chord)
	}
	return chords
}

// progressionChords is the progression's chords as spelled for concertKey, with a
// numeral repeated in a row kept once; nil when it can't be spelled.
func progressionChords(p Progression, concertKey *string) []heardChord {
	mode := ProgressionMode(p.Mode)
	tonic, ok := spellingTonic(concertKey, mode)
	if !ok {
		return nil
	}
	var chords []heardChord
	for _, bar := range p.Bars {
		numeral, ok := ParseProgressionNumeral(bar, mode)
		if !ok {
			return nil
		}
		chords = appendChanged(chords, heardChord{root: (tonic + numeral.RootSemitones) % 12, triad: triadOf[numeral.Quality]})
	}
	return chords
}

// appendChanged appends c unless it repeats the last chord. A chord that
// can't be heard (root -1) never repeats, so it always breaks a run.
func appendChanged(chords []heardChord, c heardChord) []heardChord {
	if n := len(chords); n > 0 && c.root >= 0 && chords[n-1] == c {
		return chords
	}
	return append(chords, c)
}
