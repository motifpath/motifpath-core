package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestParseChordSymbol(t *testing.T) {
	pc := func(n int) *int { return &n }
	note := func(s string) *string { return &s }

	parsed := []struct {
		name  string
		input string
		want  domain.ParsedChordSymbol
	}{
		{"a major chord has no suffix", "C", domain.ParsedChordSymbol{Root: "C", RootPitchClass: 0, Quality: domain.ChordQualityMajor, CanonicalSymbol: "C"}},
		{"aliases of one quality share its canonical suffix", "BbΔ7", domain.ParsedChordSymbol{Root: "Bb", RootPitchClass: 10, Quality: domain.ChordQualityMajor7, CanonicalSymbol: "Bbmaj7"}},
		{"unicode accidentals read as ASCII ones", "F♯ø", domain.ParsedChordSymbol{Root: "F#", RootPitchClass: 6, Quality: domain.ChordQualityHalfDiminished7, CanonicalSymbol: "F#m7b5"}},
		{"matching is case-sensitive", "CM7", domain.ParsedChordSymbol{Root: "C", RootPitchClass: 0, Quality: domain.ChordQualityMajor7, CanonicalSymbol: "Cmaj7"}},
		{"an accidental after the letter belongs to the root", "Cb5", domain.ParsedChordSymbol{Root: "Cb", RootPitchClass: 11, Quality: domain.ChordQualityPower, CanonicalSymbol: "Cb5"}},
		{"a slash bass is kept as written", "D/F♯", domain.ParsedChordSymbol{Root: "D", RootPitchClass: 2, Quality: domain.ChordQualityMajor, Bass: note("F#"), BassPitchClass: pc(6), CanonicalSymbol: "D/F#"}},
		{"a bass equal to the root is dropped", "Am/A", domain.ParsedChordSymbol{Root: "A", RootPitchClass: 9, Quality: domain.ChordQualityMinor, CanonicalSymbol: "Am"}},
		{"suffix accidentals read as ASCII ones", "B♭7♭9", domain.ParsedChordSymbol{Root: "Bb", RootPitchClass: 10, Quality: domain.ChordQualityDominant7Flat9, CanonicalSymbol: "Bb7b9"}},
	}
	for _, tt := range parsed {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.ParseChordSymbol(tt.input)

			require.Equal(t, domain.ChordSymbolParsed, got.Status)
			require.NotNil(t, got.Parsed)
			assert.Equal(t, tt.want, *got.Parsed)
		})
	}

	notChords := []struct {
		name    string
		input   string
		status  domain.ChordSymbolStatus
		warning domain.ChordSymbolWarning
	}{
		{"N.C. is no chord", "N.C.", domain.ChordSymbolNoChord, ""},
		{"NC is no chord", "NC", domain.ChordSymbolNoChord, ""},
		{"a lowercase root is not a chord", "cm", domain.ChordSymbolUnparsed, domain.ChordSymbolWarningUnparsed},
		{"whitespace is never trimmed", " C", domain.ChordSymbolUnparsed, domain.ChordSymbolWarningUnparsed},
		{"a bass must be a note", "C/Gm", domain.ChordSymbolUnparsed, domain.ChordSymbolWarningUnparsed},
		{"an unsupported quality is not reinterpreted", "C7#11", domain.ChordSymbolUnparsed, domain.ChordSymbolWarningUnsupportedQuality},
		{"o is not a diminished alias", "Do", domain.ChordSymbolUnparsed, domain.ChordSymbolWarningUnsupportedQuality},
	}
	for _, tt := range notChords {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.ParseChordSymbol(tt.input)

			assert.Equal(t, tt.status, got.Status)
			assert.Equal(t, tt.warning, got.Warning)
			assert.Nil(t, got.Parsed)
		})
	}
}
