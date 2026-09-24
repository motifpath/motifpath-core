package http

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestToCourseCatalogEntry(t *testing.T) {
	publishedAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	creator := "0b6b8b8e-6f5b-4f7a-9c53-2a5f7a3a1111"
	live := domain.Course{
		ID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f2222", CreatedBy: creator,
		Title: "Edited Title", Summary: "Edited summary.", Level: domain.DifficultyLevelExpert,
		Status: domain.CourseStatusPublished,
	}
	version := domain.CourseVersion{
		CourseID: live.ID, VersionNumber: 1, PublishedAt: publishedAt,
		TitleSnapshot: "Published Title", SummarySnapshot: "Published summary.", LevelSnapshot: domain.DifficultyLevelBeginner,
	}
	student := domain.User{ID: "student-1", Role: domain.RoleStudent}
	teacher := domain.User{ID: "teacher-1", Role: domain.RoleTeacher}

	t.Run("a student sees the latest published version's title, summary and level", func(t *testing.T) {
		entry := toCourseCatalogEntry(live, student, &version)

		assert.Equal(t, "Published Title", entry.Title)
		assert.Equal(t, "Published summary.", entry.Summary)
		assert.EqualValues(t, domain.DifficultyLevelBeginner, entry.Level)
		assert.Nil(t, entry.HasUnpublishedChanges)
	})

	t.Run("a teacher sees the live draft and whether it has unpublished changes", func(t *testing.T) {
		entry := toCourseCatalogEntry(live, teacher, &version)

		assert.Equal(t, "Edited Title", entry.Title)
		assert.EqualValues(t, domain.DifficultyLevelExpert, entry.Level)
		require.NotNil(t, entry.HasUnpublishedChanges)
		assert.True(t, *entry.HasUnpublishedChanges)
	})

	t.Run("every entry reports the course's creator", func(t *testing.T) {
		assert.Equal(t, creator, toCourseCatalogEntry(live, student, &version).CreatedBy.String())
		assert.Equal(t, creator, toCourseCatalogEntry(live, teacher, &version).CreatedBy.String())
	})

	t.Run("an unpublished course has no published_at", func(t *testing.T) {
		entry := toCourseCatalogEntry(live, teacher, nil)

		assert.Nil(t, entry.PublishedAt)
	})
}
