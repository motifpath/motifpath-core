package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

func newCourseService(paths *fakeLearningPathRepository, courses *fakeCourseRepository) *application.CourseService {
	return application.NewCourseService(paths, courses, idSequence(), func() time.Time { return fixedCreatedAt })
}

// checkpointInputs builds an unlabelled CheckpointInput slice from learning
// path ids, for the cases that don't exercise title overrides.
func checkpointInputs(ids ...string) []application.CheckpointInput {
	items := make([]application.CheckpointInput, len(ids))
	for i, id := range ids {
		items[i] = application.CheckpointInput{LearningPathID: id}
	}
	return items
}

func TestCourseService_CreateCourse(t *testing.T) {
	t.Run("a teacher creates a course with multiple checkpoints", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01", Title: "Open Chords"})
		paths.put(domain.LearningPath{ID: "path-02", Title: "Strumming Patterns"})
		svc := newCourseService(paths, newFakeCourseRepository())

		course, err := svc.CreateCourse(context.Background(), teacherCaller(), "Fingerstyle Journey",
			"From first chords to a repertoire.", domain.DifficultyLevelBeginner, checkpointInputs("path-01", "path-02"))

		require.NoError(t, err)
		assert.Equal(t, "teacher-1", course.CreatedBy)
		assert.Equal(t, domain.CourseStatusDraft, course.Status)
		require.Len(t, course.Checkpoints, 2)
		assert.Equal(t, 1, course.Checkpoints[0].Position)
		assert.Equal(t, "Open Chords", course.Checkpoints[0].EffectiveTitle)
		assert.Equal(t, 2, course.Checkpoints[1].Position)
		assert.Equal(t, "Strumming Patterns", course.Checkpoints[1].EffectiveTitle)
	})

	t.Run("a checkpoint title override is stored and returned as effective_title", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01", Title: "Open Chords"})
		svc := newCourseService(paths, newFakeCourseRepository())

		course, err := svc.CreateCourse(context.Background(), teacherCaller(), "Title", "Summary",
			domain.DifficultyLevelBeginner,
			[]application.CheckpointInput{{LearningPathID: "path-01", Title: strPtr("Stage 1: Open chords")}})

		require.NoError(t, err)
		require.Len(t, course.Checkpoints, 1)
		require.NotNil(t, course.Checkpoints[0].Title)
		assert.Equal(t, "Stage 1: Open chords", *course.Checkpoints[0].Title)
		assert.Equal(t, "Stage 1: Open chords", course.Checkpoints[0].EffectiveTitle)
	})

	t.Run("an admin creates a course", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01", Title: "Open Chords"})
		svc := newCourseService(paths, newFakeCourseRepository())

		_, err := svc.CreateCourse(context.Background(), adminCaller(), "Title", "Summary",
			domain.DifficultyLevelBeginner, checkpointInputs("path-01"))

		require.NoError(t, err)
	})

	t.Run("creating a course without a title is rejected", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01"})
		svc := newCourseService(paths, newFakeCourseRepository())

		_, err := svc.CreateCourse(context.Background(), teacherCaller(), "", "Summary",
			domain.DifficultyLevelBeginner, checkpointInputs("path-01"))

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "title", valErr.Fields[0].Field)
	})

	t.Run("creating a course with no checkpoints is rejected", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		_, err := svc.CreateCourse(context.Background(), teacherCaller(), "Title", "Summary",
			domain.DifficultyLevelBeginner, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		require.Len(t, valErr.Fields, 1)
		assert.Equal(t, "checkpoints", valErr.Fields[0].Field)
	})

	t.Run("creating a course that references a non-existent learning path is rejected", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		_, err := svc.CreateCourse(context.Background(), teacherCaller(), "Title", "Summary",
			domain.DifficultyLevelBeginner, checkpointInputs("missing"))

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "learning_path_id", valErr.Fields[0].Field)
	})

	t.Run("a student cannot create a course", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		_, err := svc.CreateCourse(context.Background(), studentCaller(), "Title", "Summary",
			domain.DifficultyLevelBeginner, checkpointInputs("path-01"))

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestCourseService_GetCourse(t *testing.T) {
	t.Run("a teacher retrieves a course by id", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", Title: "Fingerstyle Journey"})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		got, err := svc.GetCourse(context.Background(), teacherCaller(), "course-1")

		require.NoError(t, err)
		assert.Equal(t, "course-1", got.ID)
	})

	t.Run("a student cannot view a course's live draft", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1"})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.GetCourse(context.Background(), studentCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("retrieving a course that does not exist returns not found", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		_, err := svc.GetCourse(context.Background(), teacherCaller(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestCourseService_ListCourses(t *testing.T) {
	t.Run("a teacher lists every course regardless of status", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", Status: domain.CourseStatusDraft})
		courses.put(domain.Course{ID: "course-2", Status: domain.CourseStatusRetired})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		got, err := svc.ListCourses(context.Background(), teacherCaller(), nil)

		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("a teacher narrows the list with a status filter", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", Status: domain.CourseStatusDraft})
		courses.put(domain.Course{ID: "course-2", Status: domain.CourseStatusRetired})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		draft := domain.CourseStatusDraft
		got, err := svc.ListCourses(context.Background(), teacherCaller(), &draft)

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "course-1", got[0].ID)
	})

	t.Run("a student's results are always published regardless of the status filter given", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", Status: domain.CourseStatusDraft})
		courses.put(domain.Course{ID: "course-2", Status: domain.CourseStatusPublished})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		retired := domain.CourseStatusRetired
		got, err := svc.ListCourses(context.Background(), studentCaller(), &retired)

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "course-2", got[0].ID)
	})

	t.Run("listing when none exist returns an empty list", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		got, err := svc.ListCourses(context.Background(), teacherCaller(), nil)

		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestCourseService_ReplaceCourse(t *testing.T) {
	t.Run("a teacher reorders a course's checkpoints", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01", Title: "One"})
		paths.put(domain.LearningPath{ID: "path-02", Title: "Two"})
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1", Title: "Old", Summary: "Old summary",
			Level: domain.DifficultyLevelBeginner, Status: domain.CourseStatusDraft, CreatedAt: fixedCreatedAt,
			Checkpoints: []domain.CourseCheckpoint{
				{Position: 1, LearningPathID: "path-01", EffectiveTitle: "One"},
				{Position: 2, LearningPathID: "path-02", EffectiveTitle: "Two"},
			}})
		svc := newCourseService(paths, courses)

		got, err := svc.ReplaceCourse(context.Background(), teacherCaller(), "course-1", "New", "New summary",
			domain.DifficultyLevelIntermediate, checkpointInputs("path-02", "path-01"))

		require.NoError(t, err)
		assert.Equal(t, "New", got.Title)
		assert.Equal(t, "New summary", got.Summary)
		assert.Equal(t, domain.DifficultyLevelIntermediate, got.Level)
		require.Len(t, got.Checkpoints, 2)
		assert.Equal(t, "path-02", got.Checkpoints[0].LearningPathID)
		assert.Equal(t, 1, got.Checkpoints[0].Position)
	})

	t.Run("replacing preserves the course's id, owner, creation time, and status", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01", Title: "One"})
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1", Title: "Old", Summary: "Old summary",
			Level: domain.DifficultyLevelBeginner, Status: domain.CourseStatusPublished, CreatedAt: fixedCreatedAt,
			Checkpoints: []domain.CourseCheckpoint{{Position: 1, LearningPathID: "path-01", EffectiveTitle: "One"}}})
		svc := newCourseService(paths, courses)

		got, err := svc.ReplaceCourse(context.Background(), teacherCaller(), "course-1", "New", "New summary",
			domain.DifficultyLevelBeginner, checkpointInputs("path-01"))

		require.NoError(t, err)
		assert.Equal(t, "course-1", got.ID)
		assert.Equal(t, "teacher-1", got.CreatedBy)
		assert.Equal(t, fixedCreatedAt, got.CreatedAt)
		assert.Equal(t, domain.CourseStatusPublished, got.Status)
	})

	t.Run("a student cannot replace a course", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1"})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.ReplaceCourse(context.Background(), studentCaller(), "course-1", "Title", "Summary",
			domain.DifficultyLevelBeginner, checkpointInputs("path-01"))

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a teacher cannot replace another teacher's course", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01", Title: "One"})
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1",
			Checkpoints: []domain.CourseCheckpoint{{Position: 1, LearningPathID: "path-01", EffectiveTitle: "One"}}})
		svc := newCourseService(paths, courses)

		_, err := svc.ReplaceCourse(context.Background(), otherTeacherCaller(), "course-1", "Hijacked", "Summary",
			domain.DifficultyLevelBeginner, checkpointInputs("path-01"))

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an admin can replace any teacher's course", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01", Title: "One"})
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1",
			Checkpoints: []domain.CourseCheckpoint{{Position: 1, LearningPathID: "path-01", EffectiveTitle: "One"}}})
		svc := newCourseService(paths, courses)

		got, err := svc.ReplaceCourse(context.Background(), adminCaller(), "course-1", "Revised by admin", "Summary",
			domain.DifficultyLevelBeginner, checkpointInputs("path-01"))

		require.NoError(t, err)
		assert.Equal(t, "Revised by admin", got.Title)
	})

	t.Run("replacing with an empty checkpoints array is rejected", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1"})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.ReplaceCourse(context.Background(), teacherCaller(), "course-1", "Title", "Summary",
			domain.DifficultyLevelBeginner, nil)

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "checkpoints", valErr.Fields[0].Field)
	})

	t.Run("replacing with a checkpoint referencing a non-existent learning path is rejected", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1"})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.ReplaceCourse(context.Background(), teacherCaller(), "course-1", "Title", "Summary",
			domain.DifficultyLevelBeginner, checkpointInputs("missing"))

		var valErr *domain.ValidationError
		require.True(t, errors.As(err, &valErr))
		assert.Equal(t, "learning_path_id", valErr.Fields[0].Field)
	})

	t.Run("replacing a course that does not exist returns not found", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01"})
		svc := newCourseService(paths, newFakeCourseRepository())

		_, err := svc.ReplaceCourse(context.Background(), teacherCaller(), "missing", "Title", "Summary",
			domain.DifficultyLevelBeginner, checkpointInputs("path-01"))

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
