package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestChordMapping(t *testing.T) {
	shell, template := "a_shape", "major-7-a-shape"
	voicing := domain.ChordVoicing{
		ID: uuid.NewString(), ChordDefinitionID: uuid.NewString(), DiagramID: uuid.NewString(), InstrumentID: uuid.NewString(),
		TuningFingerprint: "E2-A2-D3-G3-B3-E4", LowestFret: 1, HighestFret: 3,
		Fingering: []domain.VoicingFinger{{PositionID: "p5", Finger: "1"}}, MutedStrings: []int{6}, OmittedIntervals: []string{},
		Difficulty: "intermediate", TechniqueTags: []string{"barre"}, ShapeFamily: &shell, IsMovable: true, RecommendedRank: 1,
		Status: domain.ChordVoicingActive, TemplateKey: &template,
	}
	chord := domain.ChordDefinition{
		ID: uuid.NewString(), CanonicalSymbol: "Bbmaj7", Root: "Bb", RootPitchClass: 10, Quality: domain.ChordQualityMajor7,
		Formula: []string{"R", "3", "5", "7"}, Aliases: []string{"BbM7"}, Voicings: []domain.ChordVoicing{voicing},
	}

	t.Run("a chord reaches the response with its voicings", func(t *testing.T) {
		got := toGeneratedChord(chord)

		assert.Equal(t, "Bbmaj7", got.CanonicalSymbol)
		assert.Equal(t, generated.ChordQuality("major_7"), got.Quality)
		assert.Equal(t, []generated.ChordInterval{"R", "3", "5", "7"}, got.Formula)
		assert.Nil(t, got.Bass)
		require.Len(t, got.Voicings, 1)
		v := got.Voicings[0]
		assert.Equal(t, 1, v.FretWindow.LowestFret)
		assert.Equal(t, 3, v.FretWindow.HighestFret)
		assert.Equal(t, generated.ChordVoicingFingeringFinger("1"), v.Fingering[0].Finger)
		assert.Equal(t, generated.ChordVoicingShapeFamily("a_shape"), *v.ShapeFamily)
		assert.Equal(t, generated.ChordVoicingProvenanceSource("template"), v.Provenance.Source)
		assert.Equal(t, "major-7-a-shape", *v.Provenance.TemplateKey)
		assert.Equal(t, generated.ChordVoicingCatalogStatus("active"), v.CatalogStatus)
		assert.NotNil(t, v.OmittedIntervals)
	})

	t.Run("a hand-authored voicing has no template", func(t *testing.T) {
		open := voicing
		open.TemplateKey = nil

		got := toGeneratedChord(domain.ChordDefinition{ID: chord.ID, Voicings: []domain.ChordVoicing{open}})

		assert.Equal(t, generated.ChordVoicingProvenanceSource("hand_authored"), got.Voicings[0].Provenance.Source)
	})

	t.Run("an unparsed search has no parsed reading and no chord", func(t *testing.T) {
		got := toGeneratedChordSearch(application.ChordSearch{
			WrittenSymbol: "H7",
			Reading:       domain.ChordSymbolReading{Status: domain.ChordSymbolUnparsed, Warning: domain.ChordSymbolWarningUnparsed},
		})

		assert.Equal(t, generated.ChordSearchResultStatus("unparsed"), got.Status)
		require.NotNil(t, got.Warning)
		assert.Equal(t, generated.ChordSearchResultWarning("unparsed_symbol"), *got.Warning)
		assert.Nil(t, got.Parsed)
		assert.Nil(t, got.Chord)
		assert.Nil(t, got.ChordWithoutBass)
	})

	t.Run("a parsed search carries its reading and chord, and no warning", func(t *testing.T) {
		reading := domain.ParseChordSymbol("B♭M7")

		got := toGeneratedChordSearch(application.ChordSearch{WrittenSymbol: "B♭M7", Reading: reading, Chord: &chord})

		assert.Equal(t, "B♭M7", got.WrittenSymbol)
		assert.Nil(t, got.Warning)
		require.NotNil(t, got.Parsed)
		assert.Equal(t, "Bbmaj7", got.Parsed.CanonicalSymbol)
		require.NotNil(t, got.Chord)
		assert.Equal(t, "Bbmaj7", got.Chord.CanonicalSymbol)
	})
}
