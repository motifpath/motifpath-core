package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// voicingGuitar is standard-tuning six-string guitar, lowest string first.
func voicingGuitar() domain.Instrument {
	six := 6
	return domain.Instrument{ID: "guitar", Family: domain.InstrumentFamilyFretted, StringCount: &six, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}}
}

// shape builds a chord_voicing diagram from string → fret pairs; strings it
// doesn't list are muted.
func shape(frets map[int]int) domain.Diagram {
	d := domain.Diagram{ID: "diagram", InstrumentID: "guitar", Kind: domain.DiagramKindBasic, Purpose: domain.DiagramPurposeChordVoicing}
	for s := 6; s >= 1; s-- {
		fret, ok := frets[s]
		if !ok {
			continue
		}
		str, fr := s, fret
		d.Positions = append(d.Positions, domain.Position{ID: "p" + string(rune('0'+s)), String: &str, Fret: &fr})
	}
	return d
}

func chord(symbol string, rootPC int, formula, omittable []string) domain.ChordDefinition {
	return domain.ChordDefinition{ID: "chord", CanonicalSymbol: symbol, RootPitchClass: rootPC, Formula: formula, Omittable: omittable}
}

func slashChord(symbol string, rootPC, bassPC int, formula []string) domain.ChordDefinition {
	c := chord(symbol, rootPC, formula, nil)
	c.BassPitchClass = &bassPC
	return c
}

func voicing(omitted ...string) domain.ChordVoicing {
	return domain.ChordVoicing{ID: "voicing", ChordDefinitionID: "chord", DiagramID: "diagram", InstrumentID: "guitar", TuningFingerprint: "E2-A2-D3-G3-B3-E4", OmittedIntervals: omitted, RecommendedRank: 1}
}

func TestValidateChordVoicing(t *testing.T) {
	aMinor := chord("Am", 9, []string{"R", "b3", "5"}, nil)
	c7 := chord("C7", 0, []string{"R", "3", "5", "b7"}, []string{"5"})
	movable := func(v domain.ChordVoicing) domain.ChordVoicing { v.IsMovable = true; return v }
	otherTuning := voicing()
	otherTuning.TuningFingerprint = "D2-A2-D3-G3-B3-E4"

	tests := []struct {
		name      string
		chord     domain.ChordDefinition
		diagram   domain.Diagram
		voicing   domain.ChordVoicing
		wantField string // empty: the voicing is valid
	}{
		{"an open Am sounds exactly A, C and E", aMinor, shape(map[int]int{5: 0, 4: 2, 3: 2, 2: 1, 1: 0}), voicing(), ""},
		{"an Am that sounds F# is rejected", aMinor, shape(map[int]int{5: 0, 4: 2, 3: 2, 2: 1, 1: 2}), voicing(), "positions"},
		{"a tone the chord needs can't be left out", aMinor, shape(map[int]int{5: 0, 4: 2, 3: 2}), voicing(), "omitted_intervals"},
		{"an omittable tone may be left out when declared", c7, shape(map[int]int{5: 3, 4: 2, 3: 3, 2: 1, 1: 0}), voicing("5"), ""},
		{"a left-out tone must be declared", c7, shape(map[int]int{5: 3, 4: 2, 3: 3, 2: 1, 1: 0}), voicing(), "omitted_intervals"},
		{"a declared omission must really be left out", aMinor, shape(map[int]int{5: 0, 4: 2, 3: 2, 2: 1, 1: 0}), voicing("5"), "omitted_intervals"},
		{"the root is never left out", chord("C5", 0, []string{"R", "5"}, []string{"R"}), shape(map[int]int{4: 5}), voicing("R"), "omitted_intervals"},
		{"a slash chord's lowest string sounds its bass", slashChord("D/F#", 2, 6, []string{"R", "3", "5"}), shape(map[int]int{6: 2, 4: 0, 3: 2, 2: 3, 1: 2}), voicing(), ""},
		{"a slash chord with another lowest note is rejected", slashChord("D/F#", 2, 6, []string{"R", "3", "5"}), shape(map[int]int{4: 0, 3: 2, 2: 3, 1: 2}), voicing(), "positions"},
		{"a slash bass outside the formula may sound", slashChord("Am/G", 9, 7, []string{"R", "b3", "5"}), shape(map[int]int{6: 3, 4: 2, 3: 2, 2: 1, 1: 0}), voicing(), ""},
		{"the voicing's tuning must be its instrument's", aMinor, shape(map[int]int{5: 0, 4: 2, 3: 2, 2: 1, 1: 0}), otherTuning, "tuning_fingerprint"},
		{"no fret is above 15", chord("A5", 9, []string{"R", "5"}, nil), shape(map[int]int{6: 17, 5: 19}), voicing(), "positions"},
		{"a movable voicing has no open strings", aMinor, shape(map[int]int{5: 0, 4: 2, 3: 2, 2: 1, 1: 0}), movable(voicing()), "is_movable"},
		{"a barre Bm moves along the neck", chord("Bm", 11, []string{"R", "b3", "5"}, nil), shape(map[int]int{5: 2, 4: 4, 3: 4, 2: 3, 1: 2}), movable(voicing()), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateChordVoicing(tt.chord, tt.diagram, voicingGuitar(), tt.voicing)

			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}

	t.Run("only a chord_voicing diagram can hold a voicing", func(t *testing.T) {
		general := shape(map[int]int{5: 0, 4: 2, 3: 2, 2: 1, 1: 0})
		general.Purpose = domain.DiagramPurposeGeneral

		err := domain.ValidateChordVoicing(aMinor, general, voicingGuitar(), voicing())

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "diagram_id", valErr.Fields[0].Field)
	})
}
