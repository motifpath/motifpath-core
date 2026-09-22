package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

var fixedEnrolledAt = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

// courseEnrollmentFixtures bundles the repositories a
// CourseEnrollmentService test needs, plus a StudentPathService instance
// wired to the same StudentPath/state/enrollment repositories so
// self-enrollment's copy-on-assign checkpoint 1 and any current-pointer
// assertions are visible to the test.
type courseEnrollmentFixtures struct {
	paths          *fakeLearningPathRepository
	courses        *fakeCourseRepository
	courseVersions *fakeCourseVersionRepository
	studentPaths   *fakeStudentPathRepository
	enrollments    *fakeCourseEnrollmentRepository
	state          *fakeStudentLearningStateRepository
	// completion is optional — nil defaults to a fresh, empty
	// fakeCompletionStateReader. Tests that need to simulate item
	// completion (checkpoint advance / course completion) set it
	// explicitly so both CourseEnrollmentService and its internal
	// StudentPathService see the same completion data.
	completion *fakeCompletionStateReader
}

func newCourseEnrollmentService(f courseEnrollmentFixtures) *application.CourseEnrollmentService {
	users := newFakeUserRepository()
	completion := f.completion
	if completion == nil {
		completion = newFakeCompletionStateReader()
	}
	studentPathSvc := application.NewStudentPathService(
		users, f.paths, f.studentPaths, publishedVersions("node-01", "node-02", "node-03"), f.state, f.enrollments, f.courseVersions,
		newFakeContentNodeRepository(), newFakeExerciseRepository(), completion,
		idSequence(), func() time.Time { return fixedEnrolledAt },
	)
	return application.NewCourseEnrollmentService(f.courses, f.courseVersions, f.paths, f.studentPaths, f.enrollments, studentPathSvc, f.state, completion, idSequence(), func() time.Time { return fixedEnrolledAt })
}

// publishedCourseWithCheckpoint seeds a published Course plus one
// CourseVersion pinned to a single checkpoint pointing at learningPathID,
// available for new enrollments.
func publishedCourseWithCheckpoint(courses *fakeCourseRepository, versions *fakeCourseVersionRepository, courseID, learningPathID string) {
	courses.put(domain.Course{ID: courseID, Title: "Fingerstyle Journey", Summary: "Summary", Level: domain.DifficultyLevelBeginner, Status: domain.CourseStatusPublished})
	_ = versions.Create(context.Background(), domain.CourseVersion{
		ID: "version-" + courseID, CourseID: courseID, VersionNumber: 1,
		TitleSnapshot: "Fingerstyle Journey", SummarySnapshot: "Summary", LevelSnapshot: domain.DifficultyLevelBeginner,
		Checkpoints:                []domain.CourseVersionCheckpoint{{Position: 1, LearningPathID: learningPathID, EffectiveTitle: "Open Chords"}},
		PublishedAt:                fixedEnrolledAt,
		AvailableForNewEnrollments: true,
	})
}

func TestCourseEnrollmentService_CreateCourseEnrollment(t *testing.T) {
	t.Run("a student self-enrolls in a published course, copying checkpoint 1 into a new StudentPath", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		svc := newCourseEnrollmentService(f)

		enrollment, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")

		require.NoError(t, err)
		assert.Equal(t, "student-1", enrollment.StudentID)
		assert.Equal(t, "course-1", enrollment.CourseID)
		assert.Equal(t, "Fingerstyle Journey", enrollment.CourseTitle)
		assert.Equal(t, 1, enrollment.CourseVersionNumber)
		assert.Equal(t, domain.CourseEnrollmentStatusActive, enrollment.Status)
		require.NotNil(t, enrollment.ActiveCheckpointPosition)
		assert.Equal(t, 1, *enrollment.ActiveCheckpointPosition)
		require.NotNil(t, enrollment.ActiveCheckpointStudentPathID)

		sp, err := f.studentPaths.GetByID(context.Background(), *enrollment.ActiveCheckpointStudentPathID)
		require.NoError(t, err)
		assert.Equal(t, "path-1", sp.SourceTemplateID)
		require.NotNil(t, sp.SourceCourseEnrollmentID)
		assert.Equal(t, enrollment.ID, *sp.SourceCourseEnrollmentID)
	})

	t.Run("sets the new enrollment as current only if nothing is currently set", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		svc := newCourseEnrollmentService(f)

		enrollment, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")

		require.NoError(t, err)
		got, err := f.state.GetByStudentID(context.Background(), "student-1")
		require.NoError(t, err)
		require.NotNil(t, got.CurrentCourseEnrollmentID)
		assert.Equal(t, enrollment.ID, *got.CurrentCourseEnrollmentID)
	})

	t.Run("does not switch the student away from an already-current standalone path", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		require.NoError(t, f.state.Upsert(context.Background(), domain.StudentLearningState{StudentID: "student-1"}.WithCurrentStandalonePath("existing-path")))
		svc := newCourseEnrollmentService(f)

		_, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")

		require.NoError(t, err)
		got, err := f.state.GetByStudentID(context.Background(), "student-1")
		require.NoError(t, err)
		require.NotNil(t, got.CurrentStandalonePathID)
		assert.Equal(t, "existing-path", *got.CurrentStandalonePathID)
	})

	t.Run("only students may self-enroll", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		svc := newCourseEnrollmentService(f)

		_, err := svc.CreateCourseEnrollment(context.Background(), teacherCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("a course that does not exist is not found", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		svc := newCourseEnrollmentService(f)

		_, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "missing-course")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a draft course is not found", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.courses.put(domain.Course{ID: "course-1", Title: "Draft Course", Status: domain.CourseStatusDraft})
		svc := newCourseEnrollmentService(f)

		_, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a retired course is not found", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		f.courses.put(domain.Course{ID: "course-1", Title: "Fingerstyle Journey", Status: domain.CourseStatusRetired})
		svc := newCourseEnrollmentService(f)

		_, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a course whose latest version is unavailable for new enrollments is not found", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		f.courses.put(domain.Course{ID: "course-1", Title: "Fingerstyle Journey", Status: domain.CourseStatusPublished})
		_ = f.courseVersions.Create(context.Background(), domain.CourseVersion{
			ID: "version-1", CourseID: "course-1", VersionNumber: 1, TitleSnapshot: "Fingerstyle Journey",
			Checkpoints:                []domain.CourseVersionCheckpoint{{Position: 1, LearningPathID: "path-1"}},
			AvailableForNewEnrollments: false,
		})
		svc := newCourseEnrollmentService(f)

		_, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a second active enrollment in the same course is a conflict", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		svc := newCourseEnrollmentService(f)
		_, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")
		require.NoError(t, err)

		_, err = svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")

		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("re-enrolling after abandoning is allowed", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		svc := newCourseEnrollmentService(f)
		first, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")
		require.NoError(t, err)
		_, err = svc.AbandonCourseEnrollment(context.Background(), studentCaller(), first.ID)
		require.NoError(t, err)

		second, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")

		require.NoError(t, err)
		assert.NotEqual(t, first.ID, second.ID)
	})
}

func TestCourseEnrollmentService_ListMyCourseEnrollments(t *testing.T) {
	t.Run("returns every enrollment the student has ever held", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		svc := newCourseEnrollmentService(f)
		_, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")
		require.NoError(t, err)

		enrollments, err := svc.ListMyCourseEnrollments(context.Background(), studentCaller())

		require.NoError(t, err)
		require.Len(t, enrollments, 1)
	})

	t.Run("only students hold course enrollments", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		svc := newCourseEnrollmentService(f)

		_, err := svc.ListMyCourseEnrollments(context.Background(), teacherCaller())

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestCourseEnrollmentService_AbandonCourseEnrollment(t *testing.T) {
	t.Run("abandoning the student's only current enrollment succeeds and clears the current pointer", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		svc := newCourseEnrollmentService(f)
		enrollment, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")
		require.NoError(t, err)

		abandoned, err := svc.AbandonCourseEnrollment(context.Background(), studentCaller(), enrollment.ID)

		require.NoError(t, err)
		assert.Equal(t, domain.CourseEnrollmentStatusAbandoned, abandoned.Status)
		got, err := f.state.GetByStudentID(context.Background(), "student-1")
		require.NoError(t, err)
		assert.False(t, got.HasCurrent())
	})

	t.Run("abandoning the current enrollment while another eligible standalone path exists is a conflict", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		f.studentPaths.byID = map[string]domain.StudentPath{}
		otherPath := domain.StudentPath{ID: "other-path", StudentID: "student-1", SourceTemplateID: "path-1", AssignedBy: "teacher-1", AssignedAt: fixedEnrolledAt}
		require.NoError(t, f.studentPaths.Create(context.Background(), otherPath))
		svc := newCourseEnrollmentService(f)
		enrollment, err := svc.CreateCourseEnrollment(context.Background(), studentCaller(), "course-1")
		require.NoError(t, err)

		_, err = svc.AbandonCourseEnrollment(context.Background(), studentCaller(), enrollment.ID)

		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("only students hold course enrollments", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		svc := newCourseEnrollmentService(f)

		_, err := svc.AbandonCourseEnrollment(context.Background(), teacherCaller(), "enrollment-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("no active enrollment with the given id belonging to the caller is not found", func(t *testing.T) {
		f := courseEnrollmentFixtures{
			paths: newFakeLearningPathRepository(), courses: newFakeCourseRepository(), courseVersions: newFakeCourseVersionRepository(),
			studentPaths: newFakeStudentPathRepository(), enrollments: newFakeCourseEnrollmentRepository(), state: newFakeStudentLearningStateRepository(),
		}
		f.paths.put(threeItemTemplate())
		publishedCourseWithCheckpoint(f.courses, f.courseVersions, "course-1", "path-1")
		svc := newCourseEnrollmentService(f)
		enrollment, err := svc.CreateCourseEnrollment(context.Background(), domain.User{ID: "bob", Role: domain.RoleStudent}, "course-1")
		require.NoError(t, err)

		_, err = svc.AbandonCourseEnrollment(context.Background(), studentCaller(), enrollment.ID)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
