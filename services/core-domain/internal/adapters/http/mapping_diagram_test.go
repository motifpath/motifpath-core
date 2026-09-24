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
		Kind:         domain.DiagramKindCustom,
		CreatedBy:    uuid.NewString(),
		LabelDisplay: domain.LabelDisplayInterval,
		Color:        strPtr("#3B82F6"),
		Positions: []domain.Position{
			{ID: positionID, Interval: "R", NoteName: "A", Shape: domain.PositionShapeDot, Color: strPtr("#EF4444")},
			{ID: uuid.NewString(), Interval: "b3", NoteName: "C", Shape: domain.PositionShapeDot},
		},
		CreatedAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC),
	}

	t.Run("a diagram's general and per-position colors reach the response", func(t *testing.T) {
		got := toGeneratedDiagram(diagram, userNames{})

		require.NotNil(t, got.Color)
		assert.Equal(t, "#3B82F6", *got.Color)
		require.NotNil(t, got.Positions[0].Color)
		assert.Equal(t, "#EF4444", *got.Positions[0].Color)
		assert.Nil(t, got.Positions[1].Color)
	})

	t.Run("an unrecorded general color stays nil in the response", func(t *testing.T) {
		plain := diagram
		plain.Color = nil

		assert.Nil(t, toGeneratedDiagram(plain, userNames{}).Color)
	})

	t.Run("the list mapping carries colors too", func(t *testing.T) {
		got := toGeneratedDiagrams([]domain.Diagram{diagram}, userNames{})

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

func TestDiagramOwnershipMapping(t *testing.T) {
	owner := uuid.New()
	diagram := domain.Diagram{
		ID: uuid.NewString(), InstrumentID: uuid.NewString(), Name: "Owned",
		Kind: domain.DiagramKindBasic, CreatedBy: owner.String(), LabelDisplay: domain.LabelDisplayInterval,
		CreatedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
	}

	t.Run("a diagram's kind and creator reach the response", func(t *testing.T) {
		got := toGeneratedDiagram(diagram, userNames{owner.String(): "Ana Souza"})

		assert.Equal(t, generated.DiagramKindBasic, got.Kind)
		assert.Equal(t, generated.UserRef{UserId: owner, DisplayName: "Ana Souza"}, got.CreatedBy)
	})

	t.Run("an omitted create kind maps to the zero value, left for the domain to default", func(t *testing.T) {
		assert.Equal(t, domain.DiagramKind(""), toDomainDiagramKind(nil))
		basic := generated.CreateDiagramRequestKindBasic
		assert.Equal(t, domain.DiagramKindBasic, toDomainDiagramKind(&basic))
	})

	t.Run("list parameters map onto the diagram filter", func(t *testing.T) {
		instrument, skill, concept, creator := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		kind := generated.Custom

		got := diagramListFilter(generated.ListDiagramsParams{
			InstrumentId: &instrument, SkillId: &skill, ConceptId: &concept, CreatedBy: &creator, Kind: &kind,
		})

		assert.Equal(t, domain.DiagramListFilter{
			InstrumentID: instrument.String(), SkillID: skill.String(), ConceptID: concept.String(),
			CreatedBy: creator.String(), Kind: domain.DiagramKindCustom,
		}, got)
	})

	t.Run("absent list parameters leave the filter empty", func(t *testing.T) {
		assert.Equal(t, domain.DiagramListFilter{}, diagramListFilter(generated.ListDiagramsParams{}))
	})
}
