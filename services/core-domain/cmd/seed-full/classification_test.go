package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

type keyedNodes map[string]domain.KnowledgeNode

func (k keyedNodes) GetByKeys(_ context.Context, keys []string) (map[string]domain.KnowledgeNode, error) {
	found := map[string]domain.KnowledgeNode{}
	for _, key := range keys {
		if n, ok := k[key]; ok {
			found[key] = n
		}
	}
	return found, nil
}

func TestClassificationSeederResolvesMapKeys(t *testing.T) {
	ctx := context.Background()
	c := &classificationSeeder{nodes: keyedNodes{
		"play-open-chords":  {ID: "s1", Kind: domain.KnowledgeNodeKindSkill, Key: "play-open-chords"},
		"open-chord-shapes": {ID: "c1", Kind: domain.KnowledgeNodeKindConcept, Key: "open-chord-shapes"},
	}}

	skillID, err := c.skillID(ctx, "play-open-chords")
	require.NoError(t, err)
	assert.Equal(t, "s1", skillID)
	conceptID, err := c.conceptID(ctx, "open-chord-shapes")
	require.NoError(t, err)
	assert.Equal(t, "c1", conceptID)

	_, err = c.skillID(ctx, "open-chord-shapes")
	assert.ErrorContains(t, err, "no skill")
	_, err = c.conceptID(ctx, "never-installed")
	assert.ErrorContains(t, err, "no concept")
}
