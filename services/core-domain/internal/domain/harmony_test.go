package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestNumeralChord(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		numeral string
		want    string
	}{
		{name: "a minor chord on vi", key: "G", numeral: "vi", want: "Em"},
		{name: "a major chord on V", key: "D", numeral: "V", want: "A"},
		{name: "the diminished chord on viio", key: "C", numeral: "viio", want: "Bdim"},
		{name: "the tonic is the key's own name", key: "Eb", numeral: "I", want: "Eb"},
		{name: "a root spelled as the chord catalog spells it, even against the key", key: "Db", numeral: "IV", want: "F#"},
		{name: "a sharp key's minor chord spelled with flats as the catalog does", key: "F#", numeral: "iii", want: "Bbm"},
		{name: "a dominant seventh on a progression's V7", key: "A", numeral: "V7", want: "E7"},
		{name: "a dominant seventh on I7", key: "G", numeral: "I7", want: "G7"},
		{name: "a borrowed flat root is spelled as the catalog spells roots", key: "C", numeral: "♭VII", want: "Bb"},
		{name: "a borrowed chord moves with the key", key: "G", numeral: "♭VII", want: "F"},
		{name: "a sharp root that sounds like its flat twin spells the same chord", key: "C", numeral: "♯VI", want: "Bb"},
		{name: "a borrowed minor chord", key: "C", numeral: "iv", want: "Fm"},
		{name: "a written major seventh", key: "C", numeral: "Imaj7", want: "Cmaj7"},
		{name: "a written minor seventh", key: "C", numeral: "ii7", want: "Dm7"},
		{name: "a written half-diminished seventh", key: "C", numeral: "iiø7", want: "Dm7b5"},
		{name: "a written diminished seventh", key: "C", numeral: "vii°7", want: "Bdim7"},
		{name: "the stored diminished triad", key: "C", numeral: "vii°", want: "Bdim"},
		{name: "a borrowed dominant seventh", key: "C", numeral: "♭VII7", want: "Bb7"},
		{name: "a flat root below the tonic wraps round the octave", key: "C", numeral: "♭I", want: "B"},
		{name: "an unknown key has no chord", key: "H", numeral: "I", want: ""},
		{name: "an unknown numeral has no chord", key: "C", numeral: "VIII", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.NumeralChord(tt.key, tt.numeral))
		})
	}
}

func TestNumeralFunction(t *testing.T) {
	tests := []struct {
		numeral string
		want    domain.ChordFunction
	}{
		{"I", domain.ChordFunctionTonic},
		{"iii", domain.ChordFunctionTonic},
		{"vi", domain.ChordFunctionTonic},
		{"ii", domain.ChordFunctionSubdominant},
		{"IV", domain.ChordFunctionSubdominant},
		{"V", domain.ChordFunctionDominant},
		{"viio", domain.ChordFunctionDominant},
	}
	for _, tt := range tests {
		t.Run(tt.numeral, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.NumeralFunction(tt.numeral))
		})
	}
}

func TestProgressionChart(t *testing.T) {
	tests := []struct {
		name string
		bars []string
		key  string
		want []string
	}{
		{name: "the pop progression in G", bars: []string{"I", "V", "vi", "IV"}, key: "G", want: []string{"G", "D", "Em", "C"}},
		{name: "a ii–V–I holds its last bar", bars: []string{"ii", "V", "I", "I"}, key: "Bb", want: []string{"Cm", "F", "Bb", "Bb"}},
		{
			name: "the 12-bar blues in dominant sevenths",
			bars: []string{"I7", "I7", "I7", "I7", "IV7", "IV7", "I7", "I7", "V7", "IV7", "I7", "V7"},
			key:  "A",
			want: []string{"A7", "A7", "A7", "A7", "D7", "D7", "A7", "A7", "E7", "D7", "A7", "E7"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.ProgressionChart(tt.bars, tt.key))
		})
	}
}

func TestHarmonyKeyOrders(t *testing.T) {
	t.Run("the numeral items leave out the tonic, whose chord is the key's own name", func(t *testing.T) {
		assert.Equal(t, []string{"ii", "iii", "IV", "V", "vi", "viio"}, domain.HarmonyItemNumerals)
	})
	t.Run("new keys come around the circle of fifths from C", func(t *testing.T) {
		assert.Equal(t, []string{"C", "G", "F", "D", "Bb", "A", "Eb", "E", "Ab", "B", "Db", "F#"}, domain.NewHarmonyKeyOrder)
		assert.ElementsMatch(t, domain.MusicKeys, domain.NewHarmonyKeyOrder)
	})
	t.Run("a chart is asked in an easy key while it is being learnt", func(t *testing.T) {
		assert.Equal(t, []string{"C", "G", "D", "F", "A"}, domain.EasyChartKeys)
	})
}
