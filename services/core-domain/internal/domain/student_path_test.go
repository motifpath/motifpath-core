package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func twoNodeItems() []domain.LearningPathItem {
	return []domain.LearningPathItem{
		{Position: 1, ContentNodeID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo},
		{Position: 2, ContentNodeID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo},
	}
}

func TestBuildStudentPathItems_LanguageLocking(t *testing.T) {
	t.Run("prerequisite met and language available leaves the item unlocked", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string]bool{})

		assert.Equal(t, domain.CompletionStatusNotStarted, items[0].Status)
	})

	t.Run("prerequisite met but language missing locks the item", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string]bool{"node-01": true})

		assert.Equal(t, domain.CompletionStatusLocked, items[0].Status)
	})

	t.Run("prerequisite unmet but language available still locks the item", func(t *testing.T) {
		raw := map[string]domain.CompletionStatus{"node-01": domain.CompletionStatusNotStarted}
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), raw, map[string]bool{})

		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
	})

	t.Run("language lock on the first item cascades to lock the rest of the path", func(t *testing.T) {
		items, current := domain.BuildStudentPathItems(twoNodeItems(), nil, map[string]bool{"node-01": true})

		require.Len(t, items, 2)
		assert.Equal(t, domain.CompletionStatusLocked, items[0].Status)
		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
		assert.Equal(t, 1, current)
	})

	t.Run("an item with no langLocked entry is never locked for language reasons", func(t *testing.T) {
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), nil, nil)

		assert.Equal(t, domain.CompletionStatusNotStarted, items[0].Status)
	})

	t.Run("a completed item stays completed even if later marked language-locked in the map", func(t *testing.T) {
		raw := map[string]domain.CompletionStatus{"node-01": domain.CompletionStatusCompleted}
		items, _ := domain.BuildStudentPathItems(twoNodeItems(), raw, map[string]bool{"node-02": true})

		assert.Equal(t, domain.CompletionStatusCompleted, items[0].Status)
		assert.Equal(t, domain.CompletionStatusLocked, items[1].Status)
	})
}
