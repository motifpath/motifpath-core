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

func TestToGeneratedPracticeSubjectList(t *testing.T) {
	instrumentID := "6f1c0a8e-6a52-4b8e-9d0e-2c6f2b8f9a11"
	areaID := "0f8d7c6b-5a49-4382-9170-6e5d4c3b2a19"
	nodeID := "1f8d7c6b-5a49-4382-9170-6e5d4c3b2a19"
	learning := domain.KnowledgeLevelLearning
	list := domain.PracticeSubjectList{
		InstrumentID: &instrumentID,
		Groups: []domain.PracticeSubjectGroup{{
			Area: domain.KnowledgeNode{ID: areaID, Names: domain.LocalizedText{"en": "Harmony"}},
			Subjects: []domain.PracticeSubject{
				{Node: domain.KnowledgeNode{ID: areaID, Names: domain.LocalizedText{"en": "Harmony"}}, ItemCount: 12, CurrentRung: domain.PracticeRungKnow},
				{Node: domain.KnowledgeNode{ID: nodeID, Names: domain.LocalizedText{"en": "Progressions"}}, OnPath: true, ItemCount: 4, Level: &learning, CurrentRung: domain.PracticeRungHear},
			},
		}},
	}

	got := toGeneratedPracticeSubjectList(list)

	require.NotNil(t, got.InstrumentId)
	assert.Equal(t, instrumentID, got.InstrumentId.String())
	require.Len(t, got.Groups, 1)
	assert.Equal(t, areaID, got.Groups[0].AreaNodeId.String())
	assert.Equal(t, generated.LocalizedNames{"en": "Harmony"}, got.Groups[0].Names)
	require.Len(t, got.Groups[0].Subjects, 2)
	area, progressions := got.Groups[0].Subjects[0], got.Groups[0].Subjects[1]
	assert.Nil(t, area.Level, "a wide node has no level")
	assert.Equal(t, 12, area.ItemCount)
	assert.False(t, area.OnPath)
	assert.Equal(t, nodeID, progressions.NodeId.String())
	assert.True(t, progressions.OnPath)
	assert.Equal(t, generated.PracticeRung("hear"), progressions.CurrentRung)
	require.NotNil(t, progressions.Level)
	assert.Equal(t, generated.KnowledgeLevel("learning"), *progressions.Level)
}

func TestToGeneratedPracticeSubjectList_InTheHeadHasNoInstrumentAndEmptyGroups(t *testing.T) {
	got := toGeneratedPracticeSubjectList(domain.PracticeSubjectList{Groups: []domain.PracticeSubjectGroup{}})

	assert.Nil(t, got.InstrumentId)
	assert.NotNil(t, got.Groups, "an empty list encodes as [], never null")
}
