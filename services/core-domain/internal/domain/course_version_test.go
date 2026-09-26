package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func fingerstyleCourse() domain.Course {
	return domain.Course{
		ID:      "course-1",
		Title:   "Fingerstyle Journey",
		Summary: "From first chords to a repertoire.",
		Level:   domain.DifficultyLevelBeginner,
		Status:  domain.CourseStatusDraft,
		Checkpoints: []domain.CourseCheckpoint{
			{Position: 1, LearningPathID: "path-01", EffectiveTitle: "Open Chords"},
			{Position: 2, LearningPathID: "path-02", EffectiveTitle: "Strumming Patterns"},
		},
	}
}

func TestNewCourseVersionSnapshot(t *testing.T) {
	publishedAt := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	t.Run("snapshots title, summary, level, and checkpoint identity", func(t *testing.T) {
		version := domain.NewCourseVersionSnapshot("version-1", fingerstyleCourse(), 1, publishedAt)

		assert.Equal(t, "version-1", version.ID)
		assert.Equal(t, "course-1", version.CourseID)
		assert.Equal(t, 1, version.VersionNumber)
		assert.Equal(t, "Fingerstyle Journey", version.TitleSnapshot)
		assert.Equal(t, "From first chords to a repertoire.", version.SummarySnapshot)
		assert.Equal(t, domain.DifficultyLevelBeginner, version.LevelSnapshot)
		assert.Equal(t, publishedAt, version.PublishedAt)
		require.Len(t, version.Checkpoints, 2)
		assert.Equal(t, domain.CourseVersionCheckpoint{Position: 1, LearningPathID: "path-01", EffectiveTitle: "Open Chords"}, version.Checkpoints[0])
		assert.Equal(t, domain.CourseVersionCheckpoint{Position: 2, LearningPathID: "path-02", EffectiveTitle: "Strumming Patterns"}, version.Checkpoints[1])
	})

	t.Run("a freshly published version is available for new enrollments by default", func(t *testing.T) {
		version := domain.NewCourseVersionSnapshot("version-1", fingerstyleCourse(), 1, publishedAt)

		assert.True(t, version.AvailableForNewEnrollments)
	})

	t.Run("a later draft edit does not alter the already-built snapshot", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)

		course.Title = "Renamed after publish"
		course.Checkpoints[0].EffectiveTitle = "Renamed checkpoint"

		assert.Equal(t, "Fingerstyle Journey", version.TitleSnapshot)
		assert.Equal(t, "Open Chords", version.Checkpoints[0].EffectiveTitle)
	})
}

func TestHasUnpublishedChanges(t *testing.T) {
	publishedAt := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	t.Run("true when the course has never been published", func(t *testing.T) {
		assert.True(t, domain.HasUnpublishedChanges(fingerstyleCourse(), nil))
	})

	t.Run("false when the live draft exactly matches the latest published version", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)

		assert.False(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when the title changed since the latest publish", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.Title = "New Title"

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when the summary changed since the latest publish", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.Summary = "New summary"

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when the level changed since the latest publish", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.Level = domain.DifficultyLevelAdvanced

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when a checkpoint was added", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.Checkpoints = append(course.Checkpoints, domain.CourseCheckpoint{Position: 3, LearningPathID: "path-03", EffectiveTitle: "Barre Chords"})

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when a checkpoint was removed", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.Checkpoints = course.Checkpoints[:1]

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when checkpoints were reordered", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.Checkpoints[0], course.Checkpoints[1] = course.Checkpoints[1], course.Checkpoints[0]
		course.Checkpoints[0].Position, course.Checkpoints[1].Position = 1, 2

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when a checkpoint's effective_title changed (e.g. a title override was added)", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.Checkpoints[0].EffectiveTitle = "Overridden title"

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when the language changed since the latest publish", func(t *testing.T) {
		course := fingerstyleCourse()
		course.Language = "en"
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.Language = "pt_BR"

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when the instruments changed since the latest publish", func(t *testing.T) {
		course := fingerstyleCourse()
		course.InstrumentIDs = []string{"guitar"}
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.InstrumentIDs = []string{"guitar", "bass"}

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("false when the same instruments come back in another order", func(t *testing.T) {
		course := fingerstyleCourse()
		course.InstrumentIDs = []string{"guitar", "bass"}
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.InstrumentIDs = []string{"bass", "guitar"}

		assert.False(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("false for every instrument, whether recorded as no list or an empty one", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		version.InstrumentIDsSnapshot = nil
		course.InstrumentIDs = []string{}

		assert.False(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when a thumbnail was added, changed or removed since the latest publish", func(t *testing.T) {
		first, second := "https://cdn.test/a.png", "https://cdn.test/b.png"
		for _, change := range []struct{ before, after *string }{{nil, &first}, {&first, &second}, {&first, nil}} {
			course := fingerstyleCourse()
			course.ThumbnailURL = change.before
			version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
			course.ThumbnailURL = change.after

			assert.True(t, domain.HasUnpublishedChanges(course, &version))
		}
	})

	t.Run("false when the thumbnail is the same url", func(t *testing.T) {
		published, current := "https://cdn.test/a.png", "https://cdn.test/a.png"
		course := fingerstyleCourse()
		course.ThumbnailURL = &published
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.ThumbnailURL = &current

		assert.False(t, domain.HasUnpublishedChanges(course, &version))
	})

	t.Run("true when a checkpoint's learning_path_id changed", func(t *testing.T) {
		course := fingerstyleCourse()
		version := domain.NewCourseVersionSnapshot("version-1", course, 1, publishedAt)
		course.Checkpoints[0].LearningPathID = "path-99"

		assert.True(t, domain.HasUnpublishedChanges(course, &version))
	})
}
