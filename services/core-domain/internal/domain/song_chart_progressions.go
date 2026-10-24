package domain

import (
	"slices"
	"strings"
)

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

// numeralTriads is the triad each numeral of a major key is.
var numeralTriads = map[string]triad{"": triadMajor, "m": triadMinor, "dim": triadDiminished}

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
//   - the progression's numerals are spelled in the concert key, a dominant
//     seventh counting as major, and a numeral repeated in a row counts once;
//   - the concert key must be stated and major, as every progression of the
//     catalog is in a major key; otherwise every tag is unplayed.
//
// A tag missing from catalog is unplayed.
func UnplayedProgressions(doc SongChartDocument, concertKey *string, capoFret int, catalog []Progression) []SectionProgression {
	unplayed := []SectionProgression{}
	tonic, major := majorTonic(concertKey)
	for si, section := range doc.Sections {
		chords := sectionChords(section, capoFret)
		for _, key := range section.ProgressionKeys {
			i := slices.IndexFunc(catalog, func(p Progression) bool { return p.Key == key })
			if major && i >= 0 {
				if run := progressionChords(catalog[i].Bars, tonic); len(run) > 0 && containsRun(chords, run) {
					continue
				}
			}
			unplayed = append(unplayed, SectionProgression{SectionIndex: si, ProgressionKey: key})
		}
	}
	return unplayed
}

// majorTonic is the pitch class of a stated major concert key's tonic; false
// for a minor key or none.
func majorTonic(concertKey *string) (int, bool) {
	if concertKey == nil || strings.HasSuffix(*concertKey, "m") {
		return 0, false
	}
	_, pc, rest, ok := leadingNote(*concertKey)
	return pc, ok && rest == ""
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

// progressionChords is the progression's chords in the major key on tonic,
// with a numeral repeated in a row kept once.
func progressionChords(bars []string, tonic int) []heardChord {
	var chords []heardChord
	for _, numeral := range bars {
		base, _ := strings.CutSuffix(numeral, "7")
		t, ok := diatonicTriads[base]
		if !ok {
			return nil
		}
		chords = appendChanged(chords, heardChord{root: (tonic + t.semitones) % 12, triad: numeralTriads[t.suffix]})
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
