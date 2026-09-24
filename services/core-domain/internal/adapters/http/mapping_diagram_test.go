package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func strPtr(s string) *string { return &s }

func TestDiagramColorMapping(t *testing.T) {
	positionID := uuid.NewString()
	diagram := domain.Diagram{
		ID:           uuid.NewString(),
		InstrumentID: uuid.NewString(),
		Name:         "Colored",
		LabelDisplay: domain.LabelDisplayInterval,
		Color:        strPtr("#3B82F6"),
		Positions: []domain.Position{
			{ID: positionID, Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, Color: strPtr("#EF4444")},
			{ID: uuid.NewString(), Interval: "b3", NoteName: "C", Shape: domain.PositionShapeDot},
		},
		CreatedAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC),
	}

	t.Run("a diagram's general and per-position colors reach the response", func(t *testing.T) {
		got := toGeneratedDiagram(diagram)

		require.NotNil(t, got.Color)
		assert.Equal(t, "#3B82F6", *got.Color)
		require.NotNil(t, got.Positions[0].Color)
		assert.Equal(t, "#EF4444", *got.Positions[0].Color)
		assert.Nil(t, got.Positions[1].Color)
	})

	t.Run("an unrecorded general color stays nil in the response", func(t *testing.T) {
		plain := diagram
		plain.Color = nil

		assert.Nil(t, toGeneratedDiagram(plain).Color)
	})

	t.Run("the list mapping carries colors too", func(t *testing.T) {
		got := toGeneratedDiagrams([]domain.Diagram{diagram})

		require.Len(t, got, 1)
		require.NotNil(t, got[0].Color)
		assert.Equal(t, "#3B82F6", *got[0].Color)
	})

	t.Run("request positions carry their color into the domain, and an omitted color stays nil", func(t *testing.T) {
		got := toDomainPositions([]generated.DiagramPosition{
			{Interval: "R", NoteName: "A", Color: strPtr("#EF4444")},
			{Interval: "b3", NoteName: "C"},
		})

		require.NotNil(t, got[0].Color)
		assert.Equal(t, "#EF4444", *got[0].Color)
		assert.Nil(t, got[1].Color)
	})
}
