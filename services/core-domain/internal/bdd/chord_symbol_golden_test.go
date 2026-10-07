//go:build integration

package bdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// chordSymbolGoldenFile is golden/chord-symbols/chord_symbol.v1.json. The web
// client's chart editor runs the same cases, so an author never sees a chord
// the server reads differently.
type chordSymbolGoldenFile struct {
	Parser string `json:"parser"`
	Cases  []struct {
		Name     string `json:"name"`
		Input    string `json:"input"`
		Expected struct {
			Status  string `json:"status"`
			Warning string `json:"warning"`
			Parsed  *struct {
				Root            string  `json:"root"`
				RootPitchClass  int     `json:"root_pitch_class"`
				Quality         string  `json:"quality"`
				Bass            *string `json:"bass"`
				BassPitchClass  *int    `json:"bass_pitch_class"`
				CanonicalSymbol string  `json:"canonical_symbol"`
			} `json:"parsed"`
		} `json:"expected"`
	} `json:"cases"`
}

func TestChordSymbolGoldenCases(t *testing.T) {
	specsDir := strings.TrimSuffix(featuresBase, "/features")
	raw, err := os.ReadFile(filepath.Join(specsDir, "golden", "chord-symbols", "chord_symbol.v1.json"))
	require.NoError(t, err, "the chord symbol parser needs its golden cases in motifpath-specs")
	var golden chordSymbolGoldenFile
	require.NoError(t, json.Unmarshal(raw, &golden))
	require.NotEmpty(t, golden.Cases)

	for _, c := range golden.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got := domain.ParseChordSymbol(c.Input)

			require.Equal(t, c.Expected.Status, string(got.Status))
			assert.Equal(t, c.Expected.Warning, string(got.Warning))
			if c.Expected.Parsed == nil {
				assert.Nil(t, got.Parsed)
				return
			}
			want := c.Expected.Parsed
			require.NotNil(t, got.Parsed)
			assert.Equal(t, domain.ParsedChordSymbol{
				Root: want.Root, RootPitchClass: want.RootPitchClass, Quality: domain.ChordQuality(want.Quality),
				Bass: want.Bass, BassPitchClass: want.BassPitchClass, CanonicalSymbol: want.CanonicalSymbol,
			}, *got.Parsed)
		})
	}
}
