package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// Generated drill items share one way of being asked next: the way with the
// fewer right answers, the first way on a tie.
func TestNextDrillWay(t *testing.T) {
	cell := domain.FretboardCell{LayoutInstrumentID: guitarLayout, String: 6, Fret: 3}
	shape := domain.DiagramShapeItem{Diagram: domain.Diagram{ID: "d-1", Shape: &domain.DiagramShape{Family: cagedGrips, Shape: "A"}}}
	states := []*domain.PracticeItemState{
		nil,
		{},
		{RightByResponse: map[string]int{"name_the_note": 1, "name_the_shape": 1}},
		{RightByResponse: map[string]int{"name_the_note": 2, "find_the_note": 2, "name_the_shape": 2, "find_the_degree": 2}},
		{RightByResponse: map[string]int{"find_the_note": 3, "find_the_degree": 3}},
		{RightByResponse: map[string]int{"name_the_note": 3, "find_the_note": 1, "name_the_shape": 3, "find_the_degree": 1}},
	}
	for _, state := range states {
		assert.Equal(t, string(domain.NextFretboardDrill(state)), domain.NextDrillWay(cell, state))
		assert.Equal(t, string(domain.NextDiagramShapeDrill(state)), domain.NextDrillWay(shape, state))
	}
}

func TestFretboardCell_AsADrillItem(t *testing.T) {
	cell := domain.FretboardCell{LayoutInstrumentID: guitarLayout, String: 6, Fret: 3}
	window := domain.FretWindow{FromFret: 2}
	planning := domain.DrillPlanning{Pick: func(int) int { return 0 }, Window: &window}

	t.Run("is a cell asked in the head, named or found", func(t *testing.T) {
		assert.Equal(t, domain.FretboardCellItemKey(guitarLayout, 6, 3), cell.ItemKey())
		assert.Equal(t, domain.PracticeItemKindFretboardCell, cell.Kind())
		assert.Equal(t, []string{"name_the_note", "find_the_note"}, cell.Ways())
		assert.Equal(t, domain.FretboardCellSeconds, cell.Seconds())
		assert.True(t, cell.SuitsSession(false))
		assert.False(t, cell.SuitsSession(true))
	})

	t.Run("planned, it is asked the way given within its window", func(t *testing.T) {
		planned := cell.Plan("find_the_note", planning)

		assert.Equal(t, "find_the_note", planned.Way())
		var item domain.PracticeSessionItem
		planned.AttachTo(&item)
		assert.Equal(t, &domain.PlannedFretboardCell{FretboardCell: cell, Drill: domain.FretboardDrillFindTheNote, Window: window}, item.FretboardCell)
	})

	t.Run("named, it shows the cell it gives and nothing else", func(t *testing.T) {
		s := cell.Plan("name_the_note", planning).Stimulus()

		assert.Equal(t, []domain.StimulusPlace{{Place: domain.Place{String: 6, Fret: 3}, Given: true}}, s.Places)
		assert.Empty(t, s.Answer.Places)
		require.NoError(t, domain.CheckChallenge(s))
	})

	t.Run("found, it says the string and draws no place", func(t *testing.T) {
		s := cell.Plan("find_the_note", planning).Stimulus()

		assert.Equal(t, []string{"string 6"}, s.Prompt)
		assert.Empty(t, s.Places)
		assert.Equal(t, []domain.Place{{String: 6, Fret: 3}}, s.Answer.Places)
		require.NoError(t, domain.CheckChallenge(s))
	})
}

func TestDiagramShapeItem_AsADrillItem(t *testing.T) {
	shape := domain.DiagramShapeItem{Diagram: domain.Diagram{
		ID: "d-ca3", InstrumentID: guitarLayout,
		Positions: []domain.Position{
			{ID: "p1", Interval: "R", String: intPtr(5), Fret: intPtr(3)},
			{ID: "p2", Interval: "5", String: intPtr(4), Fret: intPtr(5)},
			{ID: "p3", Interval: "3", String: intPtr(2), Fret: intPtr(5)},
			{ID: "p4", Interval: "R", String: intPtr(3), Fret: intPtr(5)},
		},
		Shape: &domain.DiagramShape{Family: cagedGrips, Shape: "A"},
	}}
	pickLast := domain.DrillPlanning{Pick: func(n int) int { return n - 1 }}

	t.Run("is a shape asked in the head, named or its degree found", func(t *testing.T) {
		assert.Equal(t, domain.DiagramShapeItemKey("d-ca3"), shape.ItemKey())
		assert.Equal(t, domain.PracticeItemKindDiagramShape, shape.Kind())
		assert.Equal(t, []string{"name_the_shape", "find_the_degree"}, shape.Ways())
		assert.Equal(t, domain.DiagramShapeSeconds, shape.Seconds())
		assert.True(t, shape.SuitsSession(false))
		assert.False(t, shape.SuitsSession(true))
	})

	t.Run("planned, it is the shape asked the way given", func(t *testing.T) {
		planned := shape.Plan("find_the_degree", pickLast)

		var item domain.PracticeSessionItem
		planned.AttachTo(&item)
		want := domain.PlanDiagramShape(shape.Diagram, domain.DiagramShapeDrillFindTheDegree, pickLast.Pick)
		assert.Equal(t, want.AskedInterval, item.DiagramShape.AskedInterval)
		assert.Equal(t, want.Drill, item.DiagramShape.Drill)
		assert.Equal(t, "find_the_degree", planned.Way())
	})

	t.Run("finding a degree labels only the roots it gives, and draws the rest alike", func(t *testing.T) {
		s := shape.Plan("find_the_degree", pickLast).Stimulus()

		assert.Equal(t, []string{"3"}, s.Prompt)
		assert.Equal(t, []domain.StimulusPlace{
			{Place: domain.Place{String: 5, Fret: 3}, Mark: domain.PlaceMark{Label: "R"}, Given: true},
			{Place: domain.Place{String: 4, Fret: 5}},
			{Place: domain.Place{String: 2, Fret: 5}},
			{Place: domain.Place{String: 3, Fret: 5}, Mark: domain.PlaceMark{Label: "R"}, Given: true},
		}, s.Places)
		assert.Equal(t, []domain.Place{{String: 2, Fret: 5}}, s.Answer.Places)
		require.NoError(t, domain.CheckChallenge(s))
	})

	t.Run("naming it draws every place alike and offers its family's members", func(t *testing.T) {
		s := shape.Plan("name_the_shape", pickLast).Stimulus()

		for _, p := range s.Places {
			assert.Equal(t, domain.PlaceMark{}, p.Mark)
			assert.False(t, p.Given)
		}
		require.Len(t, s.Options, 5)
		assert.Equal(t, domain.StimulusOption{Key: "A", Labels: []string{"A shape"}}, s.Options[1])
		assert.Equal(t, domain.DrillAnswer{Options: []string{"A"}, Labels: []string{"A shape"}}, s.Answer)
		require.NoError(t, domain.CheckChallenge(s))
	})

	t.Run("a shape of roots and one degree marks that degree by drawing it", func(t *testing.T) {
		dyad := domain.DiagramShapeItem{Diagram: domain.Diagram{
			ID: "d-power", InstrumentID: guitarLayout,
			Positions: []domain.Position{
				{ID: "p1", Interval: "R", String: intPtr(6), Fret: intPtr(3)},
				{ID: "p2", Interval: "5", String: intPtr(5), Fret: intPtr(5)},
			},
			Shape: &domain.DiagramShape{Family: cagedGrips, Shape: "G"},
		}}

		assert.ErrorIs(t, domain.CheckChallenge(dyad.Plan("find_the_degree", pickLast).Stimulus()), domain.ErrMarkedAnswer)
	})
}
