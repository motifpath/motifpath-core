package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestToGeneratedPracticeSessionItem_DiagramShape(t *testing.T) {
	diagramID := "928330d5-903e-572c-9d41-5fde99d51ed1"
	guitarID := "6ea2d087-ab9c-59dc-9657-8546025414d2"
	members := []domain.DiagramShapeMember{
		{Shape: "C", Names: domain.LocalizedText{"en": "C shape", "pt_BR": "Forma de C"}},
		{Shape: "A", Names: domain.LocalizedText{"en": "A shape", "pt_BR": "Forma de A"}},
	}
	item := func(shape domain.PlannedDiagramShape) domain.PracticeSessionItem {
		return domain.PracticeSessionItem{
			ItemKey: domain.DiagramShapeItemKey(diagramID), Kind: domain.PracticeItemKindDiagramShape, Reason: domain.PracticePickNew,
			Level: domain.KnowledgeLevelNew, EstimatedSeconds: domain.DiagramShapeSeconds, DiagramShape: &shape,
		}
	}

	t.Run("a shape to name lists its options in the student's language and asks no degree", func(t *testing.T) {
		got := toGeneratedPracticeSessionItem(item(domain.PlannedDiagramShape{
			DiagramID: diagramID, LayoutInstrumentID: guitarID, Drill: domain.DiagramShapeDrillNameTheShape, Family: "caged-grip", Shape: "A",
			Options: members,
		}), nil, "pt_BR")

		assert.Equal(t, generated.PracticeItemKindDiagramShape, got.Kind)
		require.NotNil(t, got.DiagramShape)
		assert.Equal(t, diagramID, got.DiagramShape.DiagramId.String())
		assert.Equal(t, generated.NameTheShape, got.DiagramShape.Drill)
		assert.Equal(t, "caged-grip", got.DiagramShape.ShapeFamily)
		assert.Equal(t, "A", got.DiagramShape.Shape)
		assert.Equal(t, guitarID, got.DiagramShape.LayoutInstrumentId.String())
		require.Len(t, got.DiagramShape.Options, 2)
		assert.Equal(t, "C", got.DiagramShape.Options[0].Shape)
		assert.Equal(t, "Forma de C", got.DiagramShape.Options[0].Name)
		assert.Equal(t, "Forma de A", got.DiagramShape.Options[1].Name)
		assert.Nil(t, got.DiagramShape.AskedInterval)
	})

	t.Run("a degree to find asks its interval and offers an empty list, never null", func(t *testing.T) {
		asked := "3"
		got := toGeneratedPracticeSessionItem(item(domain.PlannedDiagramShape{
			DiagramID: diagramID, LayoutInstrumentID: guitarID, Drill: domain.DiagramShapeDrillFindTheDegree, Family: "caged-grip", Shape: "A",
			Options: []domain.DiagramShapeMember{}, AskedInterval: &asked,
		}), nil, "en")

		require.NotNil(t, got.DiagramShape)
		assert.Equal(t, generated.FindTheDegree, got.DiagramShape.Drill)
		require.NotNil(t, got.DiagramShape.AskedInterval)
		assert.Equal(t, generated.ShapeInterval("3"), *got.DiagramShape.AskedInterval)
		assert.NotNil(t, got.DiagramShape.Options)
		assert.Empty(t, got.DiagramShape.Options)
	})
}
