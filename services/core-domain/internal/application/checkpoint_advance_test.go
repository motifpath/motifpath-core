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

// twoCheckpointCourseVersion seeds a published CourseVersion for courseID
// with two checkpoints, at positions 1 and 2, pointing at path1ID/path2ID —
// everything checkAndAdvanceCheckpoint needs to find "the next checkpoint"
// or discover "that was the last one". No Course record is needed alongside
// it: checkAndAdvanceCheckpoint only ever reads the CourseVersion.
func twoCheckpointCourseVersion(versions *fakeCourseVersionRepository, courseID, path1ID, path2ID string) {
	_ = versions.Create(context.Background(), domain.CourseVersion{
		ID: "version-" + courseID, CourseID: courseID, VersionNumber: 1,
		TitleSnapshot: "Fingerstyle Journey",
		Checkpoints: []domain.CourseVersionCheckpoint{
			{Position: 1, LearningPathID: path1ID, EffectiveTitle: "Checkpoint 1"},
			{Position: 2, LearningPathID: path2ID, EffectiveTitle: "Checkpoint 2"},
		},
		PublishedAt:                fixedEnrolledAt,
		AvailableForNewEnrollments: true,
	})
}

// singleItemTemplate returns a one-item LearningPath template referencing
// contentNodeID — trivial to mark "every item complete" in a test.
func singleItemTemplate(id, contentNodeID string) domain.LearningPath {
	return domain.LearningPath{ID: id, Title: "Checkpoint", Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: contentNodeID}}}
}

// checkpointAdvanceFixtures bundles every repository checkAndAdvanceCheckpoint
// touches, shared explicitly across a StudentPathService and (when needed) a
// CourseEnrollmentService built on top of them — the
// newStudentPathService/-WithContent helpers default courseVersions to an
// empty fake, which has nothing for a next-checkpoint lookup to find.
type checkpointAdvanceFixtures struct {
	users          *fakeUserRepository
	paths          *fakeLearningPathRepository
	studentPaths   *fakeStudentPathRepository
	versions       *fakeContentNodeVersionRepository
	state          *fakeStudentLearningStateRepository
	enrollments    *fakeCourseEnrollmentRepository
	courseVersions *fakeCourseVersionRepository
	completion     *fakeCompletionStateReader
}

func newCheckpointAdvanceFixtures() checkpointAdvanceFixtures {
	return checkpointAdvanceFixtures{
		users:          newFakeUserRepository(),
		paths:          newFakeLearningPathRepository(),
		studentPaths:   newFakeStudentPathRepository(),
		versions:       publishedVersions("node-01", "node-02"),
		state:          newFakeStudentLearningStateRepository(),
		enrollments:    newFakeCourseEnrollmentRepository(),
		courseVersions: newFakeCourseVersionRepository(),
		completion:     newFakeCompletionStateReader(),
	}
}

func (f checkpointAdvanceFixtures) studentPathService() *application.StudentPathService {
	return application.NewStudentPathService(
		f.users, f.paths, f.studentPaths, f.versions, f.state, f.enrollments, f.courseVersions,
		newFakeContentNodeRepository(), newFakeExerciseRepository(), f.completion,
		idSequence(), func() time.Time { return fixedEnrolledAt },
	)
}

func (f checkpointAdvanceFixtures) courseEnrollmentService(studentPathSvc *application.StudentPathService) *application.CourseEnrollmentService {
	courses := newFakeCourseRepository()
	courses.put(domain.Course{ID: "course-1", Title: "Fingerstyle Journey", Status: domain.CourseStatusPublished})
	return application.NewCourseEnrollmentService(
		courses, f.courseVersions, f.paths, f.studentPaths, f.enrollments, studentPathSvc, f.state, f.completion,
		idSequence(), func() time.Time { return fixedEnrolledAt },
	)
}

// seedActiveCheckpoint1 puts alice into an active CourseEnrollment for
// course-1, with checkpoint 1 (a single-item path over node-01) as her
// current path.
func seedActiveCheckpoint1(f checkpointAdvanceFixtures) {
	f.paths.put(singleItemTemplate("path-1", "node-01"))
	f.paths.put(singleItemTemplate("path-2", "node-02"))
	twoCheckpointCourseVersion(f.courseVersions, "course-1", "path-1", "path-2")

	sp, err := domain.NewStudentPathFromTemplate("checkpoint-path-1", "student-1", singleItemTemplate("path-1", "node-01"), "student-1", fixedEnrolledAt, map[string]string{
		"node-01": "version-node-01",
	})
	if err != nil {
		panic(err)
	}
	sourceCourseEnrollmentID := "enrollment-1"
	checkpointPosition := 1
	sp.SourceCourseEnrollmentID = &sourceCourseEnrollmentID
	sp.CourseCheckpointPosition = &checkpointPosition
	if err := f.studentPaths.Create(context.Background(), sp); err != nil {
		panic(err)
	}

	checkpointPathID := "checkpoint-path-1"
	f.enrollments.put(domain.CourseEnrollment{
		ID: "enrollment-1", StudentID: "student-1", CourseID: "course-1", CourseTitle: "Fingerstyle Journey", CourseVersionNumber: 1,
		Status: domain.CourseEnrollmentStatusActive, ActiveCheckpointStudentPathID: &checkpointPathID, ActiveCheckpointPosition: &checkpointPosition,
		EnrolledAt: fixedEnrolledAt,
	})
	_ = f.state.Upsert(context.Background(), domain.StudentLearningState{StudentID: "student-1"}.WithCurrentCourseEnrollment("enrollment-1"))
}

func TestStudentPathService_ChecksCheckpointCompletionOnRead(t *testing.T) {
	t.Run("completing every item of the active checkpoint advances GetMyPath to the next checkpoint", func(t *testing.T) {
		f := newCheckpointAdvanceFixtures()
		seedActiveCheckpoint1(f)
		f.completion.set("student-1", "node-01", domain.CompletionStatusCompleted)
		svc := f.studentPathService()

		view, err := svc.GetMyPath(context.Background(), studentCaller())

		require.NoError(t, err)
		require.NotNil(t, view.CourseEnrollmentID)
		assert.Equal(t, "enrollment-1", *view.CourseEnrollmentID)
		require.NotNil(t, view.CourseCheckpointPosition)
		assert.Equal(t, 2, *view.CourseCheckpointPosition)
		require.Len(t, view.Items, 1)
		assert.Equal(t, "node-02", view.Items[0].ContentNodeID)
		assert.Equal(t, domain.CompletionStatusNotStarted, view.Items[0].Status)

		enrollment, err := f.enrollments.GetByID(context.Background(), "enrollment-1")
		require.NoError(t, err)
		require.NotNil(t, enrollment.ActiveCheckpointPosition)
		assert.Equal(t, 2, *enrollment.ActiveCheckpointPosition)
		require.NotNil(t, enrollment.ActiveCheckpointStudentPathID)
		assert.NotEqual(t, "checkpoint-path-1", *enrollment.ActiveCheckpointStudentPathID)
		assert.Equal(t, domain.CourseEnrollmentStatusActive, enrollment.Status)
	})

	t.Run("an incomplete checkpoint leaves GetMyPath and the enrollment untouched", func(t *testing.T) {
		f := newCheckpointAdvanceFixtures()
		seedActiveCheckpoint1(f)
		svc := f.studentPathService()

		view, err := svc.GetMyPath(context.Background(), studentCaller())

		require.NoError(t, err)
		require.NotNil(t, view.CourseCheckpointPosition)
		assert.Equal(t, 1, *view.CourseCheckpointPosition)
	})

	t.Run("completing the last checkpoint's items completes the course, clears the current pointer, and GetMyPath now reports no current path", func(t *testing.T) {
		f := newCheckpointAdvanceFixtures()
		seedActiveCheckpoint1(f)
		f.completion.set("student-1", "node-01", domain.CompletionStatusCompleted)
		svc := f.studentPathService()
		_, err := svc.GetMyPath(context.Background(), studentCaller()) // advances to checkpoint 2
		require.NoError(t, err)
		f.completion.set("student-1", "node-02", domain.CompletionStatusCompleted)

		_, err = svc.GetMyPath(context.Background(), studentCaller())

		assert.ErrorIs(t, err, domain.ErrNotFound)
		enrollment, err := f.enrollments.GetByID(context.Background(), "enrollment-1")
		require.NoError(t, err)
		assert.Equal(t, domain.CourseEnrollmentStatusCompleted, enrollment.Status)
		assert.Nil(t, enrollment.ActiveCheckpointStudentPathID)
		assert.Nil(t, enrollment.ActiveCheckpointPosition)
		state, err := f.state.GetByStudentID(context.Background(), "student-1")
		require.NoError(t, err)
		assert.False(t, state.HasCurrent())
	})

	t.Run("SetCurrentPath discovers the same checkpoint advance when switching onto the course", func(t *testing.T) {
		f := newCheckpointAdvanceFixtures()
		seedActiveCheckpoint1(f)
		f.completion.set("student-1", "node-01", domain.CompletionStatusCompleted)
		// caller's current pointer starts elsewhere, so SetCurrentPath (not
		// GetMyPath) is what first reads this enrollment.
		otherPath, err := domain.NewStudentPathFromTemplate("other-path", "student-1", singleItemTemplate("path-other", "node-01"), "student-1", fixedEnrolledAt, map[string]string{"node-01": "version-node-01"})
		require.NoError(t, err)
		require.NoError(t, f.studentPaths.Create(context.Background(), otherPath))
		require.NoError(t, f.state.Upsert(context.Background(), domain.StudentLearningState{StudentID: "student-1"}.WithCurrentStandalonePath("other-path")))
		svc := f.studentPathService()
		enrollmentID := "enrollment-1"

		view, err := svc.SetCurrentPath(context.Background(), studentCaller(), application.SetCurrentPathInput{CourseEnrollmentID: &enrollmentID})

		require.NoError(t, err)
		require.NotNil(t, view.CourseCheckpointPosition)
		assert.Equal(t, 2, *view.CourseCheckpointPosition)
	})
}

func TestCourseEnrollmentService_ListMyCourseEnrollments_CheckpointAdvanceAndCompletion(t *testing.T) {
	t.Run("listing advances a checkpoint whose items are all complete", func(t *testing.T) {
		f := newCheckpointAdvanceFixtures()
		seedActiveCheckpoint1(f)
		f.completion.set("student-1", "node-01", domain.CompletionStatusCompleted)
		svc := f.courseEnrollmentService(f.studentPathService())

		list, err := svc.ListMyCourseEnrollments(context.Background(), studentCaller())

		require.NoError(t, err)
		require.Len(t, list, 1)
		require.NotNil(t, list[0].ActiveCheckpointPosition)
		assert.Equal(t, 2, *list[0].ActiveCheckpointPosition)
		assert.Equal(t, domain.CourseEnrollmentStatusActive, list[0].Status)
	})

	t.Run("listing completes the course when the last checkpoint's items are all complete, clearing the current pointer", func(t *testing.T) {
		f := newCheckpointAdvanceFixtures()
		seedActiveCheckpoint1(f)
		f.completion.set("student-1", "node-01", domain.CompletionStatusCompleted)
		f.completion.set("student-1", "node-02", domain.CompletionStatusCompleted)
		studentPathSvc := f.studentPathService()
		svc := f.courseEnrollmentService(studentPathSvc)
		// Advance to checkpoint 2 first (checkpoint 1 is already complete),
		// mirroring a student who already progressed there before finishing
		// checkpoint 2's own item.
		_, err := studentPathSvc.GetMyPath(context.Background(), studentCaller())
		require.NoError(t, err)

		list, err := svc.ListMyCourseEnrollments(context.Background(), studentCaller())

		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, domain.CourseEnrollmentStatusCompleted, list[0].Status)
		assert.Nil(t, list[0].ActiveCheckpointStudentPathID)
		assert.Nil(t, list[0].ActiveCheckpointPosition)
		state, err := f.state.GetByStudentID(context.Background(), "student-1")
		require.NoError(t, err)
		assert.False(t, state.HasCurrent())
	})

	t.Run("the congrats list offers another active enrollment to resume", func(t *testing.T) {
		f := newCheckpointAdvanceFixtures()
		seedActiveCheckpoint1(f)
		f.completion.set("student-1", "node-01", domain.CompletionStatusCompleted)
		f.completion.set("student-1", "node-02", domain.CompletionStatusCompleted)
		studentPathSvc := f.studentPathService()
		svc := f.courseEnrollmentService(studentPathSvc)
		_, err := studentPathSvc.GetMyPath(context.Background(), studentCaller()) // advance to checkpoint 2
		require.NoError(t, err)

		require.NoError(t, f.versions.Create(context.Background(), domain.ContentNodeVersion{ID: "version-node-03", ContentNodeID: "node-03", VersionNumber: 1}))
		otherCheckpointPathID := "rhythm-checkpoint-path"
		otherPosition := 1
		otherSP, err := domain.NewStudentPathFromTemplate(otherCheckpointPathID, "student-1", singleItemTemplate("path-rhythm", "node-03"), "student-1", fixedEnrolledAt, map[string]string{
			"node-03": "version-node-03",
		})
		require.NoError(t, err)
		require.NoError(t, f.studentPaths.Create(context.Background(), otherSP))
		f.enrollments.put(domain.CourseEnrollment{
			ID: "enrollment-2", StudentID: "student-1", CourseID: "course-2", CourseTitle: "Rhythm Mastery",
			Status: domain.CourseEnrollmentStatusActive, ActiveCheckpointStudentPathID: &otherCheckpointPathID, ActiveCheckpointPosition: &otherPosition,
			EnrolledAt: fixedEnrolledAt,
		})

		list, err := svc.ListMyCourseEnrollments(context.Background(), studentCaller())

		require.NoError(t, err)
		var sawCompleted, sawOtherActive bool
		for _, e := range list {
			switch e.ID {
			case "enrollment-1":
				sawCompleted = e.Status == domain.CourseEnrollmentStatusCompleted
			case "enrollment-2":
				sawOtherActive = e.Status == domain.CourseEnrollmentStatusActive
			}
		}
		assert.True(t, sawCompleted, "expected the just-completed enrollment to report status completed")
		assert.True(t, sawOtherActive, "expected the other course's active enrollment to remain listed as active")
	})

	t.Run("the congrats list offers only the catalog when no other enrollment is active", func(t *testing.T) {
		f := newCheckpointAdvanceFixtures()
		seedActiveCheckpoint1(f)
		f.completion.set("student-1", "node-01", domain.CompletionStatusCompleted)
		f.completion.set("student-1", "node-02", domain.CompletionStatusCompleted)
		studentPathSvc := f.studentPathService()
		svc := f.courseEnrollmentService(studentPathSvc)
		_, err := studentPathSvc.GetMyPath(context.Background(), studentCaller())
		require.NoError(t, err)

		list, err := svc.ListMyCourseEnrollments(context.Background(), studentCaller())

		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, domain.CourseEnrollmentStatusCompleted, list[0].Status)
	})
}

// TestFullTwoCheckpointCourseLifecycle is the end-to-end acceptance test for
// Step 8: a student self-enrolls in a two-checkpoint course, completes
// checkpoint 1's item (discovered on the next read), advances to checkpoint
// 2, completes it too, and the course completes with the current pointer
// cleared — all synchronously, as a side effect of ordinary reads, with no
// event-driven or background step anywhere in the path.
func TestFullTwoCheckpointCourseLifecycle(t *testing.T) {
	f := newCheckpointAdvanceFixtures()
	f.paths.put(singleItemTemplate("path-1", "node-01"))
	f.paths.put(singleItemTemplate("path-2", "node-02"))
	twoCheckpointCourseVersion(f.courseVersions, "course-1", "path-1", "path-2")
	studentPathSvc := f.studentPathService()
	svc := f.courseEnrollmentService(studentPathSvc)
	ctx := context.Background()

	// 1. Self-enroll: checkpoint 1's StudentPath is created and becomes current.
	enrollment, err := svc.CreateCourseEnrollment(ctx, studentCaller(), "course-1")
	require.NoError(t, err)
	require.NotNil(t, enrollment.ActiveCheckpointPosition)
	assert.Equal(t, 1, *enrollment.ActiveCheckpointPosition)

	view, err := studentPathSvc.GetMyPath(ctx, studentCaller())
	require.NoError(t, err)
	require.Len(t, view.Items, 1)
	assert.Equal(t, "node-01", view.Items[0].ContentNodeID)
	assert.Equal(t, domain.CompletionStatusNotStarted, view.Items[0].Status)

	// 2. Complete checkpoint 1's only item (simulating the external
	// completion-state store the Aggregation Worker owns), then read again:
	// GetMyPath discovers it synchronously and advances to checkpoint 2.
	f.completion.set("student-1", "node-01", domain.CompletionStatusCompleted)

	view, err = studentPathSvc.GetMyPath(ctx, studentCaller())
	require.NoError(t, err)
	require.NotNil(t, view.CourseCheckpointPosition)
	assert.Equal(t, 2, *view.CourseCheckpointPosition)
	require.Len(t, view.Items, 1)
	assert.Equal(t, "node-02", view.Items[0].ContentNodeID)

	got, err := f.enrollments.GetByID(ctx, enrollment.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ActiveCheckpointPosition)
	assert.Equal(t, 2, *got.ActiveCheckpointPosition)
	assert.Equal(t, domain.CourseEnrollmentStatusActive, got.Status)

	// 3. Complete checkpoint 2's only item — the last checkpoint. The next
	// read (ListMyCourseEnrollments, matching the congrats-page scenarios)
	// completes the course and clears the current pointer synchronously,
	// signaled in that same response via the enrollment's own status field.
	f.completion.set("student-1", "node-02", domain.CompletionStatusCompleted)

	list, err := svc.ListMyCourseEnrollments(ctx, studentCaller())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, domain.CourseEnrollmentStatusCompleted, list[0].Status)
	assert.Nil(t, list[0].ActiveCheckpointStudentPathID)
	assert.Nil(t, list[0].ActiveCheckpointPosition)

	state, err := f.state.GetByStudentID(ctx, "student-1")
	require.NoError(t, err)
	assert.False(t, state.HasCurrent(), "the completed course was the student's current pointer, so it must now be cleared")

	_, err = studentPathSvc.GetMyPath(ctx, studentCaller())
	assert.ErrorIs(t, err, domain.ErrNotFound, "no current path is set anymore once the course has completed")
}
