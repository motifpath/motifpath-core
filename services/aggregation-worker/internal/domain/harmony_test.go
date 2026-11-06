package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every numeral of the grammar, read in C, spelled out by hand: core keeps its own
// reader, so this table is what stops the two from drifting.
func TestChordOf_ReadsTheWholeNumeralGrammar(t *testing.T) {
	roots := []struct {
		upper, lower string
		natural      string
		flat, sharp  string
	}{
		{"I", "i", "C", "B", "Db"},
		{"II", "ii", "D", "Db", "Eb"},
		{"III", "iii", "E", "Eb", "F"},
		{"IV", "iv", "F", "E", "F#"},
		{"V", "v", "G", "F#", "Ab"},
		{"VI", "vi", "A", "Ab", "Bb"},
		{"VII", "vii", "B", "Bb", "C"},
	}
	accidentals := []struct {
		symbol string
		root   func(natural, flat, sharp string) string
	}{
		{"", func(natural, _, _ string) string { return natural }},
		{"♭", func(_, flat, _ string) string { return flat }},
		{"♯", func(_, _, sharp string) string { return sharp }},
	}
	upperSuffixes := map[string]string{"": "", "7": "7", "maj7": "maj7"}
	lowerSuffixes := map[string]string{"": "m", "7": "m7", "°": "dim", "ø7": "m7b5", "°7": "dim7"}

	for _, accidental := range accidentals {
		for _, r := range roots {
			root := accidental.root(r.natural, r.flat, r.sharp)
			for suffix, symbol := range upperSuffixes {
				numeral := accidental.symbol + r.upper + suffix
				t.Run(numeral, func(t *testing.T) {
					c, ok := chordOf(0, numeral)
					require.True(t, ok)
					assert.Equal(t, root+symbol, c.symbol())
				})
			}
			for suffix, symbol := range lowerSuffixes {
				numeral := accidental.symbol + r.lower + suffix
				t.Run(numeral, func(t *testing.T) {
					c, ok := chordOf(0, numeral)
					require.True(t, ok)
					assert.Equal(t, root+symbol, c.symbol())
				})
			}
		}
	}
}

func TestChordOf_ReadsTheCatalogsSpellingOfTheDiminishedTriad(t *testing.T) {
	c, ok := chordOf(0, "viio")
	require.True(t, ok)
	assert.Equal(t, "Bdim", c.symbol())
}

func TestChordOf_MovesWithTheKey(t *testing.T) {
	c, ok := chordOf(7, "♭VII") // in G
	require.True(t, ok)
	assert.Equal(t, "F", c.symbol())
}

func TestChordOf_RefusesWhatIsNoNumeral(t *testing.T) {
	for _, numeral := range []string{"", "VIII", "Vm", "V°", "imaj7", "♭", "bVII", "#VI", "X"} {
		t.Run(numeral, func(t *testing.T) {
			_, ok := chordOf(0, numeral)
			assert.False(t, ok)
		})
	}
}
