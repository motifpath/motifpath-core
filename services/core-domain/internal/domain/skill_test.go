package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestNewSkill(t *testing.T) {
	t.Run("a root skill has a nil parent", func(t *testing.T) {
		skill, err := domain.NewSkill("skill-1", "guitar-technique", nil)

		require.NoError(t, err)
		assert.Equal(t, "skill-1", skill.ID)
		assert.Equal(t, "guitar-technique", skill.Name)
		assert.Nil(t, skill.ParentID)
	})

	t.Run("a child skill carries its parent id", func(t *testing.T) {
		parentID := "skill-1"
		skill, err := domain.NewSkill("skill-2", "right-hand-technique", &parentID)

		require.NoError(t, err)
		require.NotNil(t, skill.ParentID)
		assert.Equal(t, parentID, *skill.ParentID)
	})

	t.Run("an empty name is rejected", func(t *testing.T) {
		_, err := domain.NewSkill("skill-1", "", nil)

		require.Error(t, err)
		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "name", valErr.Fields[0].Field)
	})
}
