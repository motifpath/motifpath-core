package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

var bilingual = map[string]string{"en": "Minor Pentatonic", "pt_BR": "Pentatônica menor"}

func annotated(label, note domain.LocalizedText) domain.Position {
	p := fretted("p1", "R", "A", 6, 5)
	p.CustomLabel = label
	p.Note = note
	return p
}

func frettedRegion(id string, fretStart, fretEnd int, description domain.LocalizedText) domain.Region {
	return domain.Region{ID: id, FretStart: intPtr(fretStart), FretEnd: intPtr(fretEnd), Description: description}
}

func keyboardRegion(id, keyStart, keyEnd string, description domain.LocalizedText) domain.Region {
	return domain.Region{ID: id, KeyStart: strPtr(keyStart), KeyEnd: strPtr(keyEnd), Description: description}
}

func TestNewDiagram_PositionAnnotations(t *testing.T) {
	instrument := mustInstrument(t, domain.InstrumentFamilyFretted)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	build := func(names map[string]string, p domain.Position) (domain.Diagram, error) {
		return domain.NewDiagram("diagram-1", "user-1", instrument, names, offered, []domain.Position{p}, []string{"s"}, []string{"c"}, domain.DiagramOptions{}, now)
	}

	t.Run("a position without a custom label or note has neither", func(t *testing.T) {
		got, err := build(bilingual, fretted("p1", "R", "A", 6, 5))

		require.NoError(t, err)
		assert.Nil(t, got.Positions[0].CustomLabel)
		assert.Nil(t, got.Positions[0].Note)
	})

	t.Run("a custom label and a note in every language of the name are kept, trimmed", func(t *testing.T) {
		got, err := build(bilingual, annotated(
			domain.LocalizedText{"en": " Av ", "pt_BR": "Ev"},
			domain.LocalizedText{"en": "Avoid holding this over Am7", "pt_BR": " Evite sustentar sobre Am7 "},
		))

		require.NoError(t, err)
		assert.Equal(t, domain.LocalizedText{"en": "Av", "pt_BR": "Ev"}, got.Positions[0].CustomLabel)
		assert.Equal(t, domain.LocalizedText{"en": "Avoid holding this over Am7", "pt_BR": "Evite sustentar sobre Am7"}, got.Positions[0].Note)
	})

	t.Run("a two-character label counts characters, not bytes", func(t *testing.T) {
		_, err := build(map[string]string{"pt_BR": "Escala"}, annotated(domain.LocalizedText{"pt_BR": "Tô"}, nil))

		require.NoError(t, err)
	})

	t.Run("a 280-character note is accepted", func(t *testing.T) {
		_, err := build(namesOf("D"), annotated(nil, domain.LocalizedText{"en": strings.Repeat("x", 280)}))

		require.NoError(t, err)
	})

	rejected := []struct {
		name     string
		names    map[string]string
		position domain.Position
	}{
		{name: "a custom label over two characters", names: namesOf("D"), position: annotated(domain.LocalizedText{"en": "Root"}, nil)},
		{name: "a blank custom label", names: namesOf("D"), position: annotated(domain.LocalizedText{"en": "  "}, nil)},
		{name: "an empty custom label map", names: namesOf("D"), position: annotated(domain.LocalizedText{}, nil)},
		{name: "a note over 280 characters", names: namesOf("D"), position: annotated(nil, domain.LocalizedText{"en": strings.Repeat("x", 281)})},
		{name: "a note in fewer languages than the name", names: bilingual, position: annotated(nil, domain.LocalizedText{"en": "Start here"})},
		{name: "a custom label in fewer languages than the name", names: bilingual, position: annotated(domain.LocalizedText{"pt_BR": "Ev"}, nil)},
		{name: "a note in a language the name lacks", names: namesOf("D"), position: annotated(nil, domain.LocalizedText{"en": "Start", "pt_BR": "Comece"})},
		{name: `a note under the "any" marker`, names: namesOf("D"), position: annotated(nil, domain.LocalizedText{"en": "Start", "any": "Start"})},
	}
	for _, tt := range rejected {
		t.Run("rejected: "+tt.name, func(t *testing.T) {
			_, err := build(tt.names, tt.position)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "positions", valErr.Fields[0].Field)
		})
	}
}

func TestNewDiagram_Regions(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	build := func(family domain.InstrumentFamily, names map[string]string, regions []domain.Region) (domain.Diagram, error) {
		instrument := mustInstrument(t, family)
		position := fretted("p1", "R", "A", 6, 5)
		if family == domain.InstrumentFamilyKeyboard {
			position = keyboard("p1", "R", "C", "C4")
		}
		return domain.NewDiagram("diagram-1", "user-1", instrument, names, offered, []domain.Position{position}, []string{"s"}, []string{"c"}, domain.DiagramOptions{Regions: regions}, now)
	}
	caption := domain.LocalizedText{"en": "Box 1"}

	t.Run("no regions means an empty list", func(t *testing.T) {
		got, err := build(domain.InstrumentFamilyFretted, namesOf("D"), nil)

		require.NoError(t, err)
		assert.Empty(t, got.Regions)
	})

	t.Run("fretted regions are kept in order, overlapping or not, with trimmed descriptions", func(t *testing.T) {
		box2 := frettedRegion("r2", 7, 10, domain.LocalizedText{"en": " Box 2 ", "pt_BR": "Caixa 2"})
		box2.StringStart, box2.StringEnd, box2.Color = intPtr(1), intPtr(3), strPtr("#22C55E")
		regions := []domain.Region{frettedRegion("r1", 5, 8, domain.LocalizedText{"en": "Box 1", "pt_BR": "Caixa 1"}), box2}

		got, err := build(domain.InstrumentFamilyFretted, bilingual, regions)

		require.NoError(t, err)
		require.Len(t, got.Regions, 2)
		assert.Equal(t, "r1", got.Regions[0].ID)
		assert.Nil(t, got.Regions[0].StringStart)
		assert.Equal(t, domain.LocalizedText{"en": "Box 2", "pt_BR": "Caixa 2"}, got.Regions[1].Description)
		assert.Equal(t, 3, *got.Regions[1].StringEnd)
		assert.Equal(t, "#22C55E", *got.Regions[1].Color)
	})

	t.Run("a single-fret region and an open-string region are accepted", func(t *testing.T) {
		_, err := build(domain.InstrumentFamilyFretted, namesOf("D"), []domain.Region{frettedRegion("r1", 0, 0, caption), frettedRegion("r2", 5, 5, caption)})

		require.NoError(t, err)
	})

	t.Run("a keyboard key range is accepted, across octaves and with accidentals", func(t *testing.T) {
		regions := []domain.Region{keyboardRegion("r1", "C4", "B4", caption), keyboardRegion("r2", "Bb3", "C#5", caption), keyboardRegion("r3", "E4", "E4", caption)}

		got, err := build(domain.InstrumentFamilyKeyboard, namesOf("D"), regions)

		require.NoError(t, err)
		assert.Equal(t, "C4", *got.Regions[0].KeyStart)
		assert.Equal(t, "B4", *got.Regions[0].KeyEnd)
	})

	withStrings := func(r domain.Region, start, end *int) domain.Region {
		r.StringStart, r.StringEnd = start, end
		return r
	}
	withKeys := func(r domain.Region, start, end string) domain.Region {
		r.KeyStart, r.KeyEnd = strPtr(start), strPtr(end)
		return r
	}
	rejected := []struct {
		name    string
		family  domain.InstrumentFamily
		names   map[string]string
		regions []domain.Region
	}{
		{name: "a backwards fret range", family: domain.InstrumentFamilyFretted, regions: []domain.Region{frettedRegion("r1", 8, 5, caption)}},
		{name: "a negative fret", family: domain.InstrumentFamilyFretted, regions: []domain.Region{frettedRegion("r1", -1, 3, caption)}},
		{name: "a missing fret_end", family: domain.InstrumentFamilyFretted, regions: []domain.Region{{ID: "r1", FretStart: intPtr(5), Description: caption}}},
		{name: "a backwards string range", family: domain.InstrumentFamilyFretted, regions: []domain.Region{withStrings(frettedRegion("r1", 5, 8, caption), intPtr(3), intPtr(1))}},
		{name: "a string past the last one", family: domain.InstrumentFamilyFretted, regions: []domain.Region{withStrings(frettedRegion("r1", 5, 8, caption), intPtr(1), intPtr(7))}},
		{name: "a string below the first one", family: domain.InstrumentFamilyFretted, regions: []domain.Region{withStrings(frettedRegion("r1", 5, 8, caption), intPtr(0), intPtr(2))}},
		{name: "only one string bound", family: domain.InstrumentFamilyFretted, regions: []domain.Region{withStrings(frettedRegion("r1", 5, 8, caption), intPtr(1), nil)}},
		{name: "a key range on a fretted instrument", family: domain.InstrumentFamilyFretted, regions: []domain.Region{keyboardRegion("r1", "C4", "B4", caption)}},
		{name: "a fret range carrying keys as well", family: domain.InstrumentFamilyFretted, regions: []domain.Region{withKeys(frettedRegion("r1", 5, 8, caption), "C4", "B4")}},
		{name: "a fret range on a keyboard instrument", family: domain.InstrumentFamilyKeyboard, regions: []domain.Region{frettedRegion("r1", 5, 8, caption)}},
		{name: "a backwards key range", family: domain.InstrumentFamilyKeyboard, regions: []domain.Region{keyboardRegion("r1", "B4", "C4", caption)}},
		{name: "a key that isn't a note and octave", family: domain.InstrumentFamilyKeyboard, regions: []domain.Region{keyboardRegion("r1", "middle C", "B4", caption)}},
		{name: "a missing key_end", family: domain.InstrumentFamilyKeyboard, regions: []domain.Region{{ID: "r1", KeyStart: strPtr("C4"), Description: caption}}},
		{name: "no description", family: domain.InstrumentFamilyFretted, regions: []domain.Region{frettedRegion("r1", 5, 8, nil)}},
		{name: "a description over 60 characters", family: domain.InstrumentFamilyFretted, regions: []domain.Region{frettedRegion("r1", 5, 8, domain.LocalizedText{"en": strings.Repeat("x", 61)})}},
		{name: "a description in fewer languages than the name", family: domain.InstrumentFamilyFretted, names: bilingual, regions: []domain.Region{frettedRegion("r1", 5, 8, caption)}},
		{name: "a malformed color", family: domain.InstrumentFamilyFretted, regions: []domain.Region{{ID: "r1", FretStart: intPtr(5), FretEnd: intPtr(8), Description: caption, Color: strPtr("green")}}},
		{name: "a repeated region_id", family: domain.InstrumentFamilyFretted, regions: []domain.Region{frettedRegion("r1", 5, 8, caption), frettedRegion("r1", 7, 10, caption)}},
	}
	for _, tt := range rejected {
		t.Run("rejected: "+tt.name, func(t *testing.T) {
			names := tt.names
			if names == nil {
				names = namesOf("D")
			}
			_, err := build(tt.family, names, tt.regions)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "regions", valErr.Fields[0].Field)
		})
	}
}
