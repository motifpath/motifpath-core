package domain

import (
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
