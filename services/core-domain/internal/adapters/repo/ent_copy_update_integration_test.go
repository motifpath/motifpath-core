//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func TestEntStudentPathRepository_ReplaceContent(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntStudentPathRepository(client)
	creator := uuid.NewString()
	nodeA, nodeB, nodeC := uuid.NewString(), uuid.NewString(), uuid.NewString()
	versionA, versionB, versionC := uuid.NewString(), uuid.NewString(), uuid.NewString()
	label := "Open chords"
	enrollmentID := uuid.NewString()
	position := 2

	original := domain.StudentPath{
		ID: uuid.NewString(), StudentID: uuid.NewString(), SourceTemplateID: uuid.NewString(),
		Title: "Open chords", AssignedBy: creator, AssignedAt: fixedAt, CreatedBySnapshot: &creator,
		SourceCourseEnrollmentID: &enrollmentID, CourseCheckpointPosition: &position,
		Items: []domain.StudentPathItemRecord{
			{Position: 1, ContentNodeID: nodeA, ContentNodeVersionID: versionA, SectionLabel: &label},
			{Position: 2, ContentNodeID: nodeB, ContentNodeVersionID: versionB},
		},
	}
	require.NoError(t, repo.Create(ctx, original))

	summary, thumbnail, level := "Your first chords", "https://cdn.motifpath.io/thumbnails/new.png", domain.DifficultyLevelBeginner
	newPosition := 3
	updated := original
	updated.Title = "Open chords, revised"
	updated.SummarySnapshot, updated.LevelSnapshot, updated.ThumbnailURLSnapshot = &summary, &level, &thumbnail
	updated.CourseCheckpointPosition = &newPosition
	updated.Items = []domain.StudentPathItemRecord{
		{Position: 1, ContentNodeID: nodeB, ContentNodeVersionID: versionB},
		{Position: 2, ContentNodeID: nodeC, ContentNodeVersionID: versionC, SectionLabel: &label},
	}

	require.NoError(t, repo.ReplaceContent(ctx, updated))

	got, err := repo.GetByID(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, updated, got)

	t.Run("an unknown copy is not found", func(t *testing.T) {
		missing := updated
		missing.ID = uuid.NewString()

		assert.ErrorIs(t, repo.ReplaceContent(ctx, missing), domain.ErrNotFound)
	})
}

func TestEntCourseEnrollmentRepository_Repin(t *testing.T) {
	client := setupPostgres(t)
	ctx := context.Background()
	repo := NewEntCourseEnrollmentRepository(client)
	checkpointPathID := uuid.NewString()
	checkpointPosition := 1
	oldThumbnail := "https://cdn.motifpath.io/thumbnails/old.png"

	enrollment := domain.CourseEnrollment{
		ID: uuid.NewString(), StudentID: uuid.NewString(), CourseID: uuid.NewString(), CourseTitle: "Fingerstyle Journey",
		CourseThumbnailURL: &oldThumbnail, CourseVersionNumber: 1, Status: domain.CourseEnrollmentStatusActive,
		ActiveCheckpointStudentPathID: &checkpointPathID, ActiveCheckpointPosition: &checkpointPosition,
		EnrolledAt: fixedAt,
	}
	require.NoError(t, repo.Create(ctx, enrollment))

	t.Run("takes the new version's number, title and thumbnail", func(t *testing.T) {
		newThumbnail := "https://cdn.motifpath.io/thumbnails/new.png"

		require.NoError(t, repo.Repin(ctx, enrollment.ID, 3, "Fingerstyle Journey, revised", &newThumbnail))

		got, err := repo.GetByID(ctx, enrollment.ID)
		require.NoError(t, err)
		assert.Equal(t, 3, got.CourseVersionNumber)
		assert.Equal(t, "Fingerstyle Journey, revised", got.CourseTitle)
		assert.Equal(t, &newThumbnail, got.CourseThumbnailURL)
		assert.Equal(t, enrollment.ActiveCheckpointStudentPathID, got.ActiveCheckpointStudentPathID)
		assert.Equal(t, domain.CourseEnrollmentStatusActive, got.Status)
	})

	t.Run("a version without a thumbnail clears it", func(t *testing.T) {
		require.NoError(t, repo.Repin(ctx, enrollment.ID, 4, "Fingerstyle Journey", nil))

		got, err := repo.GetByID(ctx, enrollment.ID)
		require.NoError(t, err)
		assert.Nil(t, got.CourseThumbnailURL)
	})

	t.Run("an unknown enrollment is not found", func(t *testing.T) {
		assert.ErrorIs(t, repo.Repin(ctx, uuid.NewString(), 2, "Any", nil), domain.ErrNotFound)
	})
}
