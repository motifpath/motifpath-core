//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntStudentPathRepository_StandaloneCopies(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	nodeRepo := NewEntContentNodeRepository(client)
	pathRepo := NewEntLearningPathRepository(client)
	versionRepo := NewEntContentNodeVersionRepository(client)
	repo := NewEntStudentPathRepository(client)

	node := seedContentNode(t, ctx, nodeRepo)
	template := seedLearningPath(t, ctx, pathRepo, node)
	creator := uuid.NewString()
	version := domain.ContentNodeVersion{
		ID: uuid.NewString(), ContentNodeID: node.ID, VersionNumber: 1,
		Title: node.Title, ContentType: node.ContentType, PublishedBy: creator, PublishedAt: fixedAt,
	}
	require.NoError(t, versionRepo.Create(ctx, version))
	summary, thumbnail, level := "Your first chords", "https://cdn.motifpath.io/thumbnails/open-chords.png", domain.DifficultyLevelBeginner

	copyFor := func(studentID string, enrollmentID *string) domain.StudentPath {
		sp := domain.StudentPath{
			ID: uuid.NewString(), StudentID: studentID, SourceTemplateID: template.ID,
			Title: template.Title, AssignedBy: studentID, AssignedAt: fixedAt,
			SummarySnapshot: &summary, LevelSnapshot: &level, ThumbnailURLSnapshot: &thumbnail, CreatedBySnapshot: &creator,
			Items: []domain.StudentPathItemRecord{{Position: 1, ContentNodeID: node.ID, ContentNodeVersionID: version.ID}},
		}
		if enrollmentID != nil {
			position := 1
			sp.SourceCourseEnrollmentID, sp.CourseCheckpointPosition = enrollmentID, &position
		}
		return sp
	}

	t.Run("a copy's presentation snapshots round-trip", func(t *testing.T) {
		sp := copyFor(uuid.NewString(), nil)
		require.NoError(t, repo.Create(ctx, sp))

		got, err := repo.GetByID(ctx, sp.ID)

		require.NoError(t, err)
		assert.Equal(t, &summary, got.SummarySnapshot)
		require.NotNil(t, got.LevelSnapshot)
		assert.Equal(t, level, *got.LevelSnapshot)
		assert.Equal(t, &thumbnail, got.ThumbnailURLSnapshot)
		assert.Equal(t, &creator, got.CreatedBySnapshot)
	})

	t.Run("a second active standalone copy of the same template is refused", func(t *testing.T) {
		student := uuid.NewString()
		first := copyFor(student, nil)
		require.NoError(t, repo.Create(ctx, first))

		err := repo.Create(ctx, copyFor(student, nil))

		require.ErrorIs(t, err, domain.ErrAlreadyExists)
		found, err := repo.FindActiveStandalone(ctx, student, template.ID)
		require.NoError(t, err)
		assert.Equal(t, first.ID, found.ID)
	})

	t.Run("an archived copy or a course checkpoint copy doesn't block a new standalone copy", func(t *testing.T) {
		student := uuid.NewString()
		archived := copyFor(student, nil)
		require.NoError(t, repo.Create(ctx, archived))
		require.NoError(t, repo.Archive(ctx, archived.ID, fixedAt.Add(time.Hour)))
		enrollmentID := uuid.NewString()
		require.NoError(t, repo.Create(ctx, copyFor(student, &enrollmentID)))

		_, err := repo.FindActiveStandalone(ctx, student, template.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)

		require.NoError(t, repo.Create(ctx, copyFor(student, nil)))
	})
}
