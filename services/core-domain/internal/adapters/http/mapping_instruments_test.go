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

func TestInstrumentMapping(t *testing.T) {
	guitar, piano := uuid.New(), uuid.New()
	level := domain.DifficultyLevelBeginner
	course := domain.Course{ID: uuid.NewString(), CreatedBy: uuid.NewString(), Language: "en", Level: level, InstrumentIDs: []string{guitar.String()}}
	published := &domain.CourseVersion{CourseID: course.ID, VersionNumber: 1, LevelSnapshot: level, InstrumentIDsSnapshot: []string{piano.String()}, PublishedAt: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}

	t.Run("courses carry their instruments; learners see the published version's", func(t *testing.T) {
		assert.Equal(t, []uuid.UUID{guitar}, toCourse(course, published, userNames{}).InstrumentIds)
		assert.Equal(t, []uuid.UUID{guitar}, toCourseCatalogEntry(course, authoringListView, published, userNames{}).InstrumentIds)
		assert.Equal(t, []uuid.UUID{piano}, toCourseCatalogEntry(course, learnerCatalogView, published, userNames{}).InstrumentIds)
		assert.Equal(t, []uuid.UUID{piano}, toGeneratedCourseVersion(*published).InstrumentIdsSnapshot)
		assert.Equal(t, []uuid.UUID{piano}, toCourseDetail(course.ID, application.PublishedCourseView{InstrumentIDs: []string{piano.String()}}).InstrumentIds)
	})

	t.Run("an item for every instrument responds with an empty list, never null", func(t *testing.T) {
		every := course
		every.InstrumentIDs = nil
		path := domain.LearningPath{ID: uuid.NewString(), TeacherID: uuid.NewString()}
		node := domain.ContentNode{ID: uuid.NewString(), TeacherID: uuid.NewString(), ContentType: domain.ContentTypeArticle}

		for _, got := range [][]uuid.UUID{
			toCourse(every, nil, userNames{}).InstrumentIds,
			toLearningPath(path, userNames{}).InstrumentIds,
			toContentNode(node, userNames{}).InstrumentIds,
			toContentNodeVersion(domain.ContentNodeVersion{ContentNodeID: node.ID}).InstrumentIdsSnapshot,
		} {
			assert.NotNil(t, got)
			assert.Empty(t, got)
		}
	})

	t.Run("paths, content nodes and node versions carry their instruments", func(t *testing.T) {
		path := domain.LearningPath{ID: uuid.NewString(), TeacherID: uuid.NewString(), InstrumentIDs: []string{guitar.String(), piano.String()}}
		node := domain.ContentNode{ID: uuid.NewString(), TeacherID: uuid.NewString(), ContentType: domain.ContentTypeArticle, InstrumentIDs: []string{piano.String()}}

		assert.Equal(t, []uuid.UUID{guitar, piano}, toLearningPath(path, userNames{}).InstrumentIds)
		assert.Equal(t, []uuid.UUID{piano}, toContentNode(node, userNames{}).InstrumentIds)
		assert.Equal(t, []uuid.UUID{piano}, toContentNodeVersion(domain.ContentNodeVersion{ContentNodeID: node.ID, InstrumentIDsSnapshot: []string{piano.String()}}).InstrumentIdsSnapshot)
	})

	t.Run("every list's instrument parameter maps onto its filter", func(t *testing.T) {
		assert.Equal(t, guitar.String(), courseListFilter(generated.ListCoursesParams{InstrumentId: &guitar}).InstrumentID)
		assert.Equal(t, guitar.String(), catalogCourseListFilter(generated.ListCatalogCoursesParams{InstrumentId: &guitar}).InstrumentID)
		assert.Equal(t, guitar.String(), learningPathListFilter(generated.ListLearningPathsParams{InstrumentId: &guitar}).InstrumentID)
	})
}
