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

// updateFixture is a learner holding a standalone copy of path-1 (three
// lessons over node-01..03) whose template an author can then change.
// node-04 is published too, so a test can add it to the template.
type updateFixture struct {
	paths        *fakeLearningPathRepository
	studentPaths *fakeStudentPathRepository
	state        *fakeStudentLearningStateRepository
	completion   *fakeCompletionStateReader
	svc          *application.StudentPathService
	copy         domain.StudentPath
}

func newUpdateFixture(t *testing.T) updateFixture {
	t.Helper()
	users := newFakeUserRepository()
	users.put(student())
	f := updateFixture{
		paths:        newFakeLearningPathRepository(),
		studentPaths: newFakeStudentPathRepository(),
		state:        newFakeStudentLearningStateRepository(),
		completion:   newFakeCompletionStateReader(),
	}
	f.paths.put(threeItemTemplate())
	f.svc = newStudentPathService(users, f.paths, f.studentPaths, publishedVersions("node-01", "node-02", "node-03", "node-04"), f.state, f.completion)
	sp, _, err := f.svc.EnrollInLearningPath(context.Background(), student(), "path-1")
	require.NoError(t, err)
	f.copy = sp
	return f
}

// changeTemplate applies change to path-1 as an author's replace would.
func (f updateFixture) changeTemplate(t *testing.T, change func(*domain.LearningPath)) {
	t.Helper()
	template, err := f.paths.GetByID(context.Background(), "path-1")
	require.NoError(t, err)
	change(&template)
	f.paths.put(template)
}

func (f updateFixture) stored(t *testing.T) domain.StudentPath {
	t.Helper()
	sp, err := f.studentPaths.GetByID(context.Background(), f.copy.ID)
	require.NoError(t, err)
	return sp
}

func (f updateFixture) updateAvailable(t *testing.T) bool {
	t.Helper()
	available, err := f.svc.UpdateAvailable(context.Background(), "student-1", []domain.StudentPath{f.stored(t)})
	require.NoError(t, err)
	return available[f.copy.ID]
}

func addLesson04(p *domain.LearningPath) {
	p.Items = append(p.Items, domain.LearningPathItem{Position: 4, ContentNodeID: "node-04"})
}

func TestStudentPathService_UpdateAvailable(t *testing.T) {
	thumbnail := "https://cdn.motifpath.io/thumbnails/new.png"
	tests := []struct {
		name  string
		setup func(t *testing.T, f updateFixture)
		want  bool
	}{
		{"a copy that matches its template", func(*testing.T, updateFixture) {}, false},
		{"a lesson added to the template", func(t *testing.T, f updateFixture) { f.changeTemplate(t, addLesson04) }, true},
		{"a new thumbnail on the template", func(t *testing.T, f updateFixture) {
			f.changeTemplate(t, func(p *domain.LearningPath) { p.ThumbnailURL = &thumbnail })
		}, true},
		{"a template that is no longer published", func(t *testing.T, f updateFixture) {
			f.changeTemplate(t, func(p *domain.LearningPath) { addLesson04(p); p.Status = domain.LearningPathStatusDraft })
		}, false},
		{"a template that was deleted", func(t *testing.T, f updateFixture) {
			require.NoError(t, f.paths.Delete(context.Background(), "path-1"))
		}, false},
		{"a finished copy", func(t *testing.T, f updateFixture) {
			for _, node := range []string{"node-01", "node-02", "node-03"} {
				f.completion.set("student-1", node, domain.CompletionStatusCompleted)
			}
			f.changeTemplate(t, addLesson04)
		}, false},
		{"an archived copy", func(t *testing.T, f updateFixture) {
			f.changeTemplate(t, addLesson04)
			require.NoError(t, f.studentPaths.Archive(context.Background(), f.copy.ID, fixedAssignedAt))
		}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newUpdateFixture(t)
			tc.setup(t, f)

			assert.Equal(t, tc.want, f.updateAvailable(t))
		})
	}

	t.Run("a course part's copy is never offered an update on its own", func(t *testing.T) {
		f := newUpdateFixture(t)
		enrollmentID, position := "enrollment-1", 1
		part := f.copy
		part.ID = "part-copy"
		part.SourceCourseEnrollmentID, part.CourseCheckpointPosition = &enrollmentID, &position
		require.NoError(t, f.studentPaths.Create(context.Background(), part))
		f.changeTemplate(t, addLesson04)

		available, err := f.svc.UpdateAvailable(context.Background(), "student-1", []domain.StudentPath{part})

		require.NoError(t, err)
		assert.False(t, available["part-copy"])
	})
}

func TestStudentPathService_UpdateStandalonePathToLatest(t *testing.T) {
	ctx := context.Background()

	t.Run("gives the copy the template's current lessons and presentation", func(t *testing.T) {
		f := newUpdateFixture(t)
		summary := "Revised"
		f.changeTemplate(t, func(p *domain.LearningPath) {
			addLesson04(p)
			p.Title = "Beginner Guitar, revised"
			p.Summary = &summary
		})

		updated, err := f.svc.UpdateStandalonePathToLatest(ctx, student(), f.copy.ID)

		require.NoError(t, err)
		assert.Equal(t, f.copy.ID, updated.ID)
		assert.Equal(t, "Beginner Guitar, revised", updated.Title)
		assert.Equal(t, &summary, updated.SummarySnapshot)
		require.Len(t, updated.Items, 4)
		assert.Equal(t, "node-04", updated.Items[3].ContentNodeID)
		assert.Equal(t, "version-node-04", updated.Items[3].ContentNodeVersionID)
		assert.Equal(t, updated, f.stored(t))
		assert.False(t, f.updateAvailable(t))
	})

	t.Run("keeps the assigner, and the copy current when it was", func(t *testing.T) {
		f := newUpdateFixture(t)
		f.changeTemplate(t, addLesson04)

		updated, err := f.svc.UpdateStandalonePathToLatest(ctx, student(), f.copy.ID)

		require.NoError(t, err)
		assert.Equal(t, f.copy.AssignedBy, updated.AssignedBy)
		state, err := f.state.GetByStudentID(ctx, "student-1")
		require.NoError(t, err)
		require.NotNil(t, state.CurrentStandalonePathID)
		assert.Equal(t, f.copy.ID, *state.CurrentStandalonePathID)
	})

	t.Run("a copy with nothing to update is returned unchanged", func(t *testing.T) {
		f := newUpdateFixture(t)

		updated, err := f.svc.UpdateStandalonePathToLatest(ctx, student(), f.copy.ID)

		require.NoError(t, err)
		assert.Equal(t, f.stored(t), updated)
	})

	refusals := []struct {
		name    string
		caller  domain.User
		setup   func(t *testing.T, f updateFixture) string
		wantErr error
	}{
		{"another learner's copy", domain.User{ID: "student-2", Role: domain.RoleStudent}, func(t *testing.T, f updateFixture) string {
			return f.copy.ID
		}, domain.ErrNotFound},
		{"an unknown copy", student(), func(*testing.T, updateFixture) string { return "missing" }, domain.ErrNotFound},
		{"an archived copy", student(), func(t *testing.T, f updateFixture) string {
			require.NoError(t, f.studentPaths.Archive(ctx, f.copy.ID, fixedAssignedAt.Add(time.Hour)))
			return f.copy.ID
		}, domain.ErrNotFound},
		{"a course part's copy", student(), func(t *testing.T, f updateFixture) string {
			enrollmentID, position := "enrollment-1", 1
			part := f.copy
			part.ID = "part-copy"
			part.SourceCourseEnrollmentID, part.CourseCheckpointPosition = &enrollmentID, &position
			require.NoError(t, f.studentPaths.Create(ctx, part))
			return part.ID
		}, domain.ErrNotFound},
		{"a finished copy", student(), func(t *testing.T, f updateFixture) string {
			for _, node := range []string{"node-01", "node-02", "node-03"} {
				f.completion.set("student-1", node, domain.CompletionStatusCompleted)
			}
			f.changeTemplate(t, addLesson04)
			return f.copy.ID
		}, domain.ErrConflict},
		{"a copy whose template is no longer published", student(), func(t *testing.T, f updateFixture) string {
			f.changeTemplate(t, func(p *domain.LearningPath) { p.Status = domain.LearningPathStatusDraft })
			return f.copy.ID
		}, domain.ErrConflict},
		{"a copy whose template was deleted", student(), func(t *testing.T, f updateFixture) string {
			require.NoError(t, f.paths.Delete(ctx, "path-1"))
			return f.copy.ID
		}, domain.ErrConflict},
	}
	for _, tc := range refusals {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			f := newUpdateFixture(t)
			id := tc.setup(t, f)

			_, err := f.svc.UpdateStandalonePathToLatest(ctx, tc.caller, id)

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// courseUpdateFixture is alice enrolled in course-1 version 1 (path-1 over
// node-01, then path-2 over node-02), on part 1 as her current course.
// path-3 over node-03 exists for newer versions to use.
type courseUpdateFixture struct {
	checkpointAdvanceFixtures
	svc *application.CourseEnrollmentService
}

func newCourseUpdateFixture() courseUpdateFixture {
	f := newCheckpointAdvanceFixtures()
	f.versions = publishedVersions("node-01", "node-02", "node-03", "node-04")
	seedActiveCheckpoint1(f)
	f.paths.put(singleItemTemplate("path-3", "node-03"))
	for _, id := range []string{"path-1", "path-2", "path-3"} {
		template, _ := f.paths.GetByID(context.Background(), id)
		template.Status = domain.LearningPathStatusPublished
		f.paths.put(template)
	}
	return courseUpdateFixture{checkpointAdvanceFixtures: f, svc: f.courseEnrollmentService(f.studentPathService())}
}

// publish adds course-1's next version, its checkpoints over pathIDs in order.
func (f courseUpdateFixture) publish(versionNumber int, title string, thumbnail *string, pathIDs ...string) {
	checkpoints := make([]domain.CourseVersionCheckpoint, len(pathIDs))
	for i, id := range pathIDs {
		checkpoints[i] = domain.CourseVersionCheckpoint{Position: i + 1, LearningPathID: id}
	}
	_ = f.courseVersions.Create(context.Background(), domain.CourseVersion{
		ID: "version-course-1-" + title, CourseID: "course-1", VersionNumber: versionNumber,
		TitleSnapshot: title, ThumbnailURLSnapshot: thumbnail, Checkpoints: checkpoints,
		PublishedAt: fixedEnrolledAt, AvailableForNewEnrollments: true,
	})
}

func (f courseUpdateFixture) enrollment(t *testing.T) domain.CourseEnrollment {
	t.Helper()
	e, err := f.enrollments.GetByID(context.Background(), "enrollment-1")
	require.NoError(t, err)
	return e
}

func (f courseUpdateFixture) activePart(t *testing.T) domain.StudentPath {
	t.Helper()
	e := f.enrollment(t)
	require.NotNil(t, e.ActiveCheckpointStudentPathID)
	sp, err := f.studentPaths.GetByID(context.Background(), *e.ActiveCheckpointStudentPathID)
	require.NoError(t, err)
	return sp
}

func (f courseUpdateFixture) updateAvailable(t *testing.T) bool {
	t.Helper()
	available, err := f.svc.UpdateAvailable(context.Background(), []domain.CourseEnrollment{f.enrollment(t)})
	require.NoError(t, err)
	return available["enrollment-1"]
}

func TestCourseEnrollmentService_UpdateAvailable(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, f courseUpdateFixture)
		want  bool
	}{
		{"an enrollment on the latest version whose part matches its path", func(*testing.T, courseUpdateFixture) {}, false},
		{"a newer course version", func(_ *testing.T, f courseUpdateFixture) {
			f.publish(2, "Fingerstyle Journey", nil, "path-1", "path-2")
		}, true},
		{"the active part's path has changed", func(_ *testing.T, f courseUpdateFixture) {
			template, _ := f.paths.GetByID(context.Background(), "path-1")
			template.Items = append(template.Items, domain.LearningPathItem{Position: 2, ContentNodeID: "node-04"})
			f.paths.put(template)
		}, true},
		{"an abandoned enrollment", func(t *testing.T, f courseUpdateFixture) {
			f.publish(2, "Fingerstyle Journey", nil, "path-1", "path-2")
			require.NoError(t, f.enrollments.Abandon(context.Background(), "enrollment-1"))
		}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newCourseUpdateFixture()
			tc.setup(t, f)

			assert.Equal(t, tc.want, f.updateAvailable(t))
		})
	}
}

func TestCourseEnrollmentService_UpdateCourseEnrollmentToLatest(t *testing.T) {
	ctx := context.Background()

	t.Run("pins the enrollment to the latest version, with its title and thumbnail", func(t *testing.T) {
		f := newCourseUpdateFixture()
		thumbnail := "https://cdn.motifpath.io/thumbnails/new.png"
		f.publish(2, "Fingerstyle Journey, revised", &thumbnail, "path-1", "path-2")

		updated, err := f.svc.UpdateCourseEnrollmentToLatest(ctx, studentCaller(), "enrollment-1")

		require.NoError(t, err)
		assert.Equal(t, 2, updated.CourseVersionNumber)
		assert.Equal(t, "Fingerstyle Journey, revised", updated.CourseTitle)
		assert.Equal(t, &thumbnail, updated.CourseThumbnailURL)
		assert.Equal(t, updated, f.enrollment(t))
		assert.False(t, f.updateAvailable(t))
	})

	t.Run("keeps the learner on their part, at its position in the new version", func(t *testing.T) {
		f := newCourseUpdateFixture()
		f.publish(2, "Fingerstyle Journey", nil, "path-3", "path-1", "path-2")

		updated, err := f.svc.UpdateCourseEnrollmentToLatest(ctx, studentCaller(), "enrollment-1")

		require.NoError(t, err)
		require.NotNil(t, updated.ActiveCheckpointPosition)
		assert.Equal(t, 2, *updated.ActiveCheckpointPosition)
		part := f.activePart(t)
		assert.Equal(t, "checkpoint-path-1", part.ID)
		require.NotNil(t, part.CourseCheckpointPosition)
		assert.Equal(t, 2, *part.CourseCheckpointPosition)
	})

	t.Run("updates the active part's copy to its path's current lessons", func(t *testing.T) {
		f := newCourseUpdateFixture()
		template, _ := f.paths.GetByID(ctx, "path-1")
		template.Items = append(template.Items, domain.LearningPathItem{Position: 2, ContentNodeID: "node-04"})
		f.paths.put(template)

		_, err := f.svc.UpdateCourseEnrollmentToLatest(ctx, studentCaller(), "enrollment-1")

		require.NoError(t, err)
		part := f.activePart(t)
		assert.Equal(t, "checkpoint-path-1", part.ID)
		require.Len(t, part.Items, 2)
		assert.Equal(t, "node-04", part.Items[1].ContentNodeID)
		assert.False(t, f.updateAvailable(t))
	})

	t.Run("when the active part is removed, continues at the first part not finished", func(t *testing.T) {
		f := newCourseUpdateFixture()
		f.completion.set("student-1", "node-02", domain.CompletionStatusCompleted)
		f.publish(2, "Fingerstyle Journey", nil, "path-2", "path-3")

		updated, err := f.svc.UpdateCourseEnrollmentToLatest(ctx, studentCaller(), "enrollment-1")

		require.NoError(t, err)
		require.NotNil(t, updated.ActiveCheckpointPosition)
		assert.Equal(t, 2, *updated.ActiveCheckpointPosition)
		part := f.activePart(t)
		assert.Equal(t, "path-3", part.SourceTemplateID)
		require.NotNil(t, part.CourseCheckpointPosition)
		assert.Equal(t, 2, *part.CourseCheckpointPosition)
		removed, err := f.studentPaths.GetByID(ctx, "checkpoint-path-1")
		require.NoError(t, err)
		assert.NotNil(t, removed.ArchivedAt)
	})

	t.Run("when every part of the new version is already finished, the course completes", func(t *testing.T) {
		f := newCourseUpdateFixture()
		f.completion.set("student-1", "node-02", domain.CompletionStatusCompleted)
		f.publish(2, "Fingerstyle Journey", nil, "path-2")

		updated, err := f.svc.UpdateCourseEnrollmentToLatest(ctx, studentCaller(), "enrollment-1")

		require.NoError(t, err)
		assert.Equal(t, domain.CourseEnrollmentStatusCompleted, updated.Status)
		assert.Equal(t, 2, updated.CourseVersionNumber)
	})

	t.Run("never changes which course or path is current", func(t *testing.T) {
		f := newCourseUpdateFixture()
		_ = f.state.Upsert(ctx, domain.StudentLearningState{StudentID: "student-1"}.WithCurrentStandalonePath("other-path"))
		f.publish(2, "Fingerstyle Journey", nil, "path-3", "path-1", "path-2")

		_, err := f.svc.UpdateCourseEnrollmentToLatest(ctx, studentCaller(), "enrollment-1")

		require.NoError(t, err)
		state, err := f.state.GetByStudentID(ctx, "student-1")
		require.NoError(t, err)
		require.NotNil(t, state.CurrentStandalonePathID)
		assert.Equal(t, "other-path", *state.CurrentStandalonePathID)
	})

	t.Run("an enrollment with nothing to update is returned unchanged", func(t *testing.T) {
		f := newCourseUpdateFixture()
		before := f.enrollment(t)

		updated, err := f.svc.UpdateCourseEnrollmentToLatest(ctx, studentCaller(), "enrollment-1")

		require.NoError(t, err)
		assert.Equal(t, before, updated)
	})

	refusals := []struct {
		name   string
		caller domain.User
		id     string
		setup  func(t *testing.T, f courseUpdateFixture)
	}{
		{"another learner's enrollment", domain.User{ID: "student-2", Role: domain.RoleStudent}, "enrollment-1", func(*testing.T, courseUpdateFixture) {}},
		{"an unknown enrollment", studentCaller(), "missing", func(*testing.T, courseUpdateFixture) {}},
		{"an abandoned enrollment", studentCaller(), "enrollment-1", func(t *testing.T, f courseUpdateFixture) {
			require.NoError(t, f.enrollments.Abandon(ctx, "enrollment-1"))
		}},
	}
	for _, tc := range refusals {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			f := newCourseUpdateFixture()
			f.publish(2, "Fingerstyle Journey", nil, "path-1", "path-2")
			tc.setup(t, f)

			_, err := f.svc.UpdateCourseEnrollmentToLatest(ctx, tc.caller, tc.id)

			require.ErrorIs(t, err, domain.ErrNotFound)
		})
	}
}
