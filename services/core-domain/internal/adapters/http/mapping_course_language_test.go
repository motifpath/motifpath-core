package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func TestCourseLanguageMapping(t *testing.T) {
	course := domain.Course{ID: uuid.NewString(), CreatedBy: uuid.NewString(), Title: "Draft", Language: "pt_BR", Level: domain.DifficultyLevelBeginner, Status: domain.CourseStatusPublished}
	published := &domain.CourseVersion{CourseID: course.ID, VersionNumber: 1, TitleSnapshot: "Published", LanguageSnapshot: "en", LevelSnapshot: domain.DifficultyLevelBeginner, PublishedAt: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}

	t.Run("the live draft's language reaches authors, the published one learners", func(t *testing.T) {
		assert.Equal(t, "pt_BR", toCourse(course, published, userNames{}).Language)
		assert.Equal(t, "pt_BR", toCourseCatalogEntry(course, authoringListView, published, userNames{}).Language)
		assert.Equal(t, "en", toCourseCatalogEntry(course, learnerCatalogView, published, userNames{}).Language)
	})

	t.Run("versions and the published outline carry the language they were published in", func(t *testing.T) {
		assert.Equal(t, "en", toGeneratedCourseVersion(*published).LanguageSnapshot)
		assert.Equal(t, "en", toCourseDetail(course.ID, application.PublishedCourseView{Language: "en"}).Language)
	})

	t.Run("both course lists map their language parameter onto the filter", func(t *testing.T) {
		language := "pt_BR"

		assert.Equal(t, "pt_BR", courseListFilter(generated.ListCoursesParams{Language: &language}).Language)
		assert.Equal(t, "pt_BR", catalogCourseListFilter(generated.ListCatalogCoursesParams{Language: &language}).Language)
	})
}
