package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestThumbnails(t *testing.T) {
	at := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	course := func(thumbnail *string) (domain.Course, error) {
		return domain.NewCourse("course-1", "teacher-1", domain.CourseFields{
			Title: "Fingerstyle", Summary: "S", Level: domain.DifficultyLevelBeginner, Language: "en",
			ThumbnailURL: thumbnail, Checkpoints: []domain.NewCourseCheckpoint{{Path: openChordsPath()}},
		}, offered, at)
	}
	path := func(thumbnail *string) (domain.LearningPath, error) {
		return domain.NewLearningPath("path-1", "teacher-1", domain.LearningPathFields{
			Title: "Open Chords", Level: domain.DifficultyLevelBeginner, ThumbnailURL: thumbnail,
			Items: []domain.NewLearningPathItem{{Node: domain.ContentNode{ID: "node-1", Title: "One", ContentType: domain.ContentTypeVideo}}},
		}, at)
	}
	media := "https://cdn.motifpath.io/v.mp4"
	nodeFields := func(thumbnail *string) domain.ContentNodeFields {
		return domain.ContentNodeFields{
			Title: "Barre Chords", SkillIDs: []string{"s"}, ConceptIDs: []string{"c"}, Difficulty: domain.DifficultyLevelBeginner,
			LanguageCodes: []string{"en"}, MediaURL: &media, ThumbnailURL: thumbnail,
		}
	}
	node := func(thumbnail *string) (domain.ContentNode, error) {
		return domain.NewContentNode("node-1", "teacher-1", domain.ContentTypeVideo, nodeFields(thumbnail), at)
	}
	thumbnail := strPtr("https://cdn.motifpath.io/thumbnails/fingerstyle.png")

	t.Run("courses, paths and content nodes keep their thumbnail, and versions snapshot it", func(t *testing.T) {
		c, err := course(thumbnail)
		require.NoError(t, err)
		p, err := path(thumbnail)
		require.NoError(t, err)
		n, err := node(thumbnail)
		require.NoError(t, err)

		assert.Equal(t, thumbnail, c.ThumbnailURL)
		assert.Equal(t, thumbnail, p.ThumbnailURL)
		assert.Equal(t, thumbnail, n.ThumbnailURL)
		assert.Equal(t, thumbnail, domain.NewCourseVersionSnapshot("v-1", c, 1, at).ThumbnailURLSnapshot)
		assert.Equal(t, thumbnail, domain.NewContentNodeVersionSnapshot("v-2", n, 1, "teacher-1", at).ThumbnailURLSnapshot)
	})

	t.Run("updating a content node without a thumbnail removes it", func(t *testing.T) {
		n, err := node(thumbnail)
		require.NoError(t, err)

		updated, err := n.Update(nodeFields(nil))

		require.NoError(t, err)
		assert.Nil(t, updated.ThumbnailURL)
	})

	t.Run("a thumbnail that is not an http or https URL is rejected everywhere", func(t *testing.T) {
		bad := strPtr("ftp://files.example/fingerstyle.png")
		_, courseErr := course(bad)
		_, pathErr := path(bad)
		_, nodeErr := node(bad)

		for _, err := range []error{courseErr, pathErr, nodeErr} {
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "thumbnail_url", valErr.Fields[0].Field)
		}
	})
}

func TestNewMediaUploadRequest_Thumbnail(t *testing.T) {
	exercise := "exercise-1"

	t.Run("a thumbnail is an image with no exercise", func(t *testing.T) {
		req, err := domain.NewMediaUploadRequest(domain.MediaUploadPurposeThumbnail, nil, domain.MediaContentTypeImage, "cover.png")

		require.NoError(t, err)
		assert.Equal(t, domain.MediaUploadPurposeThumbnail, req.Purpose)
	})

	for name, tc := range map[string]struct {
		exerciseID  *string
		contentType domain.MediaContentType
		field       string
	}{
		"audio":            {contentType: domain.MediaContentTypeAudio, field: "content_type"},
		"with an exercise": {exerciseID: &exercise, contentType: domain.MediaContentTypeImage, field: "exercise_id"},
	} {
		t.Run("rejected: a thumbnail "+name, func(t *testing.T) {
			_, err := domain.NewMediaUploadRequest(domain.MediaUploadPurposeThumbnail, tc.exerciseID, tc.contentType, "cover.png")

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tc.field, valErr.Fields[0].Field)
		})
	}
}

func TestNewCourseEnrollment_Thumbnail(t *testing.T) {
	version := domain.CourseVersion{
		CourseID: "course-1", VersionNumber: 1, TitleSnapshot: "Fingerstyle",
		ThumbnailURLSnapshot: strPtr("https://cdn.motifpath.io/thumbnails/v1.png"),
		Checkpoints:          []domain.CourseVersionCheckpoint{{Position: 1, LearningPathID: "path-01"}},
	}

	enrollment, err := domain.NewCourseEnrollment("enrollment-1", "alice", version, "student-path-1", time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	assert.Equal(t, version.ThumbnailURLSnapshot, enrollment.CourseThumbnailURL)
}
