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
	return newLearningPathServiceWithVersions(nodes, paths, newFakeCourseVersionRepository())
}

func newLearningPathServiceWithVersions(nodes *fakeContentNodeRepository, paths *fakeLearningPathRepository, courseVersions *fakeCourseVersionRepository) *application.LearningPathService {
	return application.NewLearningPathService(nodes, paths, courseVersions, idSequence(), func() time.Time { return fixedCreatedAt })
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

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Beginner Guitar — Week 1", Items: pathItems("node-01", "node-02", "node-03")})

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

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Rhythm Foundations", Items: []application.PathItemInput{
			{ContentNodeID: "node-01", SectionLabel: strPtr("Open chords")},
			{ContentNodeID: "node-02", SectionLabel: strPtr("Open chords")},
			{ContentNodeID: "node-03", SectionLabel: strPtr("Strumming patterns")},
		}})

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

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Unlabelled", Items: []application.PathItemInput{{ContentNodeID: "node-01"}, {ContentNodeID: "node-02"}}})

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

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Rhythm Foundations", Items: []application.PathItemInput{
			{ContentNodeID: "node-01", SectionLabel: strPtr("Open chords ")},
			{ContentNodeID: "node-02", SectionLabel: strPtr(" Open chords")},
		}})

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

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Rhythm Foundations", Items: []application.PathItemInput{
			{ContentNodeID: "node-01", SectionLabel: strPtr("")},
			{ContentNodeID: "node-02", SectionLabel: strPtr("   ")},
		}})

		require.NoError(t, err)
		require.Len(t, path.Items, 2)
		assert.Nil(t, path.Items[0].SectionLabel)
		assert.Nil(t, path.Items[1].SectionLabel)
	})

	t.Run("an admin creates a learning path", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), adminCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Advanced Techniques", Items: pathItems("node-01")})

		require.NoError(t, err)
	})

	t.Run("creating a learning path without a title is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01"})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), teacherCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "", Items: pathItems("node-01")})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "title")
	})

	t.Run("creating a learning path with no items is rejected", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), teacherCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Title", Items: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "items")
	})

	t.Run("creating a learning path that references a non-existent content node is rejected", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), teacherCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Title", Items: pathItems("missing")})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "content_node_id")
	})

	t.Run("a student cannot create a learning path", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		_, err := svc.CreateLearningPath(context.Background(), studentCaller(), application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Title", Items: pathItems("node-01")})

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
	firstPage := domain.PageRequest{Limit: 20, Offset: 0}

	t.Run("a teacher lists all learning paths", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", Title: "Week 1"})
		paths.put(domain.LearningPath{ID: "path-2", Title: "Week 2"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		got, err := svc.ListLearningPaths(context.Background(), teacherCaller(), domain.LearningPathFilter{}, firstPage)

		require.NoError(t, err)
		assert.Len(t, got.Items, 2)
		assert.Equal(t, 2, got.Total)
	})

	t.Run("an admin lists all learning paths", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		got, err := svc.ListLearningPaths(context.Background(), adminCaller(), domain.LearningPathFilter{}, firstPage)

		require.NoError(t, err)
		assert.Len(t, got.Items, 1)
	})

	t.Run("searches by title text", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", Title: "Open Chords Path"})
		paths.put(domain.LearningPath{ID: "path-2", Title: "Strumming Path"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		got, err := svc.ListLearningPaths(context.Background(), teacherCaller(), domain.LearningPathFilter{Query: "chords"}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "path-1", got.Items[0].ID)
		assert.Equal(t, 1, got.Total)
	})

	t.Run("returns the requested page and the filtered total", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		for _, id := range []string{"path-1", "path-2", "path-3"} {
			paths.put(domain.LearningPath{ID: id, Title: id})
		}
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		got, err := svc.ListLearningPaths(context.Background(), teacherCaller(), domain.LearningPathFilter{}, domain.PageRequest{Limit: 2, Offset: 2})

		require.NoError(t, err)
		assert.Equal(t, 3, got.Total)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "path-3", got.Items[0].ID)
	})

	t.Run("listing when none exist returns an empty page", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		got, err := svc.ListLearningPaths(context.Background(), teacherCaller(), domain.LearningPathFilter{}, firstPage)

		require.NoError(t, err)
		assert.Empty(t, got.Items)
		assert.Zero(t, got.Total)
	})

	t.Run("a student cannot list learning paths", func(t *testing.T) {
		svc := newLearningPathService(newFakeContentNodeRepository(), newFakeLearningPathRepository())

		_, err := svc.ListLearningPaths(context.Background(), studentCaller(), domain.LearningPathFilter{}, firstPage)

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

		got, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "path-1", application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Beginner Guitar", Items: pathItems("node-02", "node-01", "node-03")})

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

		got, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "path-1", application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "New title", Items: pathItems("node-01")})

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

		_, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "path-1", application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Title", Items: nil})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "items")
	})

	t.Run("replacing with an item referencing a non-existent content node is rejected", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		_, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "path-1", application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Title", Items: pathItems("missing")})

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "content_node_id")
	})

	t.Run("a student cannot replace a learning path", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		svc := newLearningPathService(newFakeContentNodeRepository(), paths)

		_, err := svc.ReplaceLearningPath(context.Background(), studentCaller(), "path-1", application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Title", Items: pathItems("node-01")})

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher cannot replace another teacher's learning path", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title",
			Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: "node-01"}}, CreatedAt: fixedCreatedAt})
		svc := newLearningPathService(nodes, paths)

		_, err := svc.ReplaceLearningPath(context.Background(), otherTeacherCaller(), "path-1", application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Hijacked title", Items: pathItems("node-01")})

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an admin can replace any teacher's learning path", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title",
			Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: "node-01"}}, CreatedAt: fixedCreatedAt})
		svc := newLearningPathService(nodes, paths)

		got, err := svc.ReplaceLearningPath(context.Background(), adminCaller(), "path-1", application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Revised by admin", Items: pathItems("node-01")})

		require.NoError(t, err)
		assert.Equal(t, "Revised by admin", got.Title)
	})

	t.Run("replacing a learning path that does not exist returns not found", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01"})
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		_, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), "missing", application.LearningPathInput{Level: domain.DifficultyLevelBeginner, Title: "Title", Items: pathItems("node-01")})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestLearningPathService_DeleteLearningPath(t *testing.T) {
	t.Run("a teacher deletes a learning path they own", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		svc := newLearningPathServiceWithVersions(newFakeContentNodeRepository(), paths, newFakeCourseVersionRepository())

		err := svc.DeleteLearningPath(context.Background(), teacherCaller(), "path-1")

		require.NoError(t, err)
		_, err = paths.GetByID(context.Background(), "path-1")
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("an admin deletes a learning path created by a teacher", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		svc := newLearningPathServiceWithVersions(newFakeContentNodeRepository(), paths, newFakeCourseVersionRepository())

		err := svc.DeleteLearningPath(context.Background(), adminCaller(), "path-1")

		require.NoError(t, err)
	})

	t.Run("a teacher cannot delete another teacher's learning path", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		svc := newLearningPathServiceWithVersions(newFakeContentNodeRepository(), paths, newFakeCourseVersionRepository())

		err := svc.DeleteLearningPath(context.Background(), otherTeacherCaller(), "path-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
		_, getErr := paths.GetByID(context.Background(), "path-1")
		assert.NoError(t, getErr, "the path must still exist after a rejected delete")
	})

	t.Run("a student cannot delete a learning path", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		svc := newLearningPathServiceWithVersions(newFakeContentNodeRepository(), paths, newFakeCourseVersionRepository())

		err := svc.DeleteLearningPath(context.Background(), studentCaller(), "path-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("deleting a learning path that does not exist returns not found", func(t *testing.T) {
		svc := newLearningPathServiceWithVersions(newFakeContentNodeRepository(), newFakeLearningPathRepository(), newFakeCourseVersionRepository())

		err := svc.DeleteLearningPath(context.Background(), teacherCaller(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("deleting a learning path referenced by a published course's checkpoint is refused", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		versions := newFakeCourseVersionRepository()
		twoCheckpointCourseVersion(versions, "course-1", "path-1", "path-2")
		svc := newLearningPathServiceWithVersions(newFakeContentNodeRepository(), paths, versions)

		err := svc.DeleteLearningPath(context.Background(), teacherCaller(), "path-1")

		assert.ErrorIs(t, err, domain.ErrConflict)
		_, getErr := paths.GetByID(context.Background(), "path-1")
		assert.NoError(t, getErr, "the path must still exist after a rejected delete")
	})

	t.Run("deleting a learning path referenced only by an unpublished course draft is allowed", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1", TeacherID: "teacher-1", Title: "Title"})
		// A draft course's checkpoints live in CourseCheckpointRepository, not
		// CourseVersionRepository — no CourseVersion has ever been published
		// for it, so IsLearningPathReferenced finds nothing.
		svc := newLearningPathServiceWithVersions(newFakeContentNodeRepository(), paths, newFakeCourseVersionRepository())

		err := svc.DeleteLearningPath(context.Background(), teacherCaller(), "path-1")

		require.NoError(t, err)
	})
}

func TestLearningPathService_LevelAndLastUpdate(t *testing.T) {
	nodes := newFakeContentNodeRepository()
	nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
	input := func(level domain.DifficultyLevel) application.LearningPathInput {
		return application.LearningPathInput{Title: "Open Chords", Level: level, Items: pathItems("node-01")}
	}

	t.Run("a path is created at a level, last updated when it was created", func(t *testing.T) {
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		path, err := svc.CreateLearningPath(context.Background(), teacherCaller(), input(domain.DifficultyLevelBeginner))

		require.NoError(t, err)
		require.NotNil(t, path.Level)
		assert.Equal(t, domain.DifficultyLevelBeginner, *path.Level)
		assert.Equal(t, fixedCreatedAt, path.UpdatedAt)
	})

	for name, level := range map[string]domain.DifficultyLevel{"no level": "", "an unknown level": "virtuoso"} {
		t.Run("rejected: "+name, func(t *testing.T) {
			svc := newLearningPathService(nodes, newFakeLearningPathRepository())

			_, err := svc.CreateLearningPath(context.Background(), teacherCaller(), input(level))

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "level", valErr.Fields[0].Field)
		})
	}

	t.Run("replacing a path records the replace time and keeps the creation time", func(t *testing.T) {
		later := fixedCreatedAt.Add(48 * time.Hour)
		clock := []time.Time{fixedCreatedAt, later}
		svc := application.NewLearningPathService(nodes, newFakeLearningPathRepository(), newFakeCourseVersionRepository(), idSequence(), func() time.Time {
			now := clock[0]
			clock = clock[1:]
			return now
		})
		created, err := svc.CreateLearningPath(context.Background(), teacherCaller(), input(domain.DifficultyLevelBeginner))
		require.NoError(t, err)

		replaced, err := svc.ReplaceLearningPath(context.Background(), teacherCaller(), created.ID, input(domain.DifficultyLevelIntermediate))

		require.NoError(t, err)
		assert.Equal(t, fixedCreatedAt, replaced.CreatedAt)
		assert.Equal(t, later, replaced.UpdatedAt)
		assert.Equal(t, domain.DifficultyLevelIntermediate, *replaced.Level)
	})

	t.Run("an unknown sort order is rejected", func(t *testing.T) {
		svc := newLearningPathService(nodes, newFakeLearningPathRepository())

		_, err := svc.ListLearningPaths(context.Background(), teacherCaller(), domain.LearningPathFilter{Sort: "popularity"}, domain.PageRequest{Limit: 20})

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "sort", valErr.Fields[0].Field)
	})
}
