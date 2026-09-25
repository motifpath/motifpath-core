package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestLearningPathLibraryMapping(t *testing.T) {
	level := domain.DifficultyLevelIntermediate
	updated := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	path := domain.LearningPath{ID: uuid.NewString(), TeacherID: uuid.NewString(), Title: "Open Chords", Level: &level, CreatedAt: updated.Add(-time.Hour), UpdatedAt: updated}

	t.Run("a path's level and last update reach the response", func(t *testing.T) {
		got := toLearningPath(path, userNames{})

		if assert.NotNil(t, got.Level) {
			assert.Equal(t, generated.LearningPathLevel("intermediate"), *got.Level)
		}
		assert.Equal(t, updated, got.UpdatedAt)
	})

	t.Run("a path with no level recorded has none in the response", func(t *testing.T) {
		legacy := path
		legacy.Level = nil

		assert.Nil(t, toLearningPath(legacy, userNames{}).Level)
	})

	t.Run("every list parameter maps onto the filter", func(t *testing.T) {
		creator, skill, concept := uuid.New(), uuid.New(), uuid.New()
		levels := []generated.ListLearningPathsParamsLevels{"beginner", "expert"}
		sort := generated.ListLearningPathsParamsSort("updated")
		q := "chords"

		got := learningPathListFilter(generated.ListLearningPathsParams{
			Q: &q, CreatedBy: &creator, Levels: &levels, SkillIds: &[]uuid.UUID{skill}, ConceptIds: &[]uuid.UUID{concept}, Sort: &sort,
		})

		assert.Equal(t, domain.LearningPathFilter{
			Query: "chords", CreatedBy: creator.String(), Levels: []domain.DifficultyLevel{"beginner", "expert"},
			SkillIDs: []string{skill.String()}, ConceptIDs: []string{concept.String()}, Sort: domain.LearningPathSortUpdated,
		}, got)
	})

	t.Run("no parameters leave the filter empty", func(t *testing.T) {
		assert.Equal(t, domain.LearningPathFilter{}, learningPathListFilter(generated.ListLearningPathsParams{}))
	})
}
