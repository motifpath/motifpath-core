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
