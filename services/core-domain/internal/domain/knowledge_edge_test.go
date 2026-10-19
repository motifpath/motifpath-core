package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func masteryLevel(l domain.MasteryLevel) *domain.MasteryLevel { return &l }

func TestNewKnowledgeEdge(t *testing.T) {
	skill := domain.KnowledgeNode{ID: "skill-1", Kind: domain.KnowledgeNodeKindSkill}
	otherSkill := domain.KnowledgeNode{ID: "skill-2", Kind: domain.KnowledgeNodeKindSkill}
	concept := domain.KnowledgeNode{ID: "concept-1", Kind: domain.KnowledgeNodeKindConcept}
	otherConcept := domain.KnowledgeNode{ID: "concept-2", Kind: domain.KnowledgeNodeKindConcept}

	t.Run("a skill applies a concept, with no level", func(t *testing.T) {
		edge, err := domain.NewKnowledgeEdge("edge-1", skill, concept, domain.KnowledgeEdgeTypeApplies, nil)

		require.NoError(t, err)
		assert.Equal(t, "edge-1", edge.ID)
		assert.Equal(t, "skill-1", edge.FromID)
		assert.Equal(t, "concept-1", edge.ToID)
		assert.Equal(t, domain.KnowledgeEdgeTypeApplies, edge.Type)
		assert.Nil(t, edge.Level)
	})

	for _, pair := range []struct {
		name     string
		from, to domain.KnowledgeNode
	}{
		{"skill requires skill", skill, otherSkill},
		{"skill requires concept", skill, concept},
		{"concept requires concept", concept, otherConcept},
		{"concept requires skill", concept, skill},
	} {
		t.Run(pair.name+" at a level", func(t *testing.T) {
			edge, err := domain.NewKnowledgeEdge("edge-1", pair.from, pair.to, domain.KnowledgeEdgeTypeRequires, masteryLevel(domain.MasteryLevelFluent))

			require.NoError(t, err)
			require.NotNil(t, edge.Level)
			assert.Equal(t, domain.MasteryLevelFluent, *edge.Level)
		})
	}

	tests := []struct {
		name      string
		from, to  domain.KnowledgeNode
		edgeType  domain.KnowledgeEdgeType
		level     *domain.MasteryLevel
		wantField string
	}{
		{name: "an unknown type", from: skill, to: concept, edgeType: "prerequisite_of", wantField: "type"},
		{name: "applies from a concept to a skill", from: concept, to: skill, edgeType: domain.KnowledgeEdgeTypeApplies, wantField: "type"},
		{name: "applies between two skills", from: skill, to: otherSkill, edgeType: domain.KnowledgeEdgeTypeApplies, wantField: "type"},
		{name: "applies between two concepts", from: concept, to: otherConcept, edgeType: domain.KnowledgeEdgeTypeApplies, wantField: "type"},
		{name: "applies with a level", from: skill, to: concept, edgeType: domain.KnowledgeEdgeTypeApplies, level: masteryLevel(domain.MasteryLevelFluent), wantField: "level"},
		{name: "requires without a level", from: skill, to: otherSkill, edgeType: domain.KnowledgeEdgeTypeRequires, wantField: "level"},
		{name: "requires with an unknown level", from: skill, to: otherSkill, edgeType: domain.KnowledgeEdgeTypeRequires, level: masteryLevel("expert"), wantField: "level"},
		{name: "a node linked to itself", from: skill, to: skill, edgeType: domain.KnowledgeEdgeTypeRequires, level: masteryLevel(domain.MasteryLevelAccurate), wantField: "to_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" is rejected", func(t *testing.T) {
			_, err := domain.NewKnowledgeEdge("edge-1", tt.from, tt.to, tt.edgeType, tt.level)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			require.Len(t, valErr.Fields, 1)
			assert.Equal(t, tt.wantField, valErr.Fields[0].Field)
		})
	}
}

func TestKnowledgeEdgeWithLevel(t *testing.T) {
	t.Run("a requires edge takes a new level", func(t *testing.T) {
		edge := domain.KnowledgeEdge{ID: "edge-1", Type: domain.KnowledgeEdgeTypeRequires, Level: masteryLevel(domain.MasteryLevelAccurate)}

		got, err := edge.WithLevel(domain.MasteryLevelRetained)

		require.NoError(t, err)
		require.NotNil(t, got.Level)
		assert.Equal(t, domain.MasteryLevelRetained, *got.Level)
	})

	t.Run("an applies edge cannot take a level", func(t *testing.T) {
		edge := domain.KnowledgeEdge{ID: "edge-1", Type: domain.KnowledgeEdgeTypeApplies}

		_, err := edge.WithLevel(domain.MasteryLevelFluent)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "level", valErr.Fields[0].Field)
	})

	t.Run("an unknown level is rejected", func(t *testing.T) {
		edge := domain.KnowledgeEdge{ID: "edge-1", Type: domain.KnowledgeEdgeTypeRequires, Level: masteryLevel(domain.MasteryLevelAccurate)}

		_, err := edge.WithLevel("expert")

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "level", valErr.Fields[0].Field)
	})
}
