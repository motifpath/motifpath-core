package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgression_BorrowedBars(t *testing.T) {
	tests := []struct {
		name string
		mode string
		bars []string
		want []int
	}{
		{"all diatonic in major", "major", []string{"I", "V", "vi", "IV"}, []int{}},
		{"a seventh doesn't change the triad", "major", []string{"I7", "IV7", "V7", "I7"}, []int{}},
		{"bars outside the mode, numbered from 0", "major", []string{"I", "♭VII", "IV", "iv"}, []int{1, 3}},
		{"the same chord read in the other mode", "minor", []string{"i", "♭VII", "iv", "♭II"}, []int{3}},
		{"the catalog's ASCII spelling reads as a numeral", "major", []string{"viio", "I"}, []int{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Progression{Mode: tc.mode, Bars: tc.bars}.BorrowedBars()

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNewCreatedProgression(t *testing.T) {
	languages := []string{"en", "pt_BR"}

	t.Run("the key spells the numerals in ASCII, the bars keep the symbols", func(t *testing.T) {
		p, err := NewCreatedProgression("minor", []string{"i", "#ivo7", "viih7", "bVImaj7"}, nil, languages)

		assert.NoError(t, err)
		assert.Equal(t, "minor/i-#ivo7-viih7-bVImaj7", p.Key)
		assert.Equal(t, []string{"i", "♯iv°7", "viiø7", "♭VImaj7"}, p.Bars)
		assert.Equal(t, LocalizedText{"en": "i–♯iv°7–viiø7–♭VImaj7", "pt_BR": "i–♯iv°7–viiø7–♭VImaj7"}, p.Names)
		assert.False(t, p.InDrills)
	})

	failures := []struct {
		name      string
		mode      string
		bars      []string
		nicknames map[string]string
		field     string
	}{
		{"an unknown mode", "dorian", []string{"I"}, nil, "mode"},
		{"no bars", "major", nil, nil, "bars"},
		{"seventeen bars", "major", make([]string, 17), nil, "bars"},
		{"a bar that isn't a chord", "major", []string{"I", "V9"}, nil, "bars/1"},
		{"a nickname in no language", "major", []string{"I"}, map[string]string{"xx": "rock"}, "nicknames/xx"},
		{"a blank nickname", "major", []string{"I"}, map[string]string{"en": "  "}, "nicknames/en"},
		{"a nickname too long", "major", []string{"I"}, map[string]string{"en": strings.Repeat("a", 61)}, "nicknames/en"},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewCreatedProgression(tc.mode, tc.bars, tc.nicknames, languages)

			var invalid *ValidationError
			if assert.ErrorAs(t, err, &invalid) {
				assert.Equal(t, tc.field, invalid.Fields[0].Field)
			}
		})
	}
}

func TestUnplayedProgressions_ByMode(t *testing.T) {
	key := "D"
	section := func(symbols ...string) SongChartDocument {
		var line []SongChartRun
		for i, s := range symbols {
			line = append(line, SongChartRun{Anchor: &ChordAnchor{ID: string(rune('a' + i)), WrittenSymbol: s}})
		}
		return SongChartDocument{Sections: []SongChartSection{{ProgressionKeys: []string{"p"}, Lines: []SongChartLine{{Runs: line}}}}}
	}
	borrowed := []Progression{{Key: "p", Mode: "major", Bars: []string{"I", "♭VII", "IV", "I"}}}
	minor := []Progression{{Key: "p", Mode: "minor", Bars: []string{"I", "♭VII", "IV", "I"}}}

	assert.Empty(t, UnplayedProgressions(section("D", "C", "G", "D"), &key, 0, borrowed), "a borrowed chord is spelled from its numeral")
	assert.Len(t, UnplayedProgressions(section("D", "C#dim", "G", "D"), &key, 0, borrowed), 1, "and must sound as written")
	assert.Len(t, UnplayedProgressions(section("D", "C", "G", "D"), &key, 0, minor), 1, "a minor progression isn't spelled in a major chart yet")
}
