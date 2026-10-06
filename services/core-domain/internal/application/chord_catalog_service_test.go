package application_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func newChordCatalogFixture() *application.ChordCatalogService {
	fSharp, six := "F#", 6
	repo := &fakeChordCatalogRepository{chords: []domain.ChordDefinition{
		{ID: "bbmaj7", CanonicalSymbol: "Bbmaj7", Root: "Bb", RootPitchClass: 10, Quality: domain.ChordQualityMajor7,
			Voicings: []domain.ChordVoicing{{ID: "bbmaj7-a-shape-1", RecommendedRank: 1}, {ID: "bbmaj7-e-shape-6", RecommendedRank: 2}}},
		{ID: "bbm", CanonicalSymbol: "Bbm", Root: "Bb", RootPitchClass: 10, Quality: domain.ChordQualityMinor},
		{ID: "d", CanonicalSymbol: "D", Root: "D", RootPitchClass: 2, Quality: domain.ChordQualityMajor},
		{ID: "d-over-f-sharp", CanonicalSymbol: "D/F#", Root: "D", RootPitchClass: 2, Quality: domain.ChordQualityMajor, Bass: &fSharp, BassPitchClass: &six},
	}}
	return application.NewChordCatalogService(repo)
}

func TestChordCatalogService_SearchChords(t *testing.T) {
	ctx := context.Background()
	chordID := func(c *domain.ChordDefinition) string {
		if c == nil {
			return ""
		}
		return c.ID
	}

	tests := []struct {
		name            string
		symbol          string
		status          domain.ChordSymbolStatus
		warning         domain.ChordSymbolWarning
		chord           string
		chordNoBass     string
		parsedCanonical string
	}{
		{name: "every spelling of a chord finds it", symbol: "B♭M7", status: domain.ChordSymbolParsed, chord: "bbmaj7", parsedCanonical: "Bbmaj7"},
		{name: "a root spelled otherwise finds the chord by pitch", symbol: "A#m", status: domain.ChordSymbolParsed, chord: "bbm", parsedCanonical: "A#m"},
		{name: "a slash chord the catalog has is found with its bass", symbol: "D/F#", status: domain.ChordSymbolParsed, chord: "d-over-f-sharp", parsedCanonical: "D/F#"},
		{name: "a slash chord the catalog lacks offers the chord without its bass", symbol: "D/A", status: domain.ChordSymbolParsed, chordNoBass: "d", parsedCanonical: "D/A"},
		{name: "a chord the catalog lacks is parsed but not found", symbol: "C#7", status: domain.ChordSymbolParsed, parsedCanonical: "C#7"},
		{name: "an unparsed symbol finds nothing, with its warning", symbol: "H7", status: domain.ChordSymbolUnparsed, warning: domain.ChordSymbolWarningUnparsed},
		{name: "an unsupported quality is not reinterpreted", symbol: "C7#11", status: domain.ChordSymbolUnparsed, warning: domain.ChordSymbolWarningUnsupportedQuality},
		{name: "a no-chord marking finds nothing", symbol: "N.C.", status: domain.ChordSymbolNoChord},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newChordCatalogFixture().SearchChords(ctx, teacherCaller(), tt.symbol)

			require.NoError(t, err)
			assert.Equal(t, tt.symbol, got.WrittenSymbol)
			assert.Equal(t, tt.status, got.Reading.Status)
			assert.Equal(t, tt.warning, got.Reading.Warning)
			assert.Equal(t, tt.chord, chordID(got.Chord))
			assert.Equal(t, tt.chordNoBass, chordID(got.ChordWithoutBass))
			if tt.parsedCanonical != "" {
				assert.Equal(t, tt.parsedCanonical, got.Reading.Parsed.CanonicalSymbol)
			}
		})
	}

	t.Run("a found chord carries its voicings best first", func(t *testing.T) {
		got, err := newChordCatalogFixture().SearchChords(ctx, adminCaller(), "Bbmaj7")

		require.NoError(t, err)
		require.NotNil(t, got.Chord)
		assert.Equal(t, "bbmaj7-a-shape-1", got.Chord.Voicings[0].ID)
	})

	t.Run("a symbol longer than 32 characters is a validation error", func(t *testing.T) {
		_, err := newChordCatalogFixture().SearchChords(ctx, teacherCaller(), strings.Repeat("C", 33))

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "symbol", valErr.Fields[0].Field)
	})

	t.Run("an empty symbol is a validation error", func(t *testing.T) {
		_, err := newChordCatalogFixture().SearchChords(ctx, teacherCaller(), "")

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
	})

	t.Run("a student cannot search the catalog", func(t *testing.T) {
		_, err := newChordCatalogFixture().SearchChords(ctx, studentCaller(), "Bbmaj7")

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestChordCatalogService_GetChord(t *testing.T) {
	ctx := context.Background()

	t.Run("a teacher reads a chord with its voicings", func(t *testing.T) {
		got, err := newChordCatalogFixture().GetChord(ctx, teacherCaller(), "bbmaj7")

		require.NoError(t, err)
		assert.Equal(t, "Bbmaj7", got.CanonicalSymbol)
		assert.Len(t, got.Voicings, 2)
	})

	t.Run("an unknown chord is not found", func(t *testing.T) {
		_, err := newChordCatalogFixture().GetChord(ctx, adminCaller(), "nope")

		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot read the catalog", func(t *testing.T) {
		_, err := newChordCatalogFixture().GetChord(ctx, studentCaller(), "bbmaj7")

		require.ErrorIs(t, err, domain.ErrForbidden)
	})
}
