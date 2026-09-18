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

var fixedCreatedAt = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

func newContentService(nodes *fakeContentNodeRepository, expanded *fakeExpandedContentRepository) *application.ContentService {
	return application.NewContentService(nodes, expanded, idSequence(), func() time.Time { return fixedCreatedAt })
}

func teacherCaller() domain.User { return domain.User{ID: "teacher-1", Role: domain.RoleTeacher} }
func adminCaller() domain.User   { return domain.User{ID: "admin-1", Role: domain.RoleAdmin} }
func studentCaller() domain.User { return domain.User{ID: "student-1", Role: domain.RoleStudent} }

func TestContentService_CreateContentNode(t *testing.T) {
	t.Run("a teacher creates a video content node with classification", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		node, err := svc.CreateContentNode(context.Background(), teacherCaller(), "Introduction to Triad Shapes",
			domain.ContentTypeVideo, "triad-shapes", "chord-theory", domain.DifficultyLevelBeginner)

		require.NoError(t, err)
		assert.Equal(t, "teacher-1", node.TeacherID)
		assert.Equal(t, domain.ReviewStatePending, node.Classification.ReviewState)
	})

	t.Run("an admin creates a content node", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), adminCaller(), "Sweep Picking Fundamentals",
			domain.ContentTypeVideo, "sweep-picking", "technique", domain.DifficultyLevelAdvanced)

		require.NoError(t, err)
	})

	t.Run("a student cannot create a content node", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), studentCaller(), "Title",
			domain.ContentTypeVideo, "skill", "concept", domain.DifficultyLevelBeginner)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("creating a content node without a title is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), "",
			domain.ContentTypeVideo, "skill", "concept", domain.DifficultyLevelBeginner)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "title", valErr.Fields[0].Field)
	})

	t.Run("creating a content node without classification is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), "Title",
			domain.ContentTypeVideo, "", "", "")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "classification", valErr.Fields[0].Field)
	})

	t.Run("creating a content node with an unrecognised difficulty level is rejected", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateContentNode(context.Background(), teacherCaller(), "Title",
			domain.ContentTypeVideo, "skill", "concept", domain.DifficultyLevel("expert"))

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "difficulty_level", valErr.Fields[0].Field)
	})
}

func TestContentService_GetContentNode(t *testing.T) {
	t.Run("any authenticated user retrieves a content node by id", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		node := domain.ContentNode{ID: "node-1", Title: "Intro"}
		nodes.put(node)
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.GetContentNode(context.Background(), "node-1")

		require.NoError(t, err)
		assert.Equal(t, node, got)
	})

	t.Run("retrieving a content node that does not exist returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.GetContentNode(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestContentService_ListContentNodes(t *testing.T) {
	t.Run("a teacher lists all content nodes", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		nodes.put(articleNode("node-2"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), "", "", "")

		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("an admin lists all content nodes", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), adminCaller(), "", "", "")

		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("filters by content type", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		nodes.put(articleNode("node-2"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), domain.ContentTypeArticle, "", "")

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "node-2", got[0].ID)
	})

	t.Run("filters by skill", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-1", Classification: domain.Classification{Skill: "triad-shapes"}})
		nodes.put(domain.ContentNode{ID: "node-2", Classification: domain.Classification{Skill: "sweep-picking"}})
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), "", "sweep-picking", "")

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "node-2", got[0].ID)
	})

	t.Run("listing when none exist returns an empty list", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		got, err := svc.ListContentNodes(context.Background(), teacherCaller(), "", "", "")

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a student cannot list content nodes", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.ListContentNodes(context.Background(), studentCaller(), "", "", "")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestContentService_UpdateContentNode(t *testing.T) {
	t.Run("a teacher updates a content node's title and classification", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{
			ID: "node-1", ContentType: domain.ContentTypeVideo,
			Classification: domain.Classification{Skill: "triad-shapes", Concept: "chord-theory", DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStateConfirmed},
		})
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", "Revised title",
			"triad-shapes", "chord-theory", domain.DifficultyLevelIntermediate)

		require.NoError(t, err)
		assert.Equal(t, "Revised title", got.Title)
		assert.Equal(t, domain.DifficultyLevelIntermediate, got.Classification.DifficultyLevel)
	})

	t.Run("updating does not change content type", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", "Revised title",
			"skill", "concept", domain.DifficultyLevelBeginner)

		require.NoError(t, err)
		assert.Equal(t, domain.ContentTypeVideo, got.ContentType)
	})

	t.Run("updating does not reset an admin-confirmed review state", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{
			ID: "node-1", ContentType: domain.ContentTypeVideo,
			Classification: domain.Classification{Skill: "s", Concept: "c", DifficultyLevel: domain.DifficultyLevelBeginner, ReviewState: domain.ReviewStateConfirmed},
		})
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		got, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", "Revised title", "s", "c", domain.DifficultyLevelBeginner)

		require.NoError(t, err)
		assert.Equal(t, domain.ReviewStateConfirmed, got.Classification.ReviewState)
	})

	t.Run("updating without a title is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", "", "skill", "concept", domain.DifficultyLevelBeginner)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "title")
	})

	t.Run("updating without classification is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "node-1", "Title", "", "", "")

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "classification")
	})

	t.Run("a student cannot update a content node", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.UpdateContentNode(context.Background(), studentCaller(), "node-1", "Hijacked title", "skill", "concept", domain.DifficultyLevelBeginner)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("updating a content node that does not exist returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.UpdateContentNode(context.Background(), teacherCaller(), "missing", "Title", "skill", "concept", domain.DifficultyLevelBeginner)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func videoNode(id string) domain.ContentNode {
	return domain.ContentNode{ID: id, ContentType: domain.ContentTypeVideo}
}

func articleNode(id string) domain.ContentNode {
	return domain.ContentNode{ID: id, ContentType: domain.ContentTypeArticle}
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func TestContentService_CreateExpandedContent(t *testing.T) {
	t.Run("a teacher adds an image to a video lesson at a specific timestamp", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/img.png",
			intPtr(150), intPtr(165), nil, nil, nil)

		require.NoError(t, err)
		assert.Equal(t, "node-1", item.ContentNodeID)
	})

	t.Run("a teacher adds an image to an article at a specific paragraph", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		item, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/img.png",
			nil, nil, intPtr(3), intPtr(8000), nil)

		require.NoError(t, err)
		assert.Equal(t, "node-1", item.ContentNodeID)
	})

	t.Run("adding expanded content to a video node without trigger_at_seconds is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/img.png",
			nil, nil, intPtr(3), intPtr(5000), nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "trigger_at_seconds")
	})

	t.Run("hide_at_seconds not greater than trigger_at_seconds is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/img.png",
			intPtr(150), intPtr(150), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "hide_at_seconds")
	})

	t.Run("article node without trigger_at_paragraph is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/img.png",
			intPtr(90), intPtr(100), nil, nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "trigger_at_paragraph")
	})

	t.Run("article node with trigger_at_paragraph zero is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/img.png",
			nil, nil, intPtr(0), intPtr(5000), nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "trigger_at_paragraph")
	})

	t.Run("article node without duration_ms is rejected", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(articleNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/img.png",
			nil, nil, intPtr(3), nil, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assertHasField(t, valErr, "duration_ms")
	})

	t.Run("a student cannot add expanded content", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		svc := newContentService(nodes, newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), studentCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/img.png",
			intPtr(150), intPtr(165), nil, nil, nil)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("adding expanded content to a non-existent content node returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "missing",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/img.png",
			intPtr(150), intPtr(165), nil, nil, nil)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestContentService_ListExpandedContent(t *testing.T) {
	t.Run("listing expanded content for a video node returns items", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		expanded := newFakeExpandedContentRepository()
		svc := newContentService(nodes, expanded)

		_, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/a.png", intPtr(90), intPtr(100), nil, nil, nil)
		require.NoError(t, err)
		_, err = svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/b.png", intPtr(150), intPtr(160), nil, nil, nil)
		require.NoError(t, err)

		items, err := svc.ListExpandedContent(context.Background(), "node-1")

		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("listing for a non-existent content node returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.ListExpandedContent(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestContentService_GetExpandedContent(t *testing.T) {
	t.Run("any authenticated user retrieves a specific expanded content item", func(t *testing.T) {
		nodes := newFakeContentNodeRepository()
		nodes.put(videoNode("node-1"))
		expanded := newFakeExpandedContentRepository()
		svc := newContentService(nodes, expanded)

		created, err := svc.CreateExpandedContent(context.Background(), teacherCaller(), "node-1",
			domain.ExpandedContentTypeImage, "https://cdn.example.com/a.png", intPtr(90), intPtr(100), nil, nil, nil)
		require.NoError(t, err)

		got, err := svc.GetExpandedContent(context.Background(), created.ID)

		require.NoError(t, err)
		assert.Equal(t, created, got)
	})

	t.Run("retrieving an expanded content item that does not exist returns not found", func(t *testing.T) {
		svc := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository())

		_, err := svc.GetExpandedContent(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func assertHasField(t *testing.T, valErr *domain.ValidationError, field string) {
	t.Helper()
	for _, f := range valErr.Fields {
		if f.Field == field {
			return
		}
	}
	t.Fatalf("expected field %q among validation errors, got %+v", field, valErr.Fields)
}
