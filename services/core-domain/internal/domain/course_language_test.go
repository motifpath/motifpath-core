package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestNewCourse_Language(t *testing.T) {
	createdAt := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	build := func(language string) (domain.Course, error) {
		return domain.NewCourse("course-1", "teacher-1", domain.CourseFields{
			Title: "Violão Fingerstyle", Summary: "Do básico ao repertório.", Level: domain.DifficultyLevelBeginner,
			Language: language, Checkpoints: []domain.NewCourseCheckpoint{{Path: openChordsPath()}},
		}, offered, createdAt)
	}

	t.Run("a course is written in one offered language", func(t *testing.T) {
		course, err := build("pt_BR")

		require.NoError(t, err)
		assert.Equal(t, "pt_BR", course.Language)
	})

	for name, language := range map[string]string{
		"no language":                         "",
		`the language-agnostic marker "any"`:  domain.LanguageCodeAny,
		"a language MotifPath does not offer": "xx",
	} {
		t.Run("rejected: "+name, func(t *testing.T) {
			_, err := build(language)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, "language", valErr.Fields[0].Field)
		})
	}
}

func TestNewCourseVersionSnapshot_Language(t *testing.T) {
	course := fingerstyleCourse()
	course.Language = "pt_BR"

	version := domain.NewCourseVersionSnapshot("version-1", course, 1, time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC))

	assert.Equal(t, "pt_BR", version.LanguageSnapshot)
}
