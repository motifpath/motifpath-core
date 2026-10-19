package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func twoCheckpointVersion() domain.CourseVersion {
	return domain.CourseVersion{
		ID:              "version-1",
		CourseID:        "course-1",
		VersionNumber:   1,
		TitleSnapshot:   "Fingerstyle Journey",
		SummarySnapshot: "From first chords to a repertoire.",
		LevelSnapshot:   domain.DifficultyLevelBeginner,
		Checkpoints: []domain.CourseVersionCheckpoint{
			{Position: 1, LearningPathID: "path-01", EffectiveTitle: "Open Chords"},
			{Position: 2, LearningPathID: "path-02", EffectiveTitle: "Strumming Patterns"},
		},
		PublishedAt:                time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
		AvailableForNewEnrollments: true,
	}
}

func TestNewCourseEnrollment(t *testing.T) {
	enrolledAt := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	t.Run("starts active at checkpoint 1, pinned to the given version", func(t *testing.T) {
		enrollment, err := domain.NewCourseEnrollment("enrollment-1", "alice", twoCheckpointVersion(), "student-path-1", enrolledAt)

		require.NoError(t, err)
		assert.Equal(t, "enrollment-1", enrollment.ID)
		assert.Equal(t, "alice", enrollment.StudentID)
		assert.Equal(t, "course-1", enrollment.CourseID)
		assert.Equal(t, "Fingerstyle Journey", enrollment.CourseTitle)
		assert.Equal(t, 1, enrollment.CourseVersionNumber)
		assert.Equal(t, domain.CourseEnrollmentStatusActive, enrollment.Status)
		require.NotNil(t, enrollment.ActiveCheckpointStudentPathID)
		assert.Equal(t, "student-path-1", *enrollment.ActiveCheckpointStudentPathID)
		require.NotNil(t, enrollment.ActiveCheckpointPosition)
		assert.Equal(t, 1, *enrollment.ActiveCheckpointPosition)
		assert.Equal(t, enrolledAt, enrollment.EnrolledAt)
	})

	t.Run("fails when the pinned version has no checkpoints", func(t *testing.T) {
		version := twoCheckpointVersion()
		version.Checkpoints = nil

		_, err := domain.NewCourseEnrollment("enrollment-1", "alice", version, "student-path-1", enrolledAt)

		var valErr *domain.ValidationError
		require.ErrorAs(t, err, &valErr)
	})
}

func TestCourseEnrollment_IsActive(t *testing.T) {
	enrollment := domain.CourseEnrollment{Status: domain.CourseEnrollmentStatusActive}
	assert.True(t, enrollment.IsActive())

	enrollment.Status = domain.CourseEnrollmentStatusAbandoned
	assert.False(t, enrollment.IsActive())
}
