//go:build integration

package repo

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntLearningPathRepository_Publishing(t *testing.T) {
	f := newCourseListFixture(t)
	teacher := uuid.NewString()
	strOf := func(s string) *string { return &s }
	path := func(title string, summary, language *string, status domain.LearningPathStatus) domain.LearningPath {
		node := domain.ContentNode{
			ID: uuid.NewString(), TeacherID: teacher, Title: "Node " + uuid.NewString(), ContentType: domain.ContentTypeVideo,
			Classification: domain.Classification{DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
			CreatedAt:      fixedAt,
		}
		require.NoError(t, f.nodes.Create(f.ctx, node))
		created := domain.LearningPath{
			ID: uuid.NewString(), TeacherID: teacher, Title: title, Summary: summary, Language: language, Status: status,
			CreatedAt: fixedAt, UpdatedAt: fixedAt,
			Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: node.ID, Title: node.Title, ContentType: node.ContentType}},
		}
		require.NoError(t, f.paths.Create(f.ctx, created))
		return created
	}
	chords := path("Open Chords", strOf("Your first chords"), strOf("en"), domain.LearningPathStatusPublished)
	acordes := path("Acordes", strOf("Primeiros acordes e ritmos"), strOf("pt_BR"), domain.LearningPathStatusDraft)
	bare := path("Bare", nil, nil, "")
	ids := func(filter domain.LearningPathFilter) []string {
		page, err := f.paths.List(f.ctx, filter, domain.PageRequest{Limit: 50})
		require.NoError(t, err)
		out := make([]string, len(page.Items))
		for i, p := range page.Items {
			out[i] = p.ID
		}
		return out
	}

	t.Run("summary, language and status round-trip; a path saved without a status is a draft", func(t *testing.T) {
		got, err := f.paths.GetByID(f.ctx, chords.ID)
		require.NoError(t, err)
		assert.Equal(t, strOf("Your first chords"), got.Summary)
		assert.Equal(t, strOf("en"), got.Language)
		assert.Equal(t, domain.LearningPathStatusPublished, got.Status)

		plain, err := f.paths.GetByID(f.ctx, bare.ID)
		require.NoError(t, err)
		assert.Nil(t, plain.Summary)
		assert.Nil(t, plain.Language)
		assert.Equal(t, domain.LearningPathStatusDraft, plain.Status)
	})

	t.Run("filters by status and by language; a path with no language never matches", func(t *testing.T) {
		assert.Equal(t, []string{chords.ID}, ids(domain.LearningPathFilter{Status: domain.LearningPathStatusPublished}))
		assert.ElementsMatch(t, []string{acordes.ID, bare.ID}, ids(domain.LearningPathFilter{Status: domain.LearningPathStatusDraft}))
		assert.Equal(t, []string{acordes.ID}, ids(domain.LearningPathFilter{Language: "pt_BR"}))
	})

	t.Run("text search matches the summary as well as the title", func(t *testing.T) {
		assert.Equal(t, []string{acordes.ID}, ids(domain.LearningPathFilter{Query: "ritmos"}))
		assert.Equal(t, []string{chords.ID}, ids(domain.LearningPathFilter{Query: "open"}))
	})

	t.Run("creator ids are listed once per creator, for the matching paths only", func(t *testing.T) {
		otherTeacher := uuid.NewString()
		node := domain.ContentNode{
			ID: uuid.NewString(), TeacherID: otherTeacher, Title: "Node " + uuid.NewString(), ContentType: domain.ContentTypeVideo,
			Classification: domain.Classification{DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStatePending},
			CreatedAt:      fixedAt,
		}
		require.NoError(t, f.nodes.Create(f.ctx, node))
		require.NoError(t, f.paths.Create(f.ctx, domain.LearningPath{
			ID: uuid.NewString(), TeacherID: otherTeacher, Title: "Other", Status: domain.LearningPathStatusDraft, CreatedAt: fixedAt, UpdatedAt: fixedAt,
			Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: node.ID, Title: node.Title, ContentType: node.ContentType}},
		}))

		published, err := f.paths.ListCreatorIDs(f.ctx, domain.LearningPathFilter{Status: domain.LearningPathStatusPublished})
		require.NoError(t, err)
		assert.Equal(t, []string{teacher}, published)

		all, err := f.paths.ListCreatorIDs(f.ctx, domain.LearningPathFilter{})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{teacher, otherTeacher}, all)
	})

	t.Run("updating the status changes only the status", func(t *testing.T) {
		require.NoError(t, f.paths.UpdateStatus(f.ctx, acordes.ID, domain.LearningPathStatusPublished))

		got, err := f.paths.GetByID(f.ctx, acordes.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.LearningPathStatusPublished, got.Status)
		assert.Equal(t, strOf("Primeiros acordes e ritmos"), got.Summary)
	})

	t.Run("updating the status of a path that does not exist is not found", func(t *testing.T) {
		require.ErrorIs(t, f.paths.UpdateStatus(f.ctx, uuid.NewString(), domain.LearningPathStatusPublished), domain.ErrNotFound)
	})

	t.Run("a replace clears a left-out summary and language but keeps the status", func(t *testing.T) {
		replaced := chords
		replaced.Summary, replaced.Language, replaced.Status = nil, nil, domain.LearningPathStatusDraft
		require.NoError(t, f.paths.Replace(f.ctx, replaced))

		got, err := f.paths.GetByID(f.ctx, chords.ID)
		require.NoError(t, err)
		assert.Nil(t, got.Summary)
		assert.Nil(t, got.Language)
		assert.Equal(t, domain.LearningPathStatusPublished, got.Status, "only publish and unpublish change the status")
	})
}
