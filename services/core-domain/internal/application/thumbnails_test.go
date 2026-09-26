package application_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestThumbnailUploads(t *testing.T) {
	storage := newFakeMediaStorage()
	svc := newMediaService(newFakeExerciseRepository(), storage)

	_, err := svc.CreateUploadURL(context.Background(), teacherCaller(), domain.MediaUploadPurposeThumbnail, nil, domain.MediaContentTypeImage, "cover.png")

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(storage.lastKey, "thumbnails/"), "thumbnails are stored apart, got %q", storage.lastKey)
	assert.True(t, strings.HasSuffix(storage.lastKey, ".png"))
}

func TestThumbnailsThroughServices(t *testing.T) {
	ctx := context.Background()
	thumbnail := "https://cdn.motifpath.io/thumbnails/cover.png"
	nodes := newFakeContentNodeRepository()
	nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo})
	paths := newFakeLearningPathRepository()
	paths.put(domain.LearningPath{ID: "path-01", Title: "Open Chords"})
	media := "https://cdn.motifpath.io/v.mp4"

	course, err := newCourseService(paths, newFakeCourseRepository()).CreateCourse(ctx, teacherCaller(), application.CourseInput{
		Title: "Fingerstyle", Summary: "S", Level: domain.DifficultyLevelBeginner, Language: "en",
		ThumbnailURL: &thumbnail, Checkpoints: checkpointInputs("path-01"),
	})
	require.NoError(t, err)
	path, err := newLearningPathService(nodes, newFakeLearningPathRepository()).CreateLearningPath(ctx, teacherCaller(), application.LearningPathInput{
		Title: "Open Chords", Level: domain.DifficultyLevelBeginner, ThumbnailURL: &thumbnail, Items: pathItems("node-01"),
	})
	require.NoError(t, err)
	node, err := newContentService(newFakeContentNodeRepository(), newFakeExpandedContentRepository()).CreateContentNode(ctx, teacherCaller(), application.ContentNodeInput{
		Title: "Barre Chords", ContentType: domain.ContentTypeVideo, SkillIDs: []string{"skill-1"}, ConceptIDs: []string{"concept-1"},
		Difficulty: domain.DifficultyLevelBeginner, Languages: []string{"en"}, MediaURL: &media, ThumbnailURL: &thumbnail,
	})
	require.NoError(t, err)

	for _, got := range []*string{course.ThumbnailURL, path.ThumbnailURL, node.ThumbnailURL} {
		require.NotNil(t, got)
		assert.Equal(t, thumbnail, *got)
	}
}
