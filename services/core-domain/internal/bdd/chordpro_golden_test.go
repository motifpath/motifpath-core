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

// chordProGoldenFile is golden/chordpro/chordpro.v1.json: how ChordPro text
// imports as a song chart draft, and how that draft exports again.
type chordProGoldenFile struct {
	Format string `json:"format"`
	Cases  []struct {
		Name     string `json:"name"`
		Input    string `json:"input"`
		Expected struct {
			Metadata struct {
				Title         *string `json:"title"`
				Artist        *string `json:"artist"`
				ConcertKey    *string `json:"concert_key"`
				CapoFret      *int    `json:"capo_fret"`
				TempoBPM      *int    `json:"tempo_bpm"`
				TimeSignature *struct {
					Beats     int `json:"beats"`
					BeatValue int `json:"beat_value"`
				} `json:"time_signature"`
			} `json:"metadata"`
			Body           json.RawMessage `json:"body"`
			ImportWarnings []struct {
				Line int    `json:"line"`
				Kind string `json:"kind"`
				Text string `json:"text"`
			} `json:"import_warnings"`
		} `json:"expected"`
		Export string `json:"export"`
	} `json:"cases"`
}

func TestChordProGoldenCases(t *testing.T) {
	specsDir := strings.TrimSuffix(featuresBase, "/features")
	raw, err := os.ReadFile(filepath.Join(specsDir, "golden", "chordpro", "chordpro.v1.json"))
	require.NoError(t, err, "the ChordPro import and export need their golden cases in motifpath-specs")
	var golden chordProGoldenFile
	require.NoError(t, json.Unmarshal(raw, &golden))
	require.Equal(t, "chordpro.v1", golden.Format)
	require.NotEmpty(t, golden.Cases)

	for _, c := range golden.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got := domain.ImportChordPro(c.Input)

			want := c.Expected.Metadata
			var wantTime *domain.TimeSignature
			if want.TimeSignature != nil {
				wantTime = &domain.TimeSignature{Beats: want.TimeSignature.Beats, BeatValue: want.TimeSignature.BeatValue}
			}
			assert.Equal(t, domain.ChordProMetadata{
				Title: want.Title, Artist: want.Artist, ConcertKey: want.ConcertKey,
				CapoFret: want.CapoFret, TempoBPM: want.TempoBPM, TimeSignature: wantTime,
			}, got.Metadata)

			var wantWarnings []domain.ChordProWarning
			for _, w := range c.Expected.ImportWarnings {
				wantWarnings = append(wantWarnings, domain.ChordProWarning{Line: w.Line, Kind: domain.ChordProWarningKind(w.Kind), Text: w.Text})
			}
			assert.Equal(t, wantWarnings, got.Warnings)

			body, err := json.Marshal(got.Body)
			require.NoError(t, err)
			assert.JSONEq(t, string(c.Expected.Body), string(body))

			assert.Equal(t, c.Export, domain.ExportChordPro(draftOf(got)))
		})
	}
}

// draftOf is the draft an import makes of an empty chart: every field the
// text doesn't set keeps its zero value.
func draftOf(imported domain.ChordProImport) domain.SongChartDraft {
	m := imported.Metadata
	draft := domain.SongChartDraft{ConcertKey: m.ConcertKey, TempoBPM: m.TempoBPM, TimeSignature: m.TimeSignature, Body: imported.Body}
	if m.Title != nil {
		draft.Title = *m.Title
	}
	if m.Artist != nil {
		draft.Artist = *m.Artist
	}
	if m.CapoFret != nil {
		draft.CapoFret = *m.CapoFret
	}
	return draft
}
