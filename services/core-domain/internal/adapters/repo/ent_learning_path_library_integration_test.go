//go:build integration

package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntLearningPathRepository_Library(t *testing.T) {
	f := newCourseListFixture(t)
	fingerpicking := seedSkill(t, f.ctx, f.nodes.client, "fingerpicking-"+uuid.NewString())
	syncopation := seedConcept(t, f.ctx, f.nodes.client, "syncopation-"+uuid.NewString())
	bob, carol := uuid.NewString(), uuid.NewString()
	levelOf := func(l domain.DifficultyLevel) *domain.DifficultyLevel { return &l }
	path := func(title, teacher string, level *domain.DifficultyLevel, updated time.Time, skills []domain.Skill, concepts []domain.Concept) domain.LearningPath {
		node := domain.ContentNode{
			ID: uuid.NewString(), TeacherID: teacher, Title: "Node " + uuid.NewString(), ContentType: domain.ContentTypeVideo,
			Classification: domain.Classification{Skills: skills, Concepts: concepts, DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
			CreatedAt:      fixedAt,
		}
		require.NoError(t, f.nodes.Create(f.ctx, node))
		created := domain.LearningPath{
			ID: uuid.NewString(), TeacherID: teacher, Title: title, Level: level, CreatedAt: fixedAt, UpdatedAt: updated,
			Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: node.ID, Title: node.Title, ContentType: node.ContentType}},
		}
		require.NoError(t, f.paths.Create(f.ctx, created))
		return created
	}
	day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }
	alpha := path("Alpha", bob, levelOf(domain.DifficultyLevelBeginner), day(1), []domain.Skill{fingerpicking}, []domain.Concept{syncopation})
	beta := path("Beta", carol, levelOf(domain.DifficultyLevelIntermediate), day(20), []domain.Skill{fingerpicking}, nil)
	legacy := path("Gamma legacy", bob, nil, day(10), nil, nil)
	ids := func(filter domain.LearningPathFilter) []string {
		page, err := f.paths.List(f.ctx, filter, domain.PageRequest{Limit: 50})
		require.NoError(t, err)
		out := make([]string, len(page.Items))
		for i, p := range page.Items {
			out[i] = p.ID
		}
		return out
	}

	t.Run("a path's level and last update round-trip, a legacy path having no level", func(t *testing.T) {
		got, err := f.paths.GetByID(f.ctx, beta.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Level)
		assert.Equal(t, domain.DifficultyLevelIntermediate, *got.Level)
		assert.True(t, got.UpdatedAt.Equal(day(20)))

		old, err := f.paths.GetByID(f.ctx, legacy.ID)
		require.NoError(t, err)
		assert.Nil(t, old.Level)
	})

	t.Run("filters by creator, by any of several levels, and never matches a legacy path on level", func(t *testing.T) {
		assert.ElementsMatch(t, []string{alpha.ID, legacy.ID}, ids(domain.LearningPathFilter{CreatedBy: bob}))
		assert.ElementsMatch(t, []string{alpha.ID, beta.ID}, ids(domain.LearningPathFilter{Levels: []domain.DifficultyLevel{domain.DifficultyLevelBeginner, domain.DifficultyLevelIntermediate}}))
		assert.Empty(t, ids(domain.LearningPathFilter{CreatedBy: bob, Levels: []domain.DifficultyLevel{domain.DifficultyLevelExpert}}))
	})

	t.Run("filters by skill, and by skill and concept on the same node", func(t *testing.T) {
		assert.ElementsMatch(t, []string{alpha.ID, beta.ID}, ids(domain.LearningPathFilter{SkillIDs: []string{fingerpicking.ID}}))
		assert.Equal(t, []string{alpha.ID}, ids(domain.LearningPathFilter{SkillIDs: []string{fingerpicking.ID}, ConceptIDs: []string{syncopation.ID}}))
	})

	t.Run("orders by title by default, or by most recent update", func(t *testing.T) {
		assert.Equal(t, []string{alpha.ID, beta.ID, legacy.ID}, ids(domain.LearningPathFilter{}))
		assert.Equal(t, []string{beta.ID, legacy.ID, alpha.ID}, ids(domain.LearningPathFilter{Sort: domain.LearningPathSortUpdated}))
	})
}
