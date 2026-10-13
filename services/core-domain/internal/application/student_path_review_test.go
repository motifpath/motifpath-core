package application_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
)

// reviewFixture holds a student who finished a course of two parts and a
// standalone path, plus an archived path, a course still in progress and
// another student's course and path — every case the two held-path reads
// distinguish.
type reviewFixture struct {
	svc          *application.StudentPathService
	state        *fakeStudentLearningStateRepository
	enrollments  *fakeCourseEnrollmentRepository
	studentPaths *fakeStudentPathRepository
}

func threeItems() []domain.StudentPathItemRecord {
	return []domain.StudentPathItemRecord{
		{Position: 1, ContentNodeID: "node-01", ContentNodeVersionID: "version-node-01"},
		{Position: 2, ContentNodeID: "node-02", ContentNodeVersionID: "version-node-02"},
		{Position: 3, ContentNodeID: "node-03", ContentNodeVersionID: "version-node-03"},
	}
}

func newReviewFixture(t *testing.T) reviewFixture {
	t.Helper()
	ctx := context.Background()
	anyLocale := []domain.Language{{Code: domain.LanguageCodeAny}}
	nodes := newFakeContentNodeRepository()
	nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo, Languages: anyLocale})
	nodes.put(domain.ContentNode{ID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo, Languages: anyLocale})
	nodes.put(domain.ContentNode{ID: "node-03", Title: "Three", ContentType: domain.ContentTypeArticle, Languages: anyLocale})

	completion := newFakeCompletionStateReader()
	for _, node := range []string{"node-01", "node-02", "node-03"} {
		completion.set("alice", node, domain.CompletionStatusCompleted)
	}

	finished, active, bobs := "enrollment-finished", "enrollment-active", "enrollment-bob"
	updated, reordered := "enrollment-updated", "enrollment-reordered"
	one, two := 1, 2
	archivedAt := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	studentPaths := newFakeStudentPathRepository()
	for _, sp := range []domain.StudentPath{
		// Part 2 first, so ordering by position is not an accident of insertion.
		{ID: "finished-part-2", StudentID: "alice", SourceTemplateID: "strumming-path", Title: "Strumming", SourceCourseEnrollmentID: &finished, CourseCheckpointPosition: &two, Items: threeItems()},
		{ID: "finished-part-1", StudentID: "alice", SourceTemplateID: "open-chords-path", Title: "Open chords", SourceCourseEnrollmentID: &finished, CourseCheckpointPosition: &one, Items: threeItems()},
		{ID: "active-part-1", StudentID: "alice", SourceTemplateID: "open-chords-path", Title: "Open chords", SourceCourseEnrollmentID: &active, CourseCheckpointPosition: &one, Items: threeItems()},
		{ID: "standalone-finished", StudentID: "alice", SourceTemplateID: "open-chords-path", Title: "Open chords", Items: threeItems()},
		{ID: "standalone-archived", StudentID: "alice", SourceTemplateID: "rhythm-path", Title: "Rhythm", ArchivedAt: &archivedAt, Items: threeItems()},
		{ID: "blues", StudentID: "alice", SourceTemplateID: "blues-path", Title: "Blues", Items: threeItems()},
		{ID: "bob-part-1", StudentID: "bob", SourceTemplateID: "open-chords-path", Title: "Open chords", SourceCourseEnrollmentID: &bobs, CourseCheckpointPosition: &one, Items: threeItems()},
		{ID: "bob-standalone", StudentID: "bob", SourceTemplateID: "open-chords-path", Title: "Open chords", Items: threeItems()},
		// Updated from version 1 to version 2, which replaced strumming with
		// fingerpicking: the strumming copy was archived and fingerpicking
		// copied at the same position.
		{ID: "updated-part-1", StudentID: "alice", SourceTemplateID: "open-chords-path", Title: "Open chords", SourceCourseEnrollmentID: &updated, CourseCheckpointPosition: &one, Items: threeItems()},
		{ID: "updated-strumming", StudentID: "alice", SourceTemplateID: "strumming-path", Title: "Strumming", SourceCourseEnrollmentID: &updated, CourseCheckpointPosition: &two, ArchivedAt: &archivedAt, Items: threeItems()},
		{ID: "updated-fingerpicking", StudentID: "alice", SourceTemplateID: "fingerpicking-path", Title: "Fingerpicking", SourceCourseEnrollmentID: &updated, CourseCheckpointPosition: &two, Items: threeItems()},
		// Updated from version 1 to version 3, which put strumming first: the
		// active strumming copy moved to position 1, the open chords copy kept it.
		{ID: "reordered-open-chords", StudentID: "alice", SourceTemplateID: "open-chords-path", Title: "Open chords", SourceCourseEnrollmentID: &reordered, CourseCheckpointPosition: &one, Items: threeItems()},
		{ID: "reordered-strumming", StudentID: "alice", SourceTemplateID: "strumming-path", Title: "Strumming", SourceCourseEnrollmentID: &reordered, CourseCheckpointPosition: &one, Items: threeItems()},
	} {
		require.NoError(t, studentPaths.Create(ctx, sp))
	}

	activePart := "active-part-1"
	enrollments := newFakeCourseEnrollmentRepository()
	enrollments.put(domain.CourseEnrollment{ID: finished, StudentID: "alice", CourseID: "course-1", CourseVersionNumber: 1, Status: domain.CourseEnrollmentStatusCompleted})
	enrollments.put(domain.CourseEnrollment{ID: active, StudentID: "alice", CourseID: "course-1", CourseVersionNumber: 1, Status: domain.CourseEnrollmentStatusActive, ActiveCheckpointStudentPathID: &activePart, ActiveCheckpointPosition: &one})
	enrollments.put(domain.CourseEnrollment{ID: bobs, StudentID: "bob", CourseID: "course-1", CourseVersionNumber: 1, Status: domain.CourseEnrollmentStatusCompleted})
	enrollments.put(domain.CourseEnrollment{ID: updated, StudentID: "alice", CourseID: "course-1", CourseVersionNumber: 2, Status: domain.CourseEnrollmentStatusCompleted})
	enrollments.put(domain.CourseEnrollment{ID: reordered, StudentID: "alice", CourseID: "course-1", CourseVersionNumber: 3, Status: domain.CourseEnrollmentStatusCompleted})

	courseVersions := newFakeCourseVersionRepository()
	for number, paths := range map[int][]string{
		1: {"open-chords-path", "strumming-path"},
		2: {"open-chords-path", "fingerpicking-path"},
		3: {"strumming-path", "open-chords-path"},
	} {
		checkpoints := make([]domain.CourseVersionCheckpoint, len(paths))
		for i, path := range paths {
			checkpoints[i] = domain.CourseVersionCheckpoint{Position: i + 1, LearningPathID: path}
		}
		require.NoError(t, courseVersions.Create(ctx, domain.CourseVersion{ID: fmt.Sprintf("course-1-v%d", number), CourseID: "course-1", VersionNumber: number, Checkpoints: checkpoints}))
	}

	blues := "blues"
	state := newFakeStudentLearningStateRepository()
	require.NoError(t, state.Upsert(ctx, domain.StudentLearningState{StudentID: "alice", CurrentStandalonePathID: &blues}))

	svc := application.NewStudentPathService(newFakeUserRepository(), newFakeLearningPathRepository(), studentPaths,
		newFakeContentNodeVersionRepository(), state, enrollments, courseVersions, nodes, newFakeExerciseRepository(), completion,
		idSequence(), func() time.Time { return fixedAssignedAt })
	return reviewFixture{svc: svc, state: state, enrollments: enrollments, studentPaths: studentPaths}
}

// assertNothingMoved checks a read left alice's current path and her
// finished enrolment exactly as they were.
func (f reviewFixture) assertNothingMoved(t *testing.T) {
	t.Helper()
	state, err := f.state.GetByStudentID(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, state.CurrentStandalonePathID)
	assert.Equal(t, "blues", *state.CurrentStandalonePathID)
	assert.Nil(t, state.CurrentCourseEnrollmentID)

	enrollment, err := f.enrollments.GetByID(context.Background(), "enrollment-finished")
	require.NoError(t, err)
	assert.Equal(t, domain.CourseEnrollmentStatusCompleted, enrollment.Status)
}

func TestStudentPathService_GetMyStudentPath(t *testing.T) {
	alice := domain.User{ID: "alice", Role: domain.RoleStudent}
	two := 2
	finished := "enrollment-finished"

	tests := []struct {
		name              string
		studentPathID     string
		wantErr           error
		wantTitle         string
		wantEnrollmentID  *string
		wantCheckpointPos *int
	}{
		{name: "a finished standalone path is read with every item completed", studentPathID: "standalone-finished", wantTitle: "Open chords"},
		{name: "an archived standalone path can still be read", studentPathID: "standalone-archived", wantTitle: "Rhythm"},
		{name: "a part of a finished course is read by its own id, with its course and position", studentPathID: "finished-part-2", wantTitle: "Strumming", wantEnrollmentID: &finished, wantCheckpointPos: &two},
		{name: "another student's path is not found", studentPathID: "bob-standalone", wantErr: domain.ErrNotFound},
		{name: "an unknown path is not found", studentPathID: "no-such-path", wantErr: domain.ErrNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newReviewFixture(t)

			view, err := f.svc.GetMyStudentPath(context.Background(), alice, tc.studentPathID)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.studentPathID, view.StudentPathID)
			assert.Equal(t, tc.wantTitle, view.Title)
			assert.Equal(t, tc.wantEnrollmentID, view.CourseEnrollmentID)
			assert.Equal(t, tc.wantCheckpointPos, view.CourseCheckpointPosition)
			require.Len(t, view.Items, 3)
			for _, item := range view.Items {
				assert.Equal(t, domain.CompletionStatusCompleted, item.Status)
			}
			assert.False(t, view.CourseCompleted)
			f.assertNothingMoved(t)
		})
	}
}

func TestStudentPathService_ListMyCourseEnrollmentParts(t *testing.T) {
	alice := domain.User{ID: "alice", Role: domain.RoleStudent}

	tests := []struct {
		name         string
		enrollmentID string
		wantErr      error
		wantPartIDs  []string
	}{
		{name: "a finished course lists every part in course order", enrollmentID: "enrollment-finished", wantPartIDs: []string{"finished-part-1", "finished-part-2"}},
		{name: "an enrolment still in progress lists only the parts reached", enrollmentID: "enrollment-active", wantPartIDs: []string{"active-part-1"}},
		{name: "after an update, only the parts of the version updated to are listed", enrollmentID: "enrollment-updated", wantPartIDs: []string{"updated-part-1", "updated-fingerpicking"}},
		{name: "after an update that reorders the parts, they follow the new order", enrollmentID: "enrollment-reordered", wantPartIDs: []string{"reordered-strumming", "reordered-open-chords"}},
		{name: "another student's enrolment is not found", enrollmentID: "enrollment-bob", wantErr: domain.ErrNotFound},
		{name: "an unknown enrolment is not found", enrollmentID: "no-such-enrollment", wantErr: domain.ErrNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newReviewFixture(t)

			parts, err := f.svc.ListMyCourseEnrollmentParts(context.Background(), alice, tc.enrollmentID)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			gotIDs := make([]string, len(parts))
			for i, part := range parts {
				gotIDs[i] = part.StudentPathID
				require.NotNil(t, part.CourseEnrollmentID)
				assert.Equal(t, tc.enrollmentID, *part.CourseEnrollmentID)
				require.NotNil(t, part.CourseCheckpointPosition)
				assert.Equal(t, i+1, *part.CourseCheckpointPosition)
				assert.False(t, part.CourseCompleted)
				for _, item := range part.Items {
					assert.Equal(t, domain.CompletionStatusCompleted, item.Status)
				}
			}
			assert.Equal(t, tc.wantPartIDs, gotIDs)
			f.assertNothingMoved(t)
		})
	}
}
