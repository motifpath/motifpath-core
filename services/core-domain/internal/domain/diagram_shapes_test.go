package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/domain"
)

// cagedGrips is the caged-grip family as the drill catalog lists it.
var cagedGrips = domain.DiagramShapeFamily{
	ID: "fam-caged", Key: "caged-grip", Names: domain.LocalizedText{"en": "CAGED grips"},
	Members: []domain.DiagramShapeMember{
		{Shape: "C", Names: domain.LocalizedText{"en": "C shape"}},
		{Shape: "A", Names: domain.LocalizedText{"en": "A shape"}},
		{Shape: "G", Names: domain.LocalizedText{"en": "G shape"}},
		{Shape: "E", Names: domain.LocalizedText{"en": "E shape"}},
		{Shape: "D", Names: domain.LocalizedText{"en": "D shape"}},
	},
}

func TestDiagramShapeItemKey(t *testing.T) {
	assert.Equal(t, "diagram_shape:d-1", domain.DiagramShapeItemKey("d-1"))
}

func TestADiagramShapesReference(t *testing.T) {
	// cAShape is "C major — CAGED A, shift 3", positions in ordinal order.
	cAShape := domain.Diagram{
		ID: "d-ca3", InstrumentID: "guitar", InstrumentIDs: []string{"guitar", "electric"},
		Positions: []domain.Position{
			{ID: "p1", Interval: "R", String: intPtr(5), Fret: intPtr(3)},
			{ID: "p2", Interval: "5", String: intPtr(4), Fret: intPtr(5)},
			{ID: "p3", Interval: "3", String: intPtr(2), Fret: intPtr(5)},
		},
		Shape: &domain.DiagramShape{Family: cagedGrips, Shape: "A"},
	}

	t.Run("carries its layout, family, member, the family's members and its positions in order", func(t *testing.T) {
		got := domain.NewDiagramReference(cAShape)

		assert.Equal(t, &domain.DiagramShapeReference{
			LayoutInstrumentID: "guitar",
			Family:             "caged-grip",
			Shape:              "A",
			FamilyMembers:      []string{"C", "A", "G", "E", "D"},
			Positions: []domain.ShapePosition{
				{String: 5, Fret: 3, Interval: "R"},
				{String: 4, Fret: 5, Interval: "5"},
				{String: 2, Fret: 5, Interval: "3"},
			},
		}, got.Shape)
		assert.Equal(t, []string{"guitar", "electric"}, got.InstrumentIDs)
	})

	t.Run("a diagram that isn't a drill shape has none", func(t *testing.T) {
		plain := cAShape
		plain.Shape = nil

		assert.Nil(t, domain.NewDiagramReference(plain).Shape)
	})
}

func TestADiagramShapesFamilyMember(t *testing.T) {
	t.Run("is the member its shape names", func(t *testing.T) {
		member, ok := domain.DiagramShape{Family: cagedGrips, Shape: "G"}.Member()

		assert.True(t, ok)
		assert.Equal(t, domain.LocalizedText{"en": "G shape"}, member.Names)
	})

	t.Run("is missing for a shape its family doesn't list", func(t *testing.T) {
		_, ok := domain.DiagramShape{Family: cagedGrips, Shape: "3"}.Member()

		assert.False(t, ok)
	})
}
