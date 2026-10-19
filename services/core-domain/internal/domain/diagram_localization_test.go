package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

var offered = []string{"en", "pt_BR"}

func TestNewDiagram_Names(t *testing.T) {
	instrument := mustInstrument(t, domain.InstrumentFamilyFretted)
	positions := []domain.Position{fretted("p1", "R", "A", 6, 5)}
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	build := func(names map[string]string, kind domain.DiagramKind) (domain.Diagram, error) {
		return domain.NewDiagram("diagram-1", "user-1", instrument, names, offered, positions, []string{"s"}, []string{"c"}, domain.DiagramOptions{Kind: kind}, now)
	}

	t.Run("a custom diagram may be named in any one offered language", func(t *testing.T) {
		for _, names := range []map[string]string{{"en": "Minor Pentatonic"}, {"pt_BR": "Pentatônica menor"}} {
			got, err := build(names, domain.DiagramKindCustom)

			require.NoError(t, err)
			assert.Equal(t, domain.LocalizedText(names), got.Names)
		}
	})

	t.Run("a basic diagram named in every language is accepted", func(t *testing.T) {
		got, err := build(map[string]string{"en": "Major Scale", "pt_BR": "Escala maior"}, domain.DiagramKindBasic)

		require.NoError(t, err)
		assert.Equal(t, []string{"en", "pt_BR"}, got.Names.Languages())
	})

	tests := []struct {
		name  string
		names map[string]string
		kind  domain.DiagramKind
	}{
		{name: "a basic diagram missing a language", names: map[string]string{"en": "Major Scale"}, kind: domain.DiagramKindBasic},
		{name: "no name at all", names: map[string]string{}, kind: domain.DiagramKindCustom},
		{name: "a language nobody offers", names: map[string]string{"en": "Scale", "fr": "Gamme"}, kind: domain.DiagramKindCustom},
		{name: `the "any" marker`, names: map[string]string{"en": "Scale", "any": "Scale"}, kind: domain.DiagramKindCustom},
		{name: "a blank name", names: map[string]string{"en": "   "}, kind: domain.DiagramKindCustom},
		{name: "a name over 200 characters", names: map[string]string{"en": strings.Repeat("x", 201)}, kind: domain.DiagramKindCustom},
	}
	for _, tt := range tests {
		t.Run("rejected: "+tt.name, func(t *testing.T) {
			_, err := build(tt.names, tt.kind)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "names", valErr.Fields[0].Field)
		})
	}
}

func TestNewDiagram_IntervalAndNoteCodes(t *testing.T) {
	instrument := mustInstrument(t, domain.InstrumentFamilyFretted)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	build := func(p domain.Position) error {
		_, err := domain.NewDiagram("diagram-1", "user-1", instrument, map[string]string{"en": "D"}, offered, []domain.Position{p}, []string{"s"}, []string{"c"}, domain.DiagramOptions{}, now)
		return err
	}

	t.Run("every canonical interval code is accepted, spelled as the author wrote it", func(t *testing.T) {
		codes := []string{"R", "b2", "2", "#2", "b3", "3", "4", "#4", "b5", "5", "#5", "b6", "6", "bb7", "b7", "7", "b9", "9", "#9", "11", "#11", "b13", "13"}
		for _, code := range codes {
			require.NoError(t, build(fretted("p1", code, "A", 6, 5)), "interval %q", code)
		}
	})

	t.Run("letter note names with up to two accidentals are accepted", func(t *testing.T) {
		for _, note := range []string{"A", "C#", "Eb", "Bbb", "F##"} {
			require.NoError(t, build(fretted("p1", "R", note, 6, 5)), "note %q", note)
		}
	})

	rejected := []struct {
		name     string
		position domain.Position
	}{
		{name: "an interval outside the canonical codes", position: fretted("p1", "m3", "C", 6, 8)},
		{name: "a root written as 1", position: fretted("p1", "1", "A", 6, 5)},
		{name: "a solfège note name", position: fretted("p1", "R", "Lá", 6, 5)},
		{name: "a lowercase note name", position: fretted("p1", "R", "a", 6, 5)},
		{name: "a note name outside A-G", position: fretted("p1", "R", "H", 6, 5)},
		{name: "three accidentals", position: fretted("p1", "R", "Cbbb", 6, 5)},
	}
	for _, tt := range rejected {
		t.Run("rejected: "+tt.name, func(t *testing.T) {
			var valErr *domain.ValidationError
			require.ErrorAs(t, build(tt.position), &valErr)
			assert.Equal(t, "positions", valErr.Fields[0].Field)
		})
	}
}
