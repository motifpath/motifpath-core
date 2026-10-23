package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// chordsBody is a one-line chart whose runs carry the given chord symbols, in order.
func chordsBody(symbols string) SongChartDocument {
	var runs []SongChartRun
	for i, s := range strings.Fields(symbols) {
		runs = append(runs, SongChartRun{Text: "la ", Anchor: &ChordAnchor{ID: string(rune('a' + i)), WrittenSymbol: s}})
	}
	return SongChartDocument{Sections: []SongChartSection{{Kind: "verse", Lines: []SongChartLine{{Runs: runs}}}}}
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
			got := UnplayedProgressions(chordsBody(c.chords), c.concertKey, c.capo, c.tagged, progressionCatalog())
			assert.Equal(t, c.unplayed, got)
		})
	}
}
