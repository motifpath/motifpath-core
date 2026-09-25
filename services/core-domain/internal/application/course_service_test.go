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

func newCourseService(paths *fakeLearningPathRepository, courses *fakeCourseRepository, versions ...*fakeCourseVersionRepository) *application.CourseService {
	v := newFakeCourseVersionRepository()
	if len(versions) > 0 {
		v = versions[0]
	}
	return application.NewCourseService(paths, courses, v, idSequence(), func() time.Time { return fixedCreatedAt })
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

func TestCourseService_ListCourseCreatorIDs(t *testing.T) {
	published, draft, retired := domain.CourseStatusPublished, domain.CourseStatusDraft, domain.CourseStatusRetired
	seeded := []domain.Course{
		{ID: "course-1", CreatedBy: "teacher-1", Status: published},
		{ID: "course-2", CreatedBy: "teacher-1", Status: published},
		{ID: "course-3", CreatedBy: "teacher-2", Status: draft},
		{ID: "course-4", CreatedBy: "teacher-3", Status: retired},
		{ID: "course-5", CreatedBy: "admin-1", Status: published},
	}

	tests := []struct {
		name    string
		caller  domain.User
		courses []domain.Course
		want    []string
	}{
		{
			name:   "a student gets each creator of a published course once, never a draft-only or retired-only one",
			caller: studentCaller(), courses: seeded,
			want: []string{"admin-1", "teacher-1"},
		},
		{
			name:   "a teacher gets only themselves, whatever their courses' status",
			caller: otherTeacherCaller(), courses: seeded,
			want: []string{"teacher-2"},
		},
		{
			name:   "a teacher with no course gets no creators",
			caller: domain.User{ID: "teacher-9", Role: domain.RoleTeacher}, courses: seeded,
			want: []string{},
		},
		{
			name:   "an admin gets the creator of every course, whatever its status",
			caller: adminCaller(), courses: seeded,
			want: []string{"admin-1", "teacher-1", "teacher-2", "teacher-3"},
		},
		{
			name:   "no visible course means no creators",
			caller: studentCaller(), courses: nil,
			want: []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			courses := newFakeCourseRepository()
			for _, c := range tc.courses {
				courses.put(c)
			}
			svc := newCourseService(newFakeLearningPathRepository(), courses)

			got, err := svc.ListCourseCreatorIDs(context.Background(), tc.caller)

			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, got)
			assert.NotNil(t, got)
		})
	}
}

func TestCourseService_ListCourses(t *testing.T) {
	firstPage := domain.PageRequest{Limit: 20, Offset: 0}

	t.Run("a teacher lists every course of their own regardless of status", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1", Status: domain.CourseStatusDraft})
		courses.put(domain.Course{ID: "course-2", CreatedBy: "teacher-1", Status: domain.CourseStatusRetired})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		got, err := svc.ListCourses(context.Background(), teacherCaller(), domain.CourseListFilter{}, firstPage)

		require.NoError(t, err)
		assert.Len(t, got.Items, 2)
		assert.Equal(t, 2, got.Total)
	})

	t.Run("a teacher never sees another teacher's courses", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1", Status: domain.CourseStatusDraft})
		courses.put(domain.Course{ID: "course-2", CreatedBy: "teacher-2", Status: domain.CourseStatusDraft})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		got, err := svc.ListCourses(context.Background(), teacherCaller(), domain.CourseListFilter{}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "course-1", got.Items[0].ID)
	})

	t.Run("a teacher may name themselves as the creator", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1", Status: domain.CourseStatusDraft})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		got, err := svc.ListCourses(context.Background(), teacherCaller(), domain.CourseListFilter{CreatedBy: "teacher-1"}, firstPage)

		require.NoError(t, err)
		assert.Len(t, got.Items, 1)
	})

	t.Run("a teacher naming another creator is forbidden", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		_, err := svc.ListCourses(context.Background(), teacherCaller(), domain.CourseListFilter{CreatedBy: "teacher-2"}, firstPage)

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an admin lists every creator's courses, or narrows to one", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1", Status: domain.CourseStatusDraft})
		courses.put(domain.Course{ID: "course-2", CreatedBy: "teacher-2", Status: domain.CourseStatusDraft})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		all, err := svc.ListCourses(context.Background(), adminCaller(), domain.CourseListFilter{}, firstPage)
		require.NoError(t, err)
		narrowed, err := svc.ListCourses(context.Background(), adminCaller(), domain.CourseListFilter{CreatedBy: "teacher-2"}, firstPage)
		require.NoError(t, err)

		assert.Len(t, all.Items, 2)
		require.Len(t, narrowed.Items, 1)
		assert.Equal(t, "course-2", narrowed.Items[0].ID)
	})

	t.Run("a teacher narrows the list with a status filter", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1", Status: domain.CourseStatusDraft})
		courses.put(domain.Course{ID: "course-2", CreatedBy: "teacher-1", Status: domain.CourseStatusRetired})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		draft := domain.CourseStatusDraft
		got, err := svc.ListCourses(context.Background(), teacherCaller(), domain.CourseListFilter{Status: &draft}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "course-1", got.Items[0].ID)
	})

	t.Run("a student's results are always published regardless of the status filter given", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", Status: domain.CourseStatusDraft})
		courses.put(domain.Course{ID: "course-2", Status: domain.CourseStatusPublished})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		retired := domain.CourseStatusRetired
		got, err := svc.ListCourses(context.Background(), studentCaller(), domain.CourseListFilter{Status: &retired}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "course-2", got.Items[0].ID)
	})

	t.Run("a student's filters are evaluated against the published version, a teacher's against the live draft", func(t *testing.T) {
		courses := newFakeCourseRepository()
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.ListCourses(context.Background(), studentCaller(), domain.CourseListFilter{}, firstPage)
		require.NoError(t, err)
		assert.True(t, courses.lastFilter.PublishedView)

		_, err = svc.ListCourses(context.Background(), teacherCaller(), domain.CourseListFilter{}, firstPage)
		require.NoError(t, err)
		assert.False(t, courses.lastFilter.PublishedView)
	})

	t.Run("a student may filter by creator", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", CreatedBy: "teacher-1", Status: domain.CourseStatusPublished})
		courses.put(domain.Course{ID: "course-2", CreatedBy: "teacher-2", Status: domain.CourseStatusPublished})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		got, err := svc.ListCourses(context.Background(), studentCaller(), domain.CourseListFilter{CreatedBy: "teacher-2"}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "course-2", got.Items[0].ID)
	})

	t.Run("search text and levels narrow the results together", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(domain.Course{ID: "course-1", Title: "Fingerstyle Journey", Level: domain.DifficultyLevelBeginner, Status: domain.CourseStatusPublished})
		courses.put(domain.Course{ID: "course-2", Title: "Fingerstyle Mastery", Level: domain.DifficultyLevelExpert, Status: domain.CourseStatusPublished})
		courses.put(domain.Course{ID: "course-3", Title: "Strumming", Level: domain.DifficultyLevelBeginner, Status: domain.CourseStatusPublished})
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		got, err := svc.ListCourses(context.Background(), studentCaller(),
			domain.CourseListFilter{Query: "fingerstyle", Levels: []domain.DifficultyLevel{domain.DifficultyLevelBeginner}}, firstPage)

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "course-1", got.Items[0].ID)
		assert.Equal(t, 1, got.Total)
	})

	t.Run("returns the requested page and the filtered total", func(t *testing.T) {
		courses := newFakeCourseRepository()
		for _, id := range []string{"course-1", "course-2", "course-3"} {
			courses.put(domain.Course{ID: id, Title: id, Status: domain.CourseStatusPublished})
		}
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		got, err := svc.ListCourses(context.Background(), studentCaller(), domain.CourseListFilter{}, domain.PageRequest{Limit: 2, Offset: 2})

		require.NoError(t, err)
		assert.Equal(t, 3, got.Total)
		require.Len(t, got.Items, 1)
		assert.Equal(t, "course-3", got.Items[0].ID)
	})

	t.Run("listing when none exist returns an empty page", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		got, err := svc.ListCourses(context.Background(), teacherCaller(), domain.CourseListFilter{}, firstPage)

		require.NoError(t, err)
		assert.Empty(t, got.Items)
		assert.Zero(t, got.Total)
	})
}

func TestCourseService_LatestVersions(t *testing.T) {
	t.Run("resolves every given course's latest version in one call", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		versions := newFakeCourseVersionRepository()
		svc := newCourseService(newFakeLearningPathRepository(), courses, versions)

		_, err := svc.PublishCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)
		_, err = svc.PublishCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)

		got, err := svc.LatestVersions(context.Background(), []string{"course-1", "never-published"})

		require.NoError(t, err)
		require.Contains(t, got, "course-1")
		assert.Equal(t, 2, got["course-1"].VersionNumber)
		assert.NotContains(t, got, "never-published")
	})

	t.Run("an empty course id list resolves to an empty map", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		got, err := svc.LatestVersions(context.Background(), nil)

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

func fingerstyleCourseDraft() domain.Course {
	return domain.Course{
		ID: "course-1", CreatedBy: "teacher-1", Title: "Fingerstyle Journey",
		Summary: "From first chords to a repertoire.", Level: domain.DifficultyLevelBeginner,
		Status: domain.CourseStatusDraft, CreatedAt: fixedCreatedAt,
		Checkpoints: []domain.CourseCheckpoint{
			{Position: 1, LearningPathID: "path-01", EffectiveTitle: "Open Chords"},
			{Position: 2, LearningPathID: "path-02", EffectiveTitle: "Strumming Patterns"},
		},
	}
}

func TestCourseService_PublishCourse(t *testing.T) {
	t.Run("an admin publishes a course draft, producing version 1 and moving status to published", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		version, err := svc.PublishCourse(context.Background(), adminCaller(), "course-1")

		require.NoError(t, err)
		assert.Equal(t, "course-1", version.CourseID)
		assert.Equal(t, 1, version.VersionNumber)
		assert.Equal(t, "Fingerstyle Journey", version.TitleSnapshot)
		assert.Equal(t, "From first chords to a repertoire.", version.SummarySnapshot)
		assert.Equal(t, domain.DifficultyLevelBeginner, version.LevelSnapshot)
		require.Len(t, version.Checkpoints, 2)
		assert.Equal(t, "path-01", version.Checkpoints[0].LearningPathID)
		assert.True(t, version.AvailableForNewEnrollments)

		got, err := svc.GetCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)
		assert.Equal(t, domain.CourseStatusPublished, got.Status)
	})

	t.Run("publishing again produces version 2 and status stays published", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		versions := newFakeCourseVersionRepository()
		svc := newCourseService(newFakeLearningPathRepository(), courses, versions)

		_, err := svc.PublishCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)

		second, err := svc.PublishCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)
		assert.Equal(t, 2, second.VersionNumber)

		got, err := svc.GetCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)
		assert.Equal(t, domain.CourseStatusPublished, got.Status)
	})

	t.Run("a subsequent draft edit does not alter the already-published version", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01", Title: "Open Chords"})
		paths.put(domain.LearningPath{ID: "path-02", Title: "Strumming Patterns"})
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		versions := newFakeCourseVersionRepository()
		svc := newCourseService(paths, courses, versions)

		published, err := svc.PublishCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)

		_, err = svc.ReplaceCourse(context.Background(), teacherCaller(), "course-1", "Renamed", "New summary",
			domain.DifficultyLevelAdvanced, checkpointInputs("path-01"))
		require.NoError(t, err)

		latest, err := versions.GetLatestByCourseID(context.Background(), "course-1")
		require.NoError(t, err)
		assert.Equal(t, published, latest)
		assert.Equal(t, "Fingerstyle Journey", latest.TitleSnapshot)
	})

	t.Run("a teacher, including the creator, cannot publish a course", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.PublishCourse(context.Background(), teacherCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a student cannot publish a course", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.PublishCourse(context.Background(), studentCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("publishing a course that does not exist returns not found", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		_, err := svc.PublishCourse(context.Background(), adminCaller(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestCourseService_RetireCourse(t *testing.T) {
	t.Run("an admin retires a published course", func(t *testing.T) {
		courses := newFakeCourseRepository()
		published := fingerstyleCourseDraft()
		published.Status = domain.CourseStatusPublished
		courses.put(published)
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		retired, err := svc.RetireCourse(context.Background(), adminCaller(), "course-1")

		require.NoError(t, err)
		assert.Equal(t, domain.CourseStatusRetired, retired.Status)

		got, err := svc.GetCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)
		assert.Equal(t, domain.CourseStatusRetired, got.Status)
	})

	t.Run("a teacher, including the creator, cannot retire a course", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.RetireCourse(context.Background(), teacherCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a student cannot retire a course", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.RetireCourse(context.Background(), studentCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("retiring a course that does not exist returns not found", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		_, err := svc.RetireCourse(context.Background(), adminCaller(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestCourseService_GetPublishedCourse(t *testing.T) {
	t.Run("renders the latest published version as an outline, resolving items live", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		section := "Open position"
		paths.put(domain.LearningPath{ID: "path-01", Title: "Open Chords", Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: "node-1", Title: "E minor", ContentType: domain.ContentTypeVideo, SectionLabel: &section},
		}})
		paths.put(domain.LearningPath{ID: "path-02", Title: "Strumming Patterns"})
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		svc := newCourseService(paths, courses)

		_, err := svc.PublishCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)

		detail, err := svc.GetPublishedCourse(context.Background(), "course-1")

		require.NoError(t, err)
		assert.Equal(t, "Fingerstyle Journey", detail.Title)
		assert.Equal(t, "From first chords to a repertoire.", detail.Summary)
		assert.Equal(t, domain.DifficultyLevelBeginner, detail.Level)
		assert.Equal(t, domain.CourseStatusPublished, detail.Status)
		require.Len(t, detail.Checkpoints, 2)
		assert.Equal(t, 1, detail.Checkpoints[0].Position)
		assert.Equal(t, "Open Chords", detail.Checkpoints[0].Title)
		require.Len(t, detail.Checkpoints[0].Items, 1)
		assert.Equal(t, "E minor", detail.Checkpoints[0].Items[0].Title)
		require.NotNil(t, detail.Checkpoints[0].Items[0].SectionLabel)
		assert.Equal(t, "Open position", *detail.Checkpoints[0].Items[0].SectionLabel)
	})

	t.Run("a checkpoint's title reflects the pinned snapshot, not a later title override", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-01", Title: "Open Chords"})
		paths.put(domain.LearningPath{ID: "path-02", Title: "Strumming Patterns"})
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		svc := newCourseService(paths, courses)

		_, err := svc.PublishCourse(context.Background(), adminCaller(), "course-1")
		require.NoError(t, err)

		_, err = svc.ReplaceCourse(context.Background(), teacherCaller(), "course-1", "Fingerstyle Journey",
			"From first chords to a repertoire.", domain.DifficultyLevelBeginner,
			[]application.CheckpointInput{{LearningPathID: "path-01", Title: strPtr("Renamed after publish")}, {LearningPathID: "path-02"}})
		require.NoError(t, err)

		detail, err := svc.GetPublishedCourse(context.Background(), "course-1")

		require.NoError(t, err)
		assert.Equal(t, "Open Chords", detail.Checkpoints[0].Title)
	})

	t.Run("a course that has never been published returns not found", func(t *testing.T) {
		courses := newFakeCourseRepository()
		courses.put(fingerstyleCourseDraft())
		svc := newCourseService(newFakeLearningPathRepository(), courses)

		_, err := svc.GetPublishedCourse(context.Background(), "course-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a course that does not exist returns not found", func(t *testing.T) {
		svc := newCourseService(newFakeLearningPathRepository(), newFakeCourseRepository())

		_, err := svc.GetPublishedCourse(context.Background(), "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
