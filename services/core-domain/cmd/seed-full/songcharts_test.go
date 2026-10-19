package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestSeededSongChartsImportAsWritten(t *testing.T) {
	specs := map[string]songChartSpec{}
	for _, s := range songChartSpecs() {
		specs[s.title] = s
	}
	require.ElementsMatch(t, []string{"Amazing Grace", "Oh! Susanna", "Ciranda, Cirandinha"}, keys(specs))

	for title, s := range specs {
		t.Run(title, func(t *testing.T) {
			imported := domain.ImportChordPro(s.chordPro)

			require.NotNil(t, imported.Metadata.Title)
			assert.Equal(t, title, *imported.Metadata.Title)
			assert.Empty(t, imported.Warnings, "every seeded line is supported ChordPro")
			assert.True(t, imported.Body.HasLyrics())
			if s.correction != "" {
				assert.Empty(t, domain.ImportChordPro(s.correction).Warnings)
			}
		})
	}
}

func TestSeededSongChartsCoverEveryState(t *testing.T) {
	fates := map[string]songChartFate{}
	for _, s := range songChartSpecs() {
		fates[s.title] = s.fate
	}

	assert.Equal(t, map[string]songChartFate{
		"Amazing Grace":       publishedThenCorrected,
		"Oh! Susanna":         publishedThenWithdrawn,
		"Ciranda, Cirandinha": draftOnly,
	}, fates)
}

func TestTheDraftChartHasChordsThatDontFullyResolve(t *testing.T) {
	var draft songChartSpec
	for _, s := range songChartSpecs() {
		if s.fate == draftOnly {
			draft = s
		}
	}
	written := map[string]bool{}
	for _, a := range domain.ImportChordPro(draft.chordPro).Body.Anchors() {
		written[a.Anchor.WrittenSymbol] = true
	}

	assert.False(t, draft.rightsConfirmed, "a draft whose rights aren't confirmed")
	assert.Equal(t, "pt_BR", draft.language)
	for _, symbol := range []string{"N.C.", "C/B", "H7"} {
		assert.True(t, written[symbol], "the draft has an anchor written %q", symbol)
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
