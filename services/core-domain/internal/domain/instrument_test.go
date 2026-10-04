package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func intPtr(n int) *int { return &n }

var (
	offeredLanguages = []string{"en", "pt_BR"}
	guitarNames      = map[string]string{"en": "Guitar", "pt_BR": "Violão"}
	guitarVoice      = domain.Voice{ID: "acoustic-guitar", Family: domain.InstrumentFamilyFretted}
	pianoVoice       = domain.Voice{ID: "piano", Family: domain.InstrumentFamilyKeyboard}
)

func TestNewInstrument(t *testing.T) {
	guitarTuning := []string{"E2", "A2", "D3", "G3", "B3", "E4"}
	pianoRange := &domain.KeyRange{Lowest: "A0", Highest: "C8"}

	tests := []struct {
		name        string
		family      domain.InstrumentFamily
		stringCount *int
		tuning      []string
		keyRange    *domain.KeyRange
		voice       domain.Voice
		wantField   string // empty means the constructor must succeed
	}{
		{name: "fretted with string count and matching tuning", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), tuning: guitarTuning, voice: guitarVoice},
		{name: "fretted with accidentals and low octaves in its tuning", family: domain.InstrumentFamilyFretted, stringCount: intPtr(4), tuning: []string{"Eb1", "Ab1", "C#2", "F#2"}, voice: guitarVoice},
		{name: "keyboard with a key range", family: domain.InstrumentFamilyKeyboard, keyRange: pianoRange, voice: pianoVoice},
		{name: "fretted without tuning", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), voice: guitarVoice, wantField: "tuning"},
		{name: "fretted whose tuning has no octaves", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), tuning: []string{"E", "A", "D", "G", "B", "E"}, voice: guitarVoice, wantField: "tuning"},
		{name: "fretted with one tuning entry that is not a pitch", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), tuning: []string{"E2", "A2", "D3", "G3", "B3", "X4"}, voice: guitarVoice, wantField: "tuning"},
		{name: "fretted without string count", family: domain.InstrumentFamilyFretted, tuning: guitarTuning, voice: guitarVoice, wantField: "string_count"},
		{name: "fretted with a non-positive string count", family: domain.InstrumentFamilyFretted, stringCount: intPtr(0), tuning: []string{}, voice: guitarVoice, wantField: "string_count"},
		{name: "fretted whose tuning length differs from string count", family: domain.InstrumentFamilyFretted, stringCount: intPtr(4), tuning: guitarTuning, voice: guitarVoice, wantField: "tuning"},
		{name: "fretted carrying a key range", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), tuning: guitarTuning, keyRange: pianoRange, voice: guitarVoice, wantField: "key_range"},
		{name: "keyboard carrying tuning", family: domain.InstrumentFamilyKeyboard, tuning: guitarTuning, keyRange: pianoRange, voice: pianoVoice, wantField: "tuning"},
		{name: "keyboard carrying a string count", family: domain.InstrumentFamilyKeyboard, stringCount: intPtr(6), keyRange: pianoRange, voice: pianoVoice, wantField: "string_count"},
		{name: "keyboard without a key range", family: domain.InstrumentFamilyKeyboard, voice: pianoVoice, wantField: "key_range"},
		{name: "keyboard with an empty lowest key", family: domain.InstrumentFamilyKeyboard, keyRange: &domain.KeyRange{Highest: "C8"}, voice: pianoVoice, wantField: "key_range"},
		{name: "keyboard with an empty highest key", family: domain.InstrumentFamilyKeyboard, keyRange: &domain.KeyRange{Lowest: "A0"}, voice: pianoVoice, wantField: "key_range"},
		{name: "unrecognised family", family: domain.InstrumentFamily("strummed"), voice: guitarVoice, wantField: "family"},
		{name: "without a default voice", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), tuning: guitarTuning, wantField: "default_voice_id"},
		{name: "fretted with a keyboard voice", family: domain.InstrumentFamilyFretted, stringCount: intPtr(6), tuning: guitarTuning, voice: pianoVoice, wantField: "default_voice_id"},
		{name: "keyboard with a fretted voice", family: domain.InstrumentFamilyKeyboard, keyRange: pianoRange, voice: guitarVoice, wantField: "default_voice_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewInstrument("instrument-1", guitarNames, offeredLanguages, tt.family, tt.stringCount, tt.tuning, tt.keyRange, tt.voice)

			if tt.wantField == "" {
				require.NoError(t, err)
				assert.Equal(t, "instrument-1", got.ID)
				assert.Equal(t, domain.LocalizedText{"en": "Guitar", "pt_BR": "Violão"}, got.Names)
				assert.Equal(t, []string{"en", "pt_BR"}, got.Names.Languages())
				assert.Equal(t, tt.family, got.Family)
				assert.Equal(t, tt.voice.ID, got.DefaultVoiceID)
				return
			}
			require.Error(t, err)
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			require.Len(t, valErr.Fields, 1)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}

	t.Run("names missing an offered language are rejected", func(t *testing.T) {
		_, err := domain.NewInstrument("instrument-1", map[string]string{"en": "Piano"}, offeredLanguages, domain.InstrumentFamilyKeyboard, nil, nil, pianoRange, pianoVoice)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "names", valErr.Fields[0].Field)
	})
}

func TestInstrument_WithDefaultVoice(t *testing.T) {
	guitar, err := domain.NewInstrument("guitar", guitarNames, offeredLanguages, domain.InstrumentFamilyFretted, intPtr(6), []string{"E2", "A2", "D3", "G3", "B3", "E4"}, nil, guitarVoice)
	require.NoError(t, err)

	t.Run("a voice of the same family replaces the default", func(t *testing.T) {
		electric := domain.Voice{ID: "electric-guitar", Family: domain.InstrumentFamilyFretted}

		got, err := guitar.WithDefaultVoice(electric)

		require.NoError(t, err)
		assert.Equal(t, "electric-guitar", got.DefaultVoiceID)
		assert.Equal(t, "acoustic-guitar", guitar.DefaultVoiceID, "the receiver is unchanged")
	})

	t.Run("a voice of another family is rejected", func(t *testing.T) {
		_, err := guitar.WithDefaultVoice(pianoVoice)

		requireFieldError(t, err, "default_voice_id")
	})
}

func TestInstrument_Icon(t *testing.T) {
	guitar, err := domain.NewInstrument("guitar", guitarNames, offeredLanguages, domain.InstrumentFamilyFretted, intPtr(6), []string{"E2", "A2", "D3", "G3", "B3", "E4"}, nil, guitarVoice)
	require.NoError(t, err)

	t.Run("a new instrument shows its family's generic icon", func(t *testing.T) {
		assert.Equal(t, "fretted", guitar.Icon)

		piano, err := domain.NewInstrument("piano", guitarNames, offeredLanguages, domain.InstrumentFamilyKeyboard, nil, nil, &domain.KeyRange{Lowest: "A0", Highest: "C8"}, pianoVoice)
		require.NoError(t, err)
		assert.Equal(t, "keyboard", piano.Icon)
	})

	t.Run("an icon key replaces it", func(t *testing.T) {
		got, err := guitar.WithIcon("electric_bass")

		require.NoError(t, err)
		assert.Equal(t, "electric_bass", got.Icon)
		assert.Equal(t, "fretted", guitar.Icon, "the receiver is unchanged")
	})

	for _, icon := range []string{"", "Electric Bass!", "electric-bass", "_bass", "9string"} {
		t.Run("rejects "+icon, func(t *testing.T) {
			_, err := guitar.WithIcon(icon)

			requireFieldError(t, err, "icon")
		})
	}
}
