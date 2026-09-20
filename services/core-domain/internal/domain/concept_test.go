package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestNewConcept(t *testing.T) {
	t.Run("a root concept has a nil parent", func(t *testing.T) {
		concept, err := domain.NewConcept("concept-1", "music-theory", nil)

		require.NoError(t, err)
		assert.Equal(t, "concept-1", concept.ID)
		assert.Equal(t, "music-theory", concept.Name)
		assert.Nil(t, concept.ParentID)
	})

	t.Run("a child concept carries its parent id", func(t *testing.T) {
		parentID := "concept-1"
		concept, err := domain.NewConcept("concept-2", "chord-theory", &parentID)

		require.NoError(t, err)
		require.NotNil(t, concept.ParentID)
		assert.Equal(t, parentID, *concept.ParentID)
	})

	t.Run("an empty name is rejected", func(t *testing.T) {
		_, err := domain.NewConcept("concept-1", "", nil)

		require.Error(t, err)
		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "name", valErr.Fields[0].Field)
	})
}
