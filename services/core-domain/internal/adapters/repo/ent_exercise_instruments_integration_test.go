//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// instrumentExercise is a minimal text_response exercise for instrumentIDs
// (none meaning every instrument), classified under skill.
func instrumentExercise(title string, skill domain.KnowledgeNode, instrumentIDs ...string) domain.Exercise {
	return domain.Exercise{
		ID: uuid.NewString(), Title: title, Prompt: domain.NewPlainTextPrompt("Name it"), ExerciseType: domain.ExerciseTypeTextResponse,
		Skills: []domain.KnowledgeNode{skill}, Options: []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: strPtr("A")}},
		ChallengeIDs: []string{}, ContentNodeIDs: []string{}, Languages: []domain.Language{{Code: "en"}},
		InstrumentIDs: instrumentIDs, CreatedAt: fixedAt,
	}
}

func TestEntExerciseInstruments(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	exercises := NewEntExerciseRepository(client)
	nodes := NewEntContentNodeRepository(client)
	challenges := NewEntChallengeRepository(client)
	knowledge := NewEntKnowledgeNodeRepository(client)
	instrumentRepo := NewEntInstrumentRepository(client)
	guitar, piano := frettedInstrument(), keyboardInstrument()
	require.NoError(t, instrumentRepo.Create(ctx, guitar))
	require.NoError(t, instrumentRepo.Create(ctx, piano))

	skill := seedSkill(t, ctx, client, "voicings-"+uuid.NewString())
	guitarDrill := instrumentExercise("Guitar drill", skill, guitar.ID)
	anyDrill := instrumentExercise("Any-instrument drill", skill)
	pianoDrill := instrumentExercise("Piano drill", skill, piano.ID)
	for _, e := range []domain.Exercise{guitarDrill, anyDrill, pianoDrill} {
		require.NoError(t, exercises.Create(ctx, e))
	}

	node := seedContentNode(t, ctx, nodes)
	challenge := domain.Challenge{ID: uuid.NewString(), ContentNodeID: node.ID, SubjectSkillID: strPtr(uuid.NewString()), PassThreshold: 70, CreatedAt: fixedAt}
	require.NoError(t, challenges.Create(ctx, challenge))
	require.NoError(t, exercises.LinkChallenge(ctx, guitarDrill.ID, challenge.ID))
	require.NoError(t, exercises.LinkContentNode(ctx, anyDrill.ID, node.ID))

	t.Run("LinkedExerciseInstrumentSets lists exercises linked through a challenge or as path exercises", func(t *testing.T) {
		got, err := nodes.LinkedExerciseInstrumentSets(ctx, node.ID)

		require.NoError(t, err)
		assert.ElementsMatch(t, [][]string{{guitar.ID}, nil}, got)
	})

	t.Run("LinkedExerciseInstrumentSets is empty for a node with no exercises", func(t *testing.T) {
		got, err := nodes.LinkedExerciseInstrumentSets(ctx, seedContentNode(t, ctx, nodes).ID)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("ClassifiedInstrumentSets counts exercises classified under the node", func(t *testing.T) {
		got, err := knowledge.ClassifiedInstrumentSets(ctx, skill.ID)

		require.NoError(t, err)
		assert.ElementsMatch(t, [][]string{{guitar.ID}, nil, {piano.ID}}, got)
	})

	t.Run("the instrument filter keeps exercises for any listed instrument or for every instrument", func(t *testing.T) {
		got, err := exercises.List(ctx, domain.ExerciseFilter{InstrumentIDs: []string{guitar.ID}}, domain.PageRequest{Limit: domain.MaxPageLimit})

		require.NoError(t, err)
		exerciseID := func(e domain.Exercise) string { return e.ID }
		assert.ElementsMatch(t, titlesOf([]domain.Exercise{guitarDrill, anyDrill}, exerciseID), titlesOf(got.Items, exerciseID))
		assert.Equal(t, 2, got.Total)

		got, err = exercises.List(ctx, domain.ExerciseFilter{InstrumentIDs: []string{guitar.ID, piano.ID}}, domain.PageRequest{Limit: domain.MaxPageLimit})

		require.NoError(t, err)
		assert.Equal(t, 3, got.Total)
	})
}
