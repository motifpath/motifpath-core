package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func intPtr(n int) *int { return &n }

func TestNewInstrument(t *testing.T) {
	guitarTuning := []string{"E", "A", "D", "G", "B", "E"}
	pianoRange := &domain.KeyRange{Lowest: "A0", Highest: "C8"}

	tests := []struct {
		name        string
		family      domain.InstrumentFamily
		stringCount *int
		tuning      []string
		keyRange    *domain.KeyRange
		wantField   string // empty means the constructor must succeed
	}{
		{name: "fretted with string count and matching tuning", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), tuning: guitarTuning},
		{name: "keyboard with a key range", family: domain.InstrumentFamilyKeyboard, keyRange: pianoRange},
		{name: "fretted without tuning", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), wantField: "tuning"},
		{name: "fretted without string count", family: domain.InstrumentFamilyFretted, tuning: guitarTuning, wantField: "string_count"},
		{name: "fretted with a non-positive string count", family: domain.InstrumentFamilyFretted, stringCount: intPtr(0), tuning: []string{}, wantField: "string_count"},
		{name: "fretted whose tuning length differs from string count", family: domain.InstrumentFamilyFretted, stringCount: intPtr(4), tuning: guitarTuning, wantField: "tuning"},
		{name: "fretted carrying a key range", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), tuning: guitarTuning, keyRange: pianoRange, wantField: "key_range"},
		{name: "keyboard carrying tuning", family: domain.InstrumentFamilyKeyboard, tuning: guitarTuning, keyRange: pianoRange, wantField: "tuning"},
		{name: "keyboard carrying a string count", family: domain.InstrumentFamilyKeyboard, stringCount: intPtr(6), keyRange: pianoRange, wantField: "string_count"},
		{name: "keyboard without a key range", family: domain.InstrumentFamilyKeyboard, wantField: "key_range"},
		{name: "keyboard with an empty lowest key", family: domain.InstrumentFamilyKeyboard, keyRange: &domain.KeyRange{Highest: "C8"}, wantField: "key_range"},
		{name: "keyboard with an empty highest key", family: domain.InstrumentFamilyKeyboard, keyRange: &domain.KeyRange{Lowest: "A0"}, wantField: "key_range"},
		{name: "unrecognised family", family: domain.InstrumentFamily("strummed"), wantField: "family"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewInstrument("instrument-1", "An instrument", tt.family, tt.stringCount, tt.tuning, tt.keyRange)

			if tt.wantField == "" {
				require.NoError(t, err)
				assert.Equal(t, "instrument-1", got.ID)
				assert.Equal(t, "An instrument", got.Name)
				assert.Equal(t, tt.family, got.Family)
				return
			}
			require.Error(t, err)
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			require.Len(t, valErr.Fields, 1)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}

	t.Run("an empty name is rejected", func(t *testing.T) {
		_, err := domain.NewInstrument("instrument-1", "", domain.InstrumentFamilyKeyboard, nil, nil, pianoRange)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "name", valErr.Fields[0].Field)
	})
}
