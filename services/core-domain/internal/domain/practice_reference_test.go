package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestNewDiagramReference(t *testing.T) {
	t.Run("carries the id, every instrument and the playback tempo", func(t *testing.T) {
		tempo := 90
		d := domain.Diagram{ID: "d-1", InstrumentID: "guitar", InstrumentIDs: []string{"guitar", "electric"}, TempoBPM: &tempo, Names: domain.LocalizedText{"en": "Lick"}}

		got := domain.NewDiagramReference(d)

		assert.Equal(t, domain.DiagramReference{ID: "d-1", InstrumentIDs: []string{"guitar", "electric"}, TempoBPM: &tempo}, got)
	})

	t.Run("a diagram without playback has no tempo", func(t *testing.T) {
		d := domain.Diagram{ID: "d-2", InstrumentID: "guitar", InstrumentIDs: []string{"guitar"}}

		got := domain.NewDiagramReference(d)

		assert.Nil(t, got.TempoBPM)
	})

	t.Run("the instrument list is a copy", func(t *testing.T) {
		d := domain.Diagram{ID: "d-3", InstrumentIDs: []string{"guitar"}}

		got := domain.NewDiagramReference(d)
		d.InstrumentIDs[0] = "piano"

		assert.Equal(t, []string{"guitar"}, got.InstrumentIDs)
	})
}
