package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestThumbnailMapping(t *testing.T) {
	draft, published := strPtr("https://cdn.motifpath.io/t/draft.png"), strPtr("https://cdn.motifpath.io/t/v1.png")
	level := domain.DifficultyLevelBeginner
	course := domain.Course{ID: uuid.NewString(), CreatedBy: uuid.NewString(), Language: "en", Level: level, ThumbnailURL: draft}
	version := &domain.CourseVersion{CourseID: course.ID, VersionNumber: 1, LevelSnapshot: level, ThumbnailURLSnapshot: published, PublishedAt: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}

	t.Run("course pages get the draft's thumbnail for authors and the published one for learners", func(t *testing.T) {
		assert.Equal(t, draft, toCourse(course, version, userNames{}).ThumbnailUrl)
		assert.Equal(t, draft, toCourseCatalogEntry(course, authoringListView, version, userNames{}).ThumbnailUrl)
		assert.Equal(t, published, toCourseCatalogEntry(course, learnerCatalogView, version, userNames{}).ThumbnailUrl)
		assert.Equal(t, published, toGeneratedCourseVersion(*version).ThumbnailUrlSnapshot)
		assert.Equal(t, published, toCourseDetail(course.ID, application.PublishedCourseView{ThumbnailURL: published}).ThumbnailUrl)
	})

	t.Run("an enrollment shows the thumbnail of the version it is pinned to", func(t *testing.T) {
		enrollment := domain.CourseEnrollment{ID: uuid.NewString(), StudentID: uuid.NewString(), CourseID: course.ID, CourseThumbnailURL: published}

		assert.Equal(t, published, toCourseEnrollment(enrollment, userNames{}).CourseThumbnailUrl)
	})

	t.Run("learning paths, content nodes and node versions carry their thumbnail", func(t *testing.T) {
		path := domain.LearningPath{ID: uuid.NewString(), TeacherID: uuid.NewString(), ThumbnailURL: draft}
		node := domain.ContentNode{ID: uuid.NewString(), TeacherID: uuid.NewString(), ContentType: domain.ContentTypeArticle, ThumbnailURL: draft}

		assert.Equal(t, draft, toLearningPath(path, userNames{}).ThumbnailUrl)
		assert.Equal(t, draft, toContentNode(node, userNames{}).ThumbnailUrl)
		assert.Equal(t, published, toContentNodeVersion(domain.ContentNodeVersion{ContentNodeID: node.ID, ThumbnailURLSnapshot: published}).ThumbnailUrlSnapshot)
	})
}
