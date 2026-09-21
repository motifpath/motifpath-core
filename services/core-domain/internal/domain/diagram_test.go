package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func strPtr(s string) *string { return &s }

func fretted(id, interval, note string, str, fret int) domain.Position {
	return domain.Position{ID: id, Interval: interval, NoteName: note, String: intPtr(str), Fret: intPtr(fret)}
}

func keyboard(id, interval, note, key string) domain.Position {
	return domain.Position{ID: id, Interval: interval, NoteName: note, Key: strPtr(key)}
}

func mustInstrument(t *testing.T, family domain.InstrumentFamily) domain.Instrument {
	t.Helper()
	switch family {
	case domain.InstrumentFamilyFretted:
		i, err := domain.NewInstrument("guitar", "6-string guitar", family, intPtr(6), []string{"E", "A", "D", "G", "B", "E"}, nil)
		require.NoError(t, err)
		return i
	case domain.InstrumentFamilyKeyboard:
		i, err := domain.NewInstrument("piano", "Piano", family, nil, nil, &domain.KeyRange{Lowest: "A0", Highest: "C8"})
		require.NoError(t, err)
		return i
	}
	t.Fatalf("no fixture for family %q", family)
	return domain.Instrument{}
}

// TestNewDiagram_PositionFamilyInvariant covers the one rule the database
// cannot enforce: every position in a diagram must use the coordinate shape
// of the diagram's instrument family, and a diagram cannot mix shapes.
func TestNewDiagram_PositionFamilyInvariant(t *testing.T) {
	tests := []struct {
		name      string
		family    domain.InstrumentFamily
		positions []domain.Position
		wantField string // empty means the constructor must succeed
	}{
		{name: "fretted positions on a fretted instrument", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{fretted("p1", "R", "A", 6, 5), fretted("p2", "b3", "C", 6, 8)}},
		{name: "an open-string fretted position is allowed", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{fretted("p1", "R", "E", 6, 0)}},
		{name: "keyboard positions on a keyboard instrument", family: domain.InstrumentFamilyKeyboard,
			positions: []domain.Position{keyboard("p1", "R", "A", "A3"), keyboard("p2", "b3", "C", "C4")}},

		{name: "keyboard position on a fretted instrument", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{keyboard("p1", "R", "A", "A3")}, wantField: "positions"},
		{name: "fretted position on a keyboard instrument", family: domain.InstrumentFamilyKeyboard,
			positions: []domain.Position{fretted("p1", "R", "A", 6, 5)}, wantField: "positions"},
		{name: "one keyboard position mixed into fretted positions", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{fretted("p1", "R", "A", 6, 5), keyboard("p2", "b3", "C", "C4")}, wantField: "positions"},
		{name: "fretted position carrying a key as well", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{{ID: "p1", Interval: "R", NoteName: "A", String: intPtr(6), Fret: intPtr(5), Key: strPtr("A3")}}, wantField: "positions"},
		{name: "keyboard position carrying a fret as well", family: domain.InstrumentFamilyKeyboard,
			positions: []domain.Position{{ID: "p1", Interval: "R", NoteName: "A", Key: strPtr("A3"), Fret: intPtr(5)}}, wantField: "positions"},
		{name: "fretted position missing its string", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{{ID: "p1", Interval: "R", NoteName: "A", Fret: intPtr(5)}}, wantField: "positions"},
		{name: "fretted position missing its fret", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{{ID: "p1", Interval: "R", NoteName: "A", String: intPtr(6)}}, wantField: "positions"},
		{name: "fretted position with a string below 1", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{fretted("p1", "R", "A", 0, 5)}, wantField: "positions"},
		{name: "fretted position with a negative fret", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{fretted("p1", "R", "A", 6, -1)}, wantField: "positions"},
		{name: "fretted position on a string the instrument does not have", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{fretted("p1", "R", "A", 7, 5)}, wantField: "positions"},
		{name: "keyboard position with an empty key", family: domain.InstrumentFamilyKeyboard,
			positions: []domain.Position{keyboard("p1", "R", "A", "")}, wantField: "positions"},
		{name: "position without an interval", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{fretted("p1", "", "A", 6, 5)}, wantField: "positions"},
		{name: "position without a note name", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{fretted("p1", "R", "", 6, 5)}, wantField: "positions"},
		{name: "position with a negative sequence index", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{{ID: "p1", Interval: "R", NoteName: "A", String: intPtr(6), Fret: intPtr(5), SequenceIndex: intPtr(-1)}}, wantField: "positions"},
		{name: "duplicate position ids", family: domain.InstrumentFamilyFretted,
			positions: []domain.Position{fretted("p1", "R", "A", 6, 5), fretted("p1", "b3", "C", 6, 8)}, wantField: "positions"},
		{name: "no positions at all", family: domain.InstrumentFamilyFretted, positions: nil, wantField: "positions"},
	}

	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instrument := mustInstrument(t, tt.family)

			got, err := domain.NewDiagram("diagram-1", instrument, "Minor Pentatonic", tt.positions, []string{"skill-1"}, []string{"concept-1"}, now)

			if tt.wantField == "" {
				require.NoError(t, err)
				assert.Equal(t, "diagram-1", got.ID)
				assert.Equal(t, instrument.ID, got.InstrumentID)
				assert.Equal(t, tt.positions, got.Positions)
				assert.Equal(t, now, got.CreatedAt)
				return
			}
			require.Error(t, err)
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			require.Len(t, valErr.Fields, 1)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}
}

func TestNewDiagram_RequiredFields(t *testing.T) {
	instrument := mustInstrument(t, domain.InstrumentFamilyFretted)
	positions := []domain.Position{fretted("p1", "R", "A", 6, 5)}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		diagName   string
		skillIDs   []string
		conceptIDs []string
		wantField  string
	}{
		{name: "empty name", diagName: "", skillIDs: []string{"s"}, conceptIDs: []string{"c"}, wantField: "name"},
		{name: "no skills", diagName: "D", skillIDs: nil, conceptIDs: []string{"c"}, wantField: "skill_ids"},
		{name: "no concepts", diagName: "D", skillIDs: []string{"s"}, conceptIDs: nil, wantField: "concept_ids"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewDiagram("diagram-1", instrument, tt.diagName, positions, tt.skillIDs, tt.conceptIDs, now)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}

	t.Run("classification carries the given ids", func(t *testing.T) {
		got, err := domain.NewDiagram("diagram-1", instrument, "D", positions, []string{"s1", "s2"}, []string{"c1"}, now)

		require.NoError(t, err)
		assert.Equal(t, []string{"s1", "s2"}, got.SkillIDs())
		assert.Equal(t, []string{"c1"}, got.ConceptIDs())
	})
}
