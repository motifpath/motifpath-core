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

// progressionNumeralGoldenFile is
// golden/progression-numerals/progression_numeral.v1.json. The web client's
// create form runs the same cases, so an admin never sees a numeral the server
// reads differently.
type progressionNumeralGoldenFile struct {
	Parser string `json:"parser"`
	Cases  []struct {
		Name     string `json:"name"`
		Input    string `json:"input"`
		Mode     string `json:"mode"`
		Expected struct {
			Status        string `json:"status"`
			Numeral       string `json:"numeral"`
			RootSemitones int    `json:"root_semitones"`
			Quality       string `json:"quality"`
			Borrowed      bool   `json:"borrowed"`
		} `json:"expected"`
	} `json:"cases"`
}

func TestProgressionNumeralGoldenCases(t *testing.T) {
	specsDir := strings.TrimSuffix(featuresBase, "/features")
	raw, err := os.ReadFile(filepath.Join(specsDir, "golden", "progression-numerals", "progression_numeral.v1.json"))
	require.NoError(t, err, "the numeral parser needs its golden cases in motifpath-specs")
	var golden progressionNumeralGoldenFile
	require.NoError(t, json.Unmarshal(raw, &golden))
	require.NotEmpty(t, golden.Cases)

	for _, c := range golden.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got, ok := domain.ParseProgressionNumeral(c.Input, domain.ProgressionMode(c.Mode))

			if c.Expected.Status == "unparsed" {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			assert.Equal(t, domain.ProgressionNumeral{
				Numeral:       c.Expected.Numeral,
				RootSemitones: c.Expected.RootSemitones,
				Quality:       domain.ChordQuality(c.Expected.Quality),
				Borrowed:      c.Expected.Borrowed,
			}, got)
		})
	}
}
