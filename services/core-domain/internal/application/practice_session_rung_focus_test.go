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
			Skills: []domain.KnowledgeNode{{ID: "skill-ears"}}, EstimatedDurationSeconds: &seconds, Languages: anyLanguage(),
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
}
