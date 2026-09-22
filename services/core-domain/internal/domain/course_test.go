package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func openChordsPath() domain.LearningPath {
	return domain.LearningPath{ID: "path-01", TeacherID: "teacher-1", Title: "Open Chords"}
}

func strummingPath() domain.LearningPath {
	return domain.LearningPath{ID: "path-02", TeacherID: "teacher-1", Title: "Strumming Patterns"}
}

func TestNewCourse(t *testing.T) {
	createdAt := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	t.Run("a course is created as a draft with 1-based checkpoint positions", func(t *testing.T) {
		course, err := domain.NewCourse("course-1", "teacher-1", "Fingerstyle Journey", "From first chords to a repertoire.",
			domain.DifficultyLevelBeginner,
			[]domain.NewCourseCheckpoint{
				{Path: openChordsPath()},
				{Path: strummingPath()},
			}, createdAt)

		require.NoError(t, err)
		assert.Equal(t, "course-1", course.ID)
		assert.Equal(t, "teacher-1", course.CreatedBy)
		assert.Equal(t, domain.CourseStatusDraft, course.Status)
		assert.Equal(t, createdAt, course.CreatedAt)
		require.Len(t, course.Checkpoints, 2)
		assert.Equal(t, 1, course.Checkpoints[0].Position)
		assert.Equal(t, "path-01", course.Checkpoints[0].LearningPathID)
		assert.Equal(t, 2, course.Checkpoints[1].Position)
		assert.Equal(t, "path-02", course.Checkpoints[1].LearningPathID)
	})

	t.Run("a checkpoint without a title override shows the learning path's own title as effective_title", func(t *testing.T) {
		course, err := domain.NewCourse("course-1", "teacher-1", "Title", "Summary", domain.DifficultyLevelBeginner,
			[]domain.NewCourseCheckpoint{{Path: openChordsPath()}}, createdAt)

		require.NoError(t, err)
		require.Len(t, course.Checkpoints, 1)
		assert.Nil(t, course.Checkpoints[0].Title)
		assert.Equal(t, "Open Chords", course.Checkpoints[0].EffectiveTitle)
	})

	t.Run("a checkpoint with a title override shows the override as effective_title", func(t *testing.T) {
		override := "Stage 1: Open chords"
		course, err := domain.NewCourse("course-1", "teacher-1", "Title", "Summary", domain.DifficultyLevelBeginner,
			[]domain.NewCourseCheckpoint{{Path: openChordsPath(), Title: &override}}, createdAt)

		require.NoError(t, err)
		require.Len(t, course.Checkpoints, 1)
		require.NotNil(t, course.Checkpoints[0].Title)
		assert.Equal(t, "Stage 1: Open chords", *course.Checkpoints[0].Title)
		assert.Equal(t, "Stage 1: Open chords", course.Checkpoints[0].EffectiveTitle)
	})

	t.Run("creating a course without a title is rejected", func(t *testing.T) {
		_, err := domain.NewCourse("course-1", "teacher-1", "", "Summary", domain.DifficultyLevelBeginner,
			[]domain.NewCourseCheckpoint{{Path: openChordsPath()}}, createdAt)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "title", valErr.Fields[0].Field)
	})

	t.Run("creating a course without a summary is rejected", func(t *testing.T) {
		_, err := domain.NewCourse("course-1", "teacher-1", "Title", "", domain.DifficultyLevelBeginner,
			[]domain.NewCourseCheckpoint{{Path: openChordsPath()}}, createdAt)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "summary", valErr.Fields[0].Field)
	})

	t.Run("creating a course with an invalid level is rejected", func(t *testing.T) {
		_, err := domain.NewCourse("course-1", "teacher-1", "Title", "Summary", domain.DifficultyLevel("not-a-level"),
			[]domain.NewCourseCheckpoint{{Path: openChordsPath()}}, createdAt)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "level", valErr.Fields[0].Field)
	})

	t.Run("creating a course with no checkpoints is rejected", func(t *testing.T) {
		_, err := domain.NewCourse("course-1", "teacher-1", "Title", "Summary", domain.DifficultyLevelBeginner, nil, createdAt)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "checkpoints", valErr.Fields[0].Field)
	})
}
