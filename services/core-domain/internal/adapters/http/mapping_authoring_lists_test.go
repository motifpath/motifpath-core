package http

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestAuthoringListFieldsMapping(t *testing.T) {
	created := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	changed := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	teacher := "0b6b8b8e-6f5b-4f7a-9c53-2a5f7a3a1111"

	t.Run("a content node carries its last change and publishing state", func(t *testing.T) {
		two := 2
		node := domain.ContentNode{ID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f2222", TeacherID: teacher, ContentType: domain.ContentTypeVideo,
			CreatedAt: created, UpdatedAt: changed, LatestPublishedVersion: &two, HasUnpublishedChanges: true}

		got := toContentNode(node, userNames{})

		assert.Equal(t, changed, got.UpdatedAt)
		assert.True(t, got.HasUnpublishedChanges)
		require.NotNil(t, got.LatestPublishedVersion)
		assert.Equal(t, 2, *got.LatestPublishedVersion)
	})

	t.Run("a never-published content node has no latest version", func(t *testing.T) {
		node := domain.ContentNode{ID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f2222", TeacherID: teacher, ContentType: domain.ContentTypeVideo,
			CreatedAt: created, UpdatedAt: created, HasUnpublishedChanges: true}

		got := toContentNode(node, userNames{})

		assert.Nil(t, got.LatestPublishedVersion)
	})

	t.Run("an exercise carries its last change", func(t *testing.T) {
		ex := domain.Exercise{ID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f3333", ExerciseType: domain.ExerciseTypeTextResponse,
			Prompt: domain.NewPlainTextPrompt("p"), CreatedAt: created, UpdatedAt: changed}

		assert.Equal(t, changed, toExercise(ex, userNames{}).UpdatedAt)
	})

	t.Run("a diagram carries its last change", func(t *testing.T) {
		d := domain.Diagram{ID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f4444", InstrumentID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f5555",
			CreatedBy: teacher, Kind: domain.DiagramKindCustom, Purpose: domain.DiagramPurposeGeneral, CreatedAt: created, UpdatedAt: changed}

		assert.Equal(t, changed, toGeneratedDiagram(d, userNames{}).UpdatedAt)
	})

	t.Run("the authoring course list carries the last change; the catalog does not", func(t *testing.T) {
		course := domain.Course{ID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f6666", CreatedBy: teacher, Title: "T", Summary: "S",
			Level: domain.DifficultyLevelBeginner, Status: domain.CourseStatusPublished, CreatedAt: created, UpdatedAt: changed,
			Checkpoints: []domain.CourseCheckpoint{{Position: 1, LearningPathID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f7777", EffectiveTitle: "P"}}}
		version := domain.CourseVersion{CourseID: course.ID, VersionNumber: 1, TitleSnapshot: "T", SummarySnapshot: "S",
			LevelSnapshot: domain.DifficultyLevelBeginner, PublishedAt: created,
			Checkpoints: []domain.CourseVersionCheckpoint{{Position: 1, LearningPathID: "6c0e2d6a-1f3b-4c58-8f3e-3b9e5a0f7777", EffectiveTitle: "P"}}}

		authoring := toCourseCatalogEntry(course, authoringListView, &version, userNames{})
		catalog := toCourseCatalogEntry(course, learnerCatalogView, &version, userNames{})

		require.NotNil(t, authoring.UpdatedAt)
		assert.Equal(t, changed, *authoring.UpdatedAt)
		assert.Nil(t, catalog.UpdatedAt)
	})
}
