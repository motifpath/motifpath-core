package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestInstrumentNamesMapping(t *testing.T) {
	six := 6
	instrument := domain.Instrument{
		ID: uuid.NewString(), Names: domain.LocalizedText{"pt_BR": "Violão", "en": "Guitar"},
		Family: domain.InstrumentFamilyFretted, StringCount: &six, Tuning: []string{"E", "A", "D", "G", "B", "E"},
	}

	got := toGeneratedInstrument(instrument)

	assert.Equal(t, generated.LocalizedNames{"en": "Guitar", "pt_BR": "Violão"}, got.Names)
	assert.Equal(t, []string{"en", "pt_BR"}, got.Languages)
}
