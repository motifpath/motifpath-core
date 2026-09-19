package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func newLearningPathService(nodes *fakeContentNodeRepository, paths *fakeLearningPathRepository) *application.LearningPathService {
	return application.NewLearningPathService(nodes, paths, idSequence(), func() time.Time { return fixedCreatedAt })
}

// pathItems builds an unlabelled PathItemInput slice from content node ids,
// for the cases that don't exercise section labels.
func pathItems(ids ...string) []application.PathItemInput {
	items := make([]application.PathItemInput, len(ids))
	for i, id := range ids {
		items[i] = application.PathItemInput{ContentNodeID: id}
	}
	return items
}

func TestLearningPathService_CreateLearningPath(t *testing.T) {
	t.Run("a teacher creates a learning path with multiple content nodes", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		nodes.put(domain.ContentNode{ID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo})
		nodes.put(domain.ContentNode{ID: "node-03", Title: "Three", ContentType: domain.ContentTypeArticle})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), "Beginner Guitar — Week 1",
			pathItems("node-01", "node-02", "node-03"))

		require.NoError(t, err)
		assert.Equal(t, "teacher-1", path.TeacherID)
		require.Len(t, path.Items, 3)
		assert.Equal(t, 1, path.Items[0].Position)
		assert.Equal(t, 2, path.Items[1].Position)
		assert.Equal(t, 3, path.Items[2].Position)
	})

	t.Run("a teacher creates a learning path with items grouped into sections", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		nodes.put(domain.ContentNode{ID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo})
		nodes.put(domain.ContentNode{ID: "node-03", Title: "Three", ContentType: domain.ContentTypeArticle})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), "Rhythm Foundations",
			[]application.PathItemInput{
				{ContentNodeID: "node-01", SectionLabel: strPtr("Open chords")},
				{ContentNodeID: "node-02", SectionLabel: strPtr("Open chords")},
				{ContentNodeID: "node-03", SectionLabel: strPtr("Strumming patterns")},
			})

		require.NoError(t, err)
		require.Len(t, path.Items, 3)
		require.NotNil(t, path.Items[0].SectionLabel)
		assert.Equal(t, "Open chords", *path.Items[0].SectionLabel)
		require.NotNil(t, path.Items[1].SectionLabel)
		assert.Equal(t, "Open chords", *path.Items[1].SectionLabel)
		require.NotNil(t, path.Items[2].SectionLabel)
		assert.Equal(t, "Strumming patterns", *path.Items[2].SectionLabel)
	})

	t.Run("a learning path created without section labels has no label on any item", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		nodes.put(domain.ContentNode{ID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), "Unlabelled",
			[]application.PathItemInput{{ContentNodeID: "node-01"}, {ContentNodeID: "node-02"}})

		require.NoError(t, err)
		require.Len(t, path.Items, 2)
		assert.Nil(t, path.Items[0].SectionLabel)
		assert.Nil(t, path.Items[1].SectionLabel)
	})

	t.Run("section labels are stored trimmed", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		nodes.put(domain.ContentNode{ID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), "Rhythm Foundations",
			[]application.PathItemInput{
				{ContentNodeID: "node-01", SectionLabel: strPtr("Open chords ")},
				{ContentNodeID: "node-02", SectionLabel: strPtr(" Open chords")},
			})

		require.NoError(t, err)
		require.Len(t, path.Items, 2)
		require.NotNil(t, path.Items[0].SectionLabel)
		require.NotNil(t, path.Items[1].SectionLabel)
		// Both items name the same section — the API contract must not make
		// them differ on whitespace alone.
		assert.Equal(t, "Open chords", *path.Items[0].SectionLabel)
		assert.Equal(t, "Open chords", *path.Items[1].SectionLabel)
	})

	t.Run("a blank section label is stored as no label", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		nodes.put(domain.ContentNode{ID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), "Rhythm Foundations",
			[]application.PathItemInput{
				{ContentNodeID: "node-01", SectionLabel: strPtr("")},
				{ContentNodeID: "node-02", SectionLabel: strPtr("   ")},
			})

		require.NoError(t, err)
		require.Len(t, path.Items, 2)
		assert.Nil(t, path.Items[0].SectionLabel)
		assert.Nil(t, path.Items[1].SectionLabel)
	})

	t.Run("an admin creates a learning path", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), adminCaller(), "Advanced Techniques", pathItems("node-01"))

		require.NoError(t, err)
	})

	t.Run("creating a learning path without a title is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01"})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), teacherCaller(), "", pathItems("node-01"))

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "title")
	})

	t.Run("creating a learning path with no items is rejected", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), teacherCaller(), "Title", nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "items")
	})

	t.Run("creating a learning path that references a non-existent content node is rejected", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), teacherCaller(), "Title", pathItems("missing"))

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "content_node_id")
	})

	t.Run("a student cannot create a learning path", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), studentCaller(), "Title", pathItems("node-01"))

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestLearningPathService_GetLearningPath(t *testing.T) {
	t.Run("a teacher retrieves a learning path by id", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		path := domain.LearningPath{ID: "path-1", Title: "Week 1"}
		paths.put(path)
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		got, err := svc.GetLearningPath(context.Background(), teacherCaller(), "path-1")

		require.NoError(t, err)
		assert.Equal(t, path, got)
	})

	t.Run("a student cannot retrieve a learning path directly", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		_, err := svc.GetLearningPath(context.Background(), studentCaller(), "path-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("retrieving a learning path that does not exist returns not found", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		_, err := svc.GetLearningPath(context.Background(), teacherCaller(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestLearningPathService_ListLearningPaths(t *testing.T) {
	t.Run("a teacher lists all learning paths", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", Title: "Week 1"})
		paths.put(domain.LearningPath{ID: "path-2", Title: "Week 2"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		got, err := svc.ListLearningPaths(context.Background(), teacherCaller())

		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("an admin lists all learning paths", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		got, err := svc.ListLearningPaths(context.Background(), adminCaller())

		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("listing when none exist returns an empty list", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		got, err := svc.ListLearningPaths(context.Background(), teacherCaller())

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a student cannot list learning paths", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		_, err := svc.ListLearningPaths(context.Background(), studentCaller())

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestLearningPathService_ReplaceLearningPath(t *testing.T) {
	t.Run("a teacher reorders a learning path's items", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		nodes.put(domain.ContentNode{ID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo})
		nodes.put(domain.ContentNode{ID: "node-03", Title: "Three", ContentType: domain.ContentTypeArticle})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Beginner Guitar",
			Items: []domain.LearningPathItem{
				{Position: 1, ContentNodeID: "node-01"},
				{Position: 2, ContentNodeID: "node-02"},
				{Position: 3, ContentNodeID: "node-03"},
			}, CreatedAt: fixedCreatedAt})
		svc := newLearningPathService(nodes, paths)

		got, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "path-1", "Beginner Guitar",
			pathItems("node-02", "node-01", "node-03"))

		require.NoError(t, err)
		require.Len(t, got.Items, 3)
		assert.Equal(t, "node-02", got.Items[0].ContentNodeID)
		assert.Equal(t, 1, got.Items[0].Position)
		assert.Equal(t, "node-01", got.Items[1].ContentNodeID)
		assert.Equal(t, 2, got.Items[1].Position)
	})

	t.Run("replacing preserves the path's id, owner, and creation time", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Old title",
			Items:     []domain.LearningPathItem{{Position: 1, ContentNodeID: "node-01"}},
			CreatedAt: fixedCreatedAt})
		svc := newLearningPathService(nodes, paths)

		got, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "path-1", "New title", pathItems("node-01"))

		require.NoError(t, err)
		assert.Equal(t, "path-1", got.ID)
		assert.Equal(t, "teacher-1", got.TeacherID)
		assert.Equal(t, "New title", got.Title)
		assert.Equal(t, fixedCreatedAt, got.CreatedAt)
	})

	t.Run("replacing with an empty items array is rejected", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		_, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "path-1", "Title", nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "items")
	})

	t.Run("replacing with an item referencing a non-existent content node is rejected", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		_, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "path-1", "Title", pathItems("missing"))

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "content_node_id")
	})

	t.Run("a student cannot replace a learning path", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		_, err := svc.ReplaceLearningPath(context.Background(), studentCaller(), "path-1", "Title", pathItems("node-01"))

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher cannot replace another teacher's learning path", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title",
			Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: "node-01"}}, CreatedAt: fixedCreatedAt})
		svc := newLearningPathService(nodes, paths)

		_, err := svc.ReplaceLearningPath(context.Background(), otherTeacherCaller(), "path-1", "Hijacked title", pathItems("node-01"))

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an admin can replace any teacher's learning path", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title",
			Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: "node-01"}}, CreatedAt: fixedCreatedAt})
		svc := newLearningPathService(nodes, paths)

		got, err := svc.ReplaceLearningPath(context.Background(), adminCaller(), "path-1", "Revised by admin", pathItems("node-01"))

		require.NoError(t, err)
		assert.Equal(t, "Revised by admin", got.Title)
	})

	t.Run("replacing a learning path that does not exist returns not found", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01"})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		_, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "missing", "Title", pathItems("node-01"))

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
