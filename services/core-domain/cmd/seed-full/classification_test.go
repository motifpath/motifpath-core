package main

import (
	"context"
	"slices"
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

func TestSeededExercisesAreClassifiedByWhatTheyAsk(t *testing.T) {
	got := map[string]exerciseClassification{}
	for _, s := range exerciseTypeSpecs("https://example.test/image.png", "https://example.test/audio.mp3") {
		got[s.title] = s.classification
	}
	got[listeningTitle] = listeningClassification

	assert.Equal(t, map[string]exerciseClassification{
		"Name the interval — text response":                     {skill: "hear-intervals", concept: "interval-names"},
		"Identify the recorded interval":                        {skill: "hear-intervals", concept: "interval-names"},
		"Tap the root note on the fretboard":                    {skill: "find-notes", concept: "root-note"},
		"Pick the matching chord shape":                         {skill: "play-open-chords", concept: "open-chord-shapes"},
		"Pick the matching recorded lick":                       {skill: "hear-licks", concept: "lick"},
		"Listen to the E major chord, then tap its major third": {skill: "hear-chord-quality", concept: "major-triads"},
	}, got)
}

func TestAdminPathTeachesFindingNotesOnTheFretboard(t *testing.T) {
	skills := map[string]string{}
	for _, s := range contentNodeSpecs() {
		skills[s.key] = s.skill
	}

	var taught []string
	for _, key := range adminPathNodeKeys {
		if slices.Contains(articleLessonKeys, key) {
			continue
		}
		skill, ok := skills[key]
		require.True(t, ok, "admin path item %q is not a seeded content node", key)
		taught = append(taught, skill)
	}
	// A session in the head asks fretboard cells only for the skills a path teaches.
	assert.Contains(t, taught, "find-notes-root-strings")
	assert.Contains(t, taught, "find-notes-top-strings")
	assert.Equal(t, []string{acousticGuitarID}, adminPathInstrumentIDs)
}

func TestAdminPathTeachesDiagramShapes(t *testing.T) {
	skills := map[string]string{}
	for _, s := range contentNodeSpecs() {
		skills[s.key] = s.skill
	}

	var taught []string
	for _, key := range adminPathNodeKeys {
		taught = append(taught, skills[key])
	}
	// A session asks diagram shapes only for the skills a path teaches: the CAGED grips and the
	// pentatonic boxes, so both a five-member and a numbered family can be named.
	assert.Contains(t, taught, "map-fretboard-caged")
	assert.Contains(t, taught, "play-pentatonic-positions")
}
