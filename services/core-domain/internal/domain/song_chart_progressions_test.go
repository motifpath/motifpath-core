package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// chordsSection is a one-line section whose runs carry the given chord
// symbols, in order, tagged with tagged.
func chordsSection(kind SectionKind, symbols string, tagged ...string) SongChartSection {
	var runs []SongChartRun
	for i, s := range strings.Fields(symbols) {
		runs = append(runs, SongChartRun{Text: "la ", Anchor: &ChordAnchor{ID: string(kind) + string(rune('a'+i)), WrittenSymbol: s}})
	}
	return SongChartSection{Kind: kind, Lines: []SongChartLine{{Runs: runs}}, ProgressionKeys: tagged}
}

// chordsBody is a chart of one verse with the given chord symbols, tagged with tagged.
func chordsBody(symbols string, tagged ...string) SongChartDocument {
	return SongChartDocument{Sections: []SongChartSection{chordsSection(SectionVerse, symbols, tagged...)}}
}

// verse is the first section's unplayed progressions.
func verse(keys ...string) []SectionProgression {
	unplayed := []SectionProgression{}
	for _, k := range keys {
		unplayed = append(unplayed, SectionProgression{SectionIndex: 0, ProgressionKey: k})
	}
	return unplayed
}

func progressionCatalog() []Progression {
	return []Progression{
		{Key: "I-IV-V-I", Mode: "major", Bars: []string{"I", "IV", "V", "I"}},
		{Key: "I-IV-I-V", Mode: "major", Bars: []string{"I", "IV", "I", "V"}},
		{Key: "I-V-vi-IV", Mode: "major", Bars: []string{"I", "V", "vi", "IV"}},
		{Key: "vi-IV-I-V", Mode: "major", Bars: []string{"vi", "IV", "I", "V"}},
		{Key: "ii-V-I", Mode: "major", Bars: []string{"ii", "V", "I", "I"}},
		{Key: "12-bar-blues", Mode: "major", Bars: []string{"I7", "I7", "I7", "I7", "IV7", "IV7", "I7", "I7", "V7", "IV7", "I7", "V7"}},
	}
}

func TestUnplayedProgressions(t *testing.T) {
	key := func(k string) *string { return &k }
	cases := []struct {
		name       string
		concertKey *string
		capo       int
		chords     string
		tagged     []string
		unplayed   []string
	}{
		{"the chords in a row play the progression", key("G"), 0, "G C G D G", []string{"I-IV-I-V"}, []string{}},
		{"a tag the chart doesn't play is listed, in tagged order", key("G"), 0, "G C G D G", []string{"I-V-vi-IV", "I-IV-I-V", "vi-IV-I-V"}, []string{"I-V-vi-IV", "vi-IV-I-V"}},
		{"the chords must come in a row, not just in order", key("G"), 0, "G C G D G", []string{"I-IV-V-I"}, []string{"I-IV-V-I"}},
		{"the chords sound moved up by the capo", key("A"), 2, "G C G D", []string{"I-IV-I-V"}, []string{}},
		{"without the capo the same chords are in another key", key("A"), 0, "G C G D", []string{"I-IV-I-V"}, []string{"I-IV-I-V"}},
		{"a seventh or a slash bass doesn't change the chord", key("G"), 0, "G7 C D/F# G", []string{"I-IV-V-I"}, []string{}},
		{"an extension of a minor chord is still minor", key("C"), 0, "C G Am7/G F", []string{"I-V-vi-IV"}, []string{}},
		{"a chord repeated in a row counts once", key("G"), 0, "G G C C D D G", []string{"I-IV-V-I"}, []string{}},
		{"a numeral repeated in a row counts once", key("C"), 0, "Dm G C", []string{"ii-V-I"}, []string{}},
		{"N.C. is skipped", key("G"), 0, "G N.C. C D N.C. G", []string{"I-IV-V-I"}, []string{}},
		{"a chord repeated across an N.C. counts once", key("G"), 0, "G C N.C. C D G", []string{"I-IV-V-I"}, []string{}},
		{"a dominant seventh numeral counts as major", key("A"), 0, "A7 D7 A7 E7 D7 A7 E7", []string{"12-bar-blues"}, []string{}},
		{"keys are compared by pitch, whatever their spelling", key("Gb"), 0, "F# B F# C#", []string{"I-IV-I-V"}, []string{}},
		{"a chord with no triad breaks the run", key("G"), 0, "G Csus4 D G", []string{"I-IV-V-I"}, []string{"I-IV-V-I"}},
		{"an unreadable chord breaks the run", key("G"), 0, "G H7 C D G", []string{"I-IV-V-I"}, []string{"I-IV-V-I"}},
		{"a chart in a minor key plays no progression", key("Em"), 0, "Em C G D", []string{"vi-IV-I-V"}, []string{"vi-IV-I-V"}},
		{"a chart with no concert key plays no progression", nil, 0, "G C G D G", []string{"I-IV-I-V"}, []string{"I-IV-I-V"}},
		{"an untagged chart lists nothing", key("G"), 0, "G C G D G", nil, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UnplayedProgressions(chordsBody(c.chords, c.tagged...), c.concertKey, c.capo, progressionCatalog())
			assert.Equal(t, verse(c.unplayed...), got)
		})
	}
}

func TestUnplayedProgressions_BySection(t *testing.T) {
	g := "G"
	cases := []struct {
		name     string
		sections []SongChartSection
		unplayed []SectionProgression
	}{
		{
			name:     "another section's chords never play a section's progression",
			sections: []SongChartSection{chordsSection(SectionVerse, "G C", "I-IV-I-V"), chordsSection(SectionChorus, "G D")},
			unplayed: []SectionProgression{{SectionIndex: 0, ProgressionKey: "I-IV-I-V"}},
		},
		{
			name:     "each section plays its own tags",
			sections: []SongChartSection{chordsSection(SectionVerse, "G C G D", "I-IV-I-V"), chordsSection(SectionChorus, "G D Em C", "I-V-vi-IV")},
			unplayed: []SectionProgression{},
		},
		{
			name: "unplayed tags are listed by section, then in tagged order",
			sections: []SongChartSection{
				chordsSection(SectionVerse, "G C G D", "I-V-vi-IV", "I-IV-I-V", "ii-V-I"),
				chordsSection(SectionChorus, "G D Em C", "I-IV-V-I"),
			},
			unplayed: []SectionProgression{
				{SectionIndex: 0, ProgressionKey: "I-V-vi-IV"}, {SectionIndex: 0, ProgressionKey: "ii-V-I"},
				{SectionIndex: 1, ProgressionKey: "I-IV-V-I"},
			},
		},
		{
			name:     "a tag the catalog doesn't have is unplayed",
			sections: []SongChartSection{chordsSection(SectionVerse, "G C G D", "I-II-III")},
			unplayed: []SectionProgression{{SectionIndex: 0, ProgressionKey: "I-II-III"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UnplayedProgressions(SongChartDocument{Sections: c.sections}, &g, 0, progressionCatalog())
			assert.Equal(t, c.unplayed, got)
		})
	}
}

func TestSongChartDocument_SectionProgressions(t *testing.T) {
	doc := SongChartDocument{Sections: []SongChartSection{
		chordsSection(SectionIntro, "G"),
		chordsSection(SectionVerse, "G C G D", "I-IV-I-V", "I-IV-V-I"),
		chordsSection(SectionChorus, "G D Em C", "I-V-vi-IV", "I-IV-I-V"),
	}}
	assert.Equal(t, []SectionProgression{
		{SectionIndex: 1, ProgressionKey: "I-IV-I-V"}, {SectionIndex: 1, ProgressionKey: "I-IV-V-I"},
		{SectionIndex: 2, ProgressionKey: "I-V-vi-IV"}, {SectionIndex: 2, ProgressionKey: "I-IV-I-V"},
	}, doc.SectionProgressions())
	assert.Equal(t, []string{"I-IV-I-V", "I-IV-V-I", "I-V-vi-IV"}, ProgressionKeysOf(doc.SectionProgressions()),
		"the chart's progressions are its sections' tags, each once, in document order")
	assert.Empty(t, SongChartDocument{Sections: []SongChartSection{chordsSection(SectionVerse, "G")}}.SectionProgressions())
}
