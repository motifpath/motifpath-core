package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestPracticeSessionService_ComposePlan_RungsAndFocus(t *testing.T) {
	ctx := context.Background()

	t.Run("every item in the head carries its drill template's rung", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.due(f.cellsOn(practiceGuitar, "skill-cells", 3, 6)...)
		f.shapeOn("shape-1", "skill-shapes", practiceGuitar)
		seconds := 30
		f.skill("skill-ears")
		require.NoError(t, f.exercises.Create(ctx, domain.Exercise{
			ID: "ex-ear", Title: "ex-ear", ExerciseType: domain.ExerciseTypeAudioRecognition,
			Skills: []domain.KnowledgeNode{{ID: "skill-ears"}}, EstimatedDurationSeconds: &seconds,
		}))
		f.onPathFor([]string{practiceGuitar}, "skill-cells", "skill-shapes", "skill-ears")

		plan := f.composeInTheHead(t, 10)

		want := map[domain.PracticeItemKind]domain.PracticeRung{
			domain.PracticeItemKindFretboardCell: domain.PracticeRungKnow,
			domain.PracticeItemKindDiagramShape:  domain.PracticeRungKnow,
			domain.PracticeItemKindExercise:      domain.PracticeRungHear,
		}
		seen := map[domain.PracticeItemKind]bool{}
		for _, item := range plan.Items {
			assert.Equal(t, want[item.Kind], item.Rung, item.ItemKey)
			seen[item.Kind] = true
		}
		assert.Len(t, seen, 3, "the session asks a cell, a shape and an exercise")
	})

	t.Run("a play-along is played in time", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.playAlong("riff", "Riff", practiceGuitar, "skill-riffs", 80)
		f.onPath("skill-riffs")

		plan := f.compose(t, practiceGuitar, 10)

		require.NotEmpty(t, itemsOfKind(plan, domain.PracticeItemKindPlayAlong))
		for _, item := range itemsOfKind(plan, domain.PracticeItemKindPlayAlong) {
			assert.Equal(t, domain.PracticeRungPlayInTime, item.Rung)
		}
	})

	t.Run("Today's mix is suggested, and names at most three of its items' nodes, the most practised first", func(t *testing.T) {
		f := newPracticeFixture(t)
		for _, skill := range []string{"skill-a", "skill-b", "skill-c", "skill-d"} {
			f.due(f.exerciseBatch(skill, skill, 4, 20)...)
		}
		f.due(f.exerciseBatch("skill-b-more", "skill-b", 4, 20)...)
		f.onPath("skill-a", "skill-b", "skill-c", "skill-d")

		plan := f.compose(t, practiceGuitar, 10)

		assert.Equal(t, domain.PracticeFocusSuggested, plan.FocusSource)
		require.NotEmpty(t, plan.FocusNodes)
		assert.LessOrEqual(t, len(plan.FocusNodes), domain.MaxFocusNodes)
		secondsByNode := map[string]int{}
		for _, item := range plan.Items {
			require.NotNil(t, item.NodeID, item.ItemKey)
			secondsByNode[*item.NodeID] += item.EstimatedSeconds
		}
		assert.Equal(t, "skill-b", plan.FocusNodes[0].ID)
		for _, n := range plan.FocusNodes {
			assert.Contains(t, secondsByNode, n.ID)
			assert.LessOrEqual(t, secondsByNode[n.ID], secondsByNode[plan.FocusNodes[0].ID])
		}
	})

	t.Run("a session the student chooses isn't composed yet", func(t *testing.T) {
		f := newPracticeFixture(t)
		f.exercise("ex-1", "skill-a", 30)
		f.onPath("skill-a")
		guitar, node := practiceGuitar, "skill-a"

		_, err := f.svc.ComposePlan(ctx, studentCaller(), &guitar, &node, 10)

		var validation *domain.ValidationError
		require.ErrorAs(t, err, &validation)
		require.Len(t, validation.Fields, 1)
		assert.Equal(t, "focus_node_id", validation.Fields[0].Field)
	})
}
