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

// enrollmentFixture is a student path service over fakes an enrollment test
// can seed and inspect.
type enrollmentFixture struct {
	users        *fakeUserRepository
	paths        *fakeLearningPathRepository
	studentPaths *fakeStudentPathRepository
	state        *fakeStudentLearningStateRepository
	completion   *fakeCompletionStateReader
	svc          *application.StudentPathService
}

func newEnrollmentFixture() enrollmentFixture {
	f := enrollmentFixture{
		users:        newFakeUserRepository(),
		paths:        newFakeLearningPathRepository(),
		studentPaths: newFakeStudentPathRepository(),
		state:        newFakeStudentLearningStateRepository(),
		completion:   newFakeCompletionStateReader(),
	}
	f.users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
	level := domain.DifficultyLevelBeginner
	template := threeItemTemplate()
	template.TeacherID = "teacher-9"
	template.Summary = strPtr("Your first chords")
	template.Level = &level
	template.ThumbnailURL = strPtr("https://cdn.motifpath.io/thumbnails/open-chords.png")
	f.paths.put(template)
	f.svc = newStudentPathService(f.users, f.paths, f.studentPaths, publishedVersions("node-01", "node-02", "node-03"), f.state, f.completion)
	return f
}

func student() domain.User { return domain.User{ID: "student-1", Role: domain.RoleStudent} }

func (f enrollmentFixture) current(t *testing.T) domain.StudentLearningState {
	t.Helper()
	state, err := f.state.GetByStudentID(context.Background(), "student-1")
	require.NoError(t, err)
	return state
}

func (f enrollmentFixture) activeCopies(t *testing.T) []domain.StudentPath {
	t.Helper()
	held, err := f.studentPaths.ListActiveStandaloneByStudentID(context.Background(), "student-1")
	require.NoError(t, err)
	return held
}

func TestStudentPathService_EnrollInLearningPath(t *testing.T) {
	t.Run("a learner enrolls in a published path, owning and assigning the copy", func(t *testing.T) {
		f := newEnrollmentFixture()

		sp, created, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")

		require.NoError(t, err)
		assert.True(t, created)
		assert.Equal(t, "student-1", sp.StudentID)
		assert.Equal(t, "student-1", sp.AssignedBy)
		assert.Equal(t, "path-1", sp.SourceTemplateID)
		require.Len(t, sp.Items, 3)
		require.NotNil(t, f.current(t).CurrentStandalonePathID)
		assert.Equal(t, sp.ID, *f.current(t).CurrentStandalonePathID)
	})

	t.Run("enrolling makes the path current even while a course is current", func(t *testing.T) {
		f := newEnrollmentFixture()
		enrollmentID := "enrollment-1"
		require.NoError(t, f.state.Upsert(context.Background(), domain.StudentLearningState{StudentID: "student-1", CurrentCourseEnrollmentID: &enrollmentID}))

		sp, _, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")

		require.NoError(t, err)
		state := f.current(t)
		assert.Nil(t, state.CurrentCourseEnrollmentID)
		require.NotNil(t, state.CurrentStandalonePathID)
		assert.Equal(t, sp.ID, *state.CurrentStandalonePathID)
	})

	t.Run("a teacher enrolls like any learner", func(t *testing.T) {
		f := newEnrollmentFixture()
		f.users.put(domain.User{ID: "teacher-1", Role: domain.RoleTeacher})

		_, created, err := f.svc.EnrollInLearningPath(context.Background(), teacherCaller(), "path-1")

		require.NoError(t, err)
		assert.True(t, created)
	})

	t.Run("re-enrolling reuses the active copy and makes it current again", func(t *testing.T) {
		f := newEnrollmentFixture()
		first, _, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")
		require.NoError(t, err)
		other := "some-other-path"
		require.NoError(t, f.state.Upsert(context.Background(), domain.StudentLearningState{StudentID: "student-1", CurrentStandalonePathID: &other}))

		again, created, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")

		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, first.ID, again.ID)
		assert.Len(t, f.activeCopies(t), 1)
		assert.Equal(t, first.ID, *f.current(t).CurrentStandalonePathID)
	})

	t.Run("a copy staff assigned is reused too", func(t *testing.T) {
		f := newEnrollmentFixture()
		assigned, _, err := f.svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-1")
		require.NoError(t, err)

		again, created, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")

		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, assigned.ID, again.ID)
	})

	t.Run("an archived copy is not reused: enrolling again makes a fresh one", func(t *testing.T) {
		f := newEnrollmentFixture()
		first, _, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")
		require.NoError(t, err)
		require.NoError(t, f.studentPaths.Archive(context.Background(), first.ID, fixedAssignedAt.Add(time.Hour)))

		again, created, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")

		require.NoError(t, err)
		assert.True(t, created)
		assert.NotEqual(t, first.ID, again.ID)
	})

	for _, tc := range []struct {
		name   string
		status domain.LearningPathStatus
		id     string
	}{
		{name: "enrolling in a draft path is not found", status: domain.LearningPathStatusDraft, id: "path-1"},
		{name: "enrolling in a path that does not exist is not found", status: domain.LearningPathStatusPublished, id: "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newEnrollmentFixture()
			template, err := f.paths.GetByID(context.Background(), "path-1")
			require.NoError(t, err)
			template.Status = tc.status
			f.paths.put(template)

			_, _, err = f.svc.EnrollInLearningPath(context.Background(), student(), tc.id)

			require.ErrorIs(t, err, domain.ErrNotFound)
			assert.Empty(t, f.activeCopies(t))
		})
	}
}

func TestStudentPathService_AssignReusesActiveCopy(t *testing.T) {
	f := newEnrollmentFixture()
	enrolled, _, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")
	require.NoError(t, err)

	assigned, created, err := f.svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-1")

	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, enrolled.ID, assigned.ID)
	assert.Len(t, f.activeCopies(t), 1)
}

func TestStudentPathService_CopiesRecordPresentation(t *testing.T) {
	cases := []struct {
		name string
		copy func(f enrollmentFixture) (domain.StudentPath, error)
	}{
		{name: "a self-enrolled copy", copy: func(f enrollmentFixture) (domain.StudentPath, error) {
			sp, _, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")
			return sp, err
		}},
		{name: "a staff-assigned copy", copy: func(f enrollmentFixture) (domain.StudentPath, error) {
			sp, _, err := f.svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-1")
			return sp, err
		}},
		{name: "a course checkpoint copy", copy: func(f enrollmentFixture) (domain.StudentPath, error) {
			template, err := f.paths.GetByID(context.Background(), "path-1")
			if err != nil {
				return domain.StudentPath{}, err
			}
			return f.svc.CopyTemplateForCheckpoint(context.Background(), "student-1", template, "student-1", "enrollment-1", 1)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newEnrollmentFixture()

			sp, err := tc.copy(f)

			require.NoError(t, err)
			assert.Equal(t, strPtr("Your first chords"), sp.SummarySnapshot)
			require.NotNil(t, sp.LevelSnapshot)
			assert.Equal(t, domain.DifficultyLevelBeginner, *sp.LevelSnapshot)
			assert.Equal(t, strPtr("https://cdn.motifpath.io/thumbnails/open-chords.png"), sp.ThumbnailURLSnapshot)
			assert.Equal(t, strPtr("teacher-9"), sp.CreatedBySnapshot)

			stored, err := f.studentPaths.GetByID(context.Background(), sp.ID)
			require.NoError(t, err)
			assert.Equal(t, sp.SummarySnapshot, stored.SummarySnapshot)
		})
	}

	t.Run("the snapshot survives the template being edited", func(t *testing.T) {
		f := newEnrollmentFixture()
		sp, _, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")
		require.NoError(t, err)
		template, err := f.paths.GetByID(context.Background(), "path-1")
		require.NoError(t, err)
		template.Summary = strPtr("Open chords, reworked")
		f.paths.put(template)

		stored, err := f.studentPaths.GetByID(context.Background(), sp.ID)

		require.NoError(t, err)
		assert.Equal(t, strPtr("Your first chords"), stored.SummarySnapshot)
	})
}

func TestStudentPathService_CompletedCounts(t *testing.T) {
	f := newEnrollmentFixture()
	sp, _, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")
	require.NoError(t, err)
	// Progress is per content node: a lesson completed anywhere counts here.
	f.completion.set("student-1", "node-01", domain.CompletionStatusCompleted)
	f.completion.set("student-1", "node-02", domain.CompletionStatusCompleted)
	f.completion.set("student-1", "node-03", domain.CompletionStatusInProgress)

	counts, err := f.svc.CompletedCounts(context.Background(), "student-1", []domain.StudentPath{sp})

	require.NoError(t, err)
	assert.Equal(t, map[string]int{sp.ID: 2}, counts)
}
