package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestToGeneratedPracticeSessionPlan_RungAndFocus(t *testing.T) {
	sessionID := "4f1c0a8e-6a52-4b8e-9d0e-2c6f2b8f9a11"
	nodeID := "0f8d7c6b-5a49-4382-9170-6e5d4c3b2a19"
	plan := domain.PracticeSessionPlan{
		ID: sessionID, Minutes: 10, FocusSource: domain.PracticeFocusSuggested,
		FocusNodes: []domain.KnowledgeNode{{ID: nodeID, Names: domain.LocalizedText{"en": "Diatonic chords", "pt_BR": "Acordes diatônicos"}}},
		Items: []domain.PracticeSessionItem{{
			ItemKey: "exercise:x", Kind: domain.PracticeItemKindExercise, Reason: domain.PracticePickNew,
			Rung: domain.PracticeRungHear, Level: domain.KnowledgeLevelNew, EstimatedSeconds: 30,
		}},
	}

	got := toGeneratedPracticeSessionPlan(plan, nil, "en")

	assert.Equal(t, generated.PracticeFocusSource("suggested"), got.FocusSource)
	require.Len(t, got.FocusNodes, 1)
	assert.Equal(t, nodeID, got.FocusNodes[0].NodeId.String())
	assert.Equal(t, generated.LocalizedNames{"en": "Diatonic chords", "pt_BR": "Acordes diatônicos"}, got.FocusNodes[0].Names)
	require.Len(t, got.Items, 1)
	assert.Equal(t, generated.PracticeRung("hear"), got.Items[0].Rung)
}
