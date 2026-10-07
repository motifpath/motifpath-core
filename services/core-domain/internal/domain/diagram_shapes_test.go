package domain_test

import (
	"testing"
	"time"

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

func TestNextDiagramShapeDrill(t *testing.T) {
	tests := []struct {
		name  string
		state *domain.PracticeItemState
		want  domain.DiagramShapeDrill
	}{
		{"a shape never practised is named", nil, domain.DiagramShapeDrillNameTheShape},
		{"a shape never answered right is named", &domain.PracticeItemState{}, domain.DiagramShapeDrillNameTheShape},
		{"named more often than its degrees were found, its degrees are found next",
			&domain.PracticeItemState{RightByResponse: map[string]int{"name_the_shape": 3, "find_the_degree": 1}}, domain.DiagramShapeDrillFindTheDegree},
		{"a tie is named", &domain.PracticeItemState{RightByResponse: map[string]int{"name_the_shape": 2, "find_the_degree": 2}}, domain.DiagramShapeDrillNameTheShape},
		{"found more often than named, it is named next",
			&domain.PracticeItemState{RightByResponse: map[string]int{"name_the_shape": 1, "find_the_degree": 2}}, domain.DiagramShapeDrillNameTheShape},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.NextDiagramShapeDrill(tt.state))
		})
	}
}

func TestPlanDiagramShape(t *testing.T) {
	// aMinorBox1 is "A minor pentatonic — Box 1, fret 5", its first string
	// only, the root twice.
	aMinorBox1 := domain.Diagram{
		ID: "d-am1", InstrumentID: "guitar",
		Positions: []domain.Position{
			{Interval: "R", String: intPtr(6), Fret: intPtr(5)},
			{Interval: "b3", String: intPtr(6), Fret: intPtr(8)},
			{Interval: "4", String: intPtr(5), Fret: intPtr(5)},
			{Interval: "5", String: intPtr(5), Fret: intPtr(7)},
			{Interval: "b7", String: intPtr(4), Fret: intPtr(5)},
			{Interval: "R", String: intPtr(4), Fret: intPtr(7)},
		},
		Shape: &domain.DiagramShape{Family: cagedGrips, Shape: "A"},
	}
	first := func(int) int { return 0 }
	last := func(n int) int { return n - 1 }

	t.Run("naming the shape offers every member of its family, in order, and asks no degree", func(t *testing.T) {
		got := domain.PlanDiagramShape(aMinorBox1, domain.DiagramShapeDrillNameTheShape, first)

		assert.Equal(t, domain.PlannedDiagramShape{
			DiagramID: "d-am1", Drill: domain.DiagramShapeDrillNameTheShape, Family: "caged-grip", Options: cagedGrips.Members,
		}, got)
	})

	t.Run("finding a degree asks one of the shape's degrees other than its root, and offers no option", func(t *testing.T) {
		firstDegree := domain.PlanDiagramShape(aMinorBox1, domain.DiagramShapeDrillFindTheDegree, first)
		lastDegree := domain.PlanDiagramShape(aMinorBox1, domain.DiagramShapeDrillFindTheDegree, last)

		assert.Equal(t, domain.PlannedDiagramShape{
			DiagramID: "d-am1", Drill: domain.DiagramShapeDrillFindTheDegree, Family: "caged-grip", Options: []domain.DiagramShapeMember{}, AskedInterval: strPtr("b3"),
		}, firstDegree)
		assert.Equal(t, strPtr("b7"), lastDegree.AskedInterval)
	})

	t.Run("each degree is offered once however often the shape repeats it", func(t *testing.T) {
		var asked []string
		for i := range 4 {
			got := domain.PlanDiagramShape(aMinorBox1, domain.DiagramShapeDrillFindTheDegree, func(n int) int {
				assert.Equal(t, 4, n, "b3, 4, 5 and b7")
				return i
			})
			asked = append(asked, *got.AskedInterval)
		}
		assert.Equal(t, []string{"b3", "4", "5", "b7"}, asked)
	})

	t.Run("a shape with nothing but roots is named instead", func(t *testing.T) {
		roots := aMinorBox1
		roots.Positions = []domain.Position{{Interval: "R", String: intPtr(6), Fret: intPtr(5)}}

		got := domain.PlanDiagramShape(roots, domain.DiagramShapeDrillFindTheDegree, first)

		assert.Equal(t, domain.DiagramShapeDrillNameTheShape, got.Drill)
		assert.Nil(t, got.AskedInterval)
	})
}

func TestADiagramShapeItemsDrillTemplate(t *testing.T) {
	item := domain.PracticeSessionItem{Kind: domain.PracticeItemKindDiagramShape, DiagramShape: &domain.PlannedDiagramShape{Drill: domain.DiagramShapeDrillFindTheDegree}}

	assert.Equal(t, "diagram_shape:find_the_degree", item.DrillTemplateKey())
}

func TestAPlanOfDiagramShapesAsksForATapCheck(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	shapes := []domain.PracticeSessionItem{{Kind: domain.PracticeItemKindDiagramShape, DiagramShape: &domain.PlannedDiagramShape{}}}

	assert.True(t, domain.TapCheckDue(shapes, nil, now))
}
