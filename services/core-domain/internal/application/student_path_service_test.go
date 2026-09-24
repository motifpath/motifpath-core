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

var fixedAssignedAt = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

func newStudentPathService(
	users *fakeUserRepository,
	paths *fakeLearningPathRepository,
	studentPaths *fakeStudentPathRepository,
	versions *fakeContentNodeVersionRepository,
	state *fakeStudentLearningStateRepository,
	completion *fakeCompletionStateReader,
	enrollments ...*fakeCourseEnrollmentRepository,
) *application.StudentPathService {
	return newStudentPathServiceWithContent(users, paths, studentPaths, versions, state, newFakeContentNodeRepository(), newFakeExerciseRepository(), completion, enrollments...)
}

func newStudentPathServiceWithContent(
	users *fakeUserRepository,
	paths *fakeLearningPathRepository,
	studentPaths *fakeStudentPathRepository,
	versions *fakeContentNodeVersionRepository,
	state *fakeStudentLearningStateRepository,
	contentNodes *fakeContentNodeRepository,
	exercises *fakeExerciseRepository,
	completion *fakeCompletionStateReader,
	enrollments ...*fakeCourseEnrollmentRepository,
) *application.StudentPathService {
	e := newFakeCourseEnrollmentRepository()
	if len(enrollments) > 0 {
		e = enrollments[0]
	}
	return application.NewStudentPathService(users, paths, studentPaths, versions, state, e, newFakeCourseVersionRepository(), contentNodes, exercises, completion, idSequence(), func() time.Time { return fixedAssignedAt })
}

// publishedVersions returns a fakeContentNodeVersionRepository pre-seeded
// with a version 1 for each given content node id — the "every referenced
// content node has already been published" precondition AssignLearningPath
// requires.
func publishedVersions(nodeIDs ...string) *fakeContentNodeVersionRepository {
	versions := newFakeContentNodeVersionRepository()
	for _, id := range nodeIDs {
		_ = versions.Create(context.Background(), domain.ContentNodeVersion{ID: "version-" + id, ContentNodeID: id, VersionNumber: 1})
	}
	return versions
}

func threeItemTemplate() domain.LearningPath {
	return domain.LearningPath{
		ID:    "path-1",
		Title: "Beginner Guitar",
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo},
			{Position: 2, ContentNodeID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo},
			{Position: 3, ContentNodeID: "node-03", Title: "Three", ContentType: domain.ContentTypeArticle},
		},
	}
}

func TestStudentPathService_AssignLearningPath(t *testing.T) {
	t.Run("a teacher assigns a learning path to a student, copying its items", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		paths := newFakeLearningPathRepository()
		paths.put(threeItemTemplate())
		svc := newStudentPathService(users, paths, newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		sp, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-1")

		require.NoError(t, err)
		assert.Equal(t, "student-1", sp.StudentID)
		assert.Equal(t, "teacher-1", sp.AssignedBy)
		assert.Equal(t, "path-1", sp.SourceTemplateID)
		require.Len(t, sp.Items, 3)
		assert.Equal(t, "version-node-01", sp.Items[0].ContentNodeVersionID)
	})

	t.Run("assigning sets the student's current path unconditionally", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		paths := newFakeLearningPathRepository()
		paths.put(threeItemTemplate())
		state := newFakeStudentLearningStateRepository()
		svc := newStudentPathService(users, paths, newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), state, newFakeCompletionStateReader())

		sp, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-1")
		require.NoError(t, err)

		got, err := state.GetByStudentID(context.Background(), "student-1")
		require.NoError(t, err)
		require.NotNil(t, got.CurrentStandalonePathID)
		assert.Equal(t, sp.ID, *got.CurrentStandalonePathID)
	})

	t.Run("an admin assigns a learning path to a student", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		paths := newFakeLearningPathRepository()
		paths.put(threeItemTemplate())
		svc := newStudentPathService(users, paths, newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), adminCaller(), "student-1", "path-1")

		require.NoError(t, err)
	})

	t.Run("assigning a new path to a student who already has a current path is additive", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		paths := newFakeLearningPathRepository()
		paths.put(threeItemTemplate())
		paths.put(domain.LearningPath{ID: "path-2", Title: "Fingerstyle", Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: "node-04"}}})
		studentPaths := newFakeStudentPathRepository()
		state := newFakeStudentLearningStateRepository()
		versions := publishedVersions("node-01", "node-02", "node-03", "node-04")
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader())

		first, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-1")
		require.NoError(t, err)

		second, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-2")
		require.NoError(t, err)

		assert.NotEqual(t, first.ID, second.ID)

		// The earlier copy still exists and is not archived.
		stillThere, err := studentPaths.GetByID(context.Background(), first.ID)
		require.NoError(t, err)
		assert.Nil(t, stillThere.ArchivedAt)

		// The current pointer now points at the new copy.
		got, err := state.GetByStudentID(context.Background(), "student-1")
		require.NoError(t, err)
		assert.Equal(t, second.ID, *got.CurrentStandalonePathID)
	})

	t.Run("assigning a path whose content node has never been published is refused", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		paths := newFakeLearningPathRepository()
		paths.put(threeItemTemplate())
		svc := newStudentPathService(users, paths, newFakeStudentPathRepository(), newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("assigning a path to a non-existent student returns not found", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(threeItemTemplate())
		svc := newStudentPathService(newFakeUserRepository(), paths, newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "missing", "path-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("assigning a non-existent path to a student returns not found", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		svc := newStudentPathService(users, newFakeLearningPathRepository(), newFakeStudentPathRepository(), newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("assigning a path to a user with role teacher returns not found", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "carol-1", Role: domain.RoleTeacher})
		paths := newFakeLearningPathRepository()
		paths.put(threeItemTemplate())
		svc := newStudentPathService(users, paths, newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "carol-1", "path-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("assigning a path to a user with role admin succeeds — admins may dogfood as students", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "admin-1", Role: domain.RoleAdmin})
		paths := newFakeLearningPathRepository()
		paths.put(threeItemTemplate())
		svc := newStudentPathService(users, paths, newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		sp, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "admin-1", "path-1")

		require.NoError(t, err)
		assert.Equal(t, "admin-1", sp.StudentID)
	})

	t.Run("a student cannot assign a learning path", func(t *testing.T) {
		svc := newStudentPathService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), studentCaller(), "student-1", "path-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestStudentPathService_GetMyPath(t *testing.T) {
	setup := func() (*fakeUserRepository, *fakeLearningPathRepository, *fakeStudentPathRepository, *fakeContentNodeVersionRepository, *fakeStudentLearningStateRepository) {
		return newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository()
	}

	assignAndGetView := func(t *testing.T, completion *fakeCompletionStateReader) application.StudentPathView {
		t.Helper()
		users, paths, studentPaths, versions, state := setup()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		anyLocale := []domain.Language{{Code: domain.LanguageCodeAny}}
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo, Languages: anyLocale})
		nodes.put(domain.ContentNode{ID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo, Languages: anyLocale})
		nodes.put(domain.ContentNode{ID: "node-03", Title: "Three", ContentType: domain.ContentTypeArticle, Languages: anyLocale})
		svc := newStudentPathServiceWithContent(users, paths, studentPaths, versions, state, nodes, newFakeExerciseRepository(), completion)

		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-1")
		require.NoError(t, err)

		view, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent})
		require.NoError(t, err)
		return view
	}

	t.Run("a freshly assigned path shows all items as not_started except the first", func(t *testing.T) {
		view := assignAndGetView(t, newFakeCompletionStateReader())

		require.Len(t, view.Items, 3)
		assert.Equal(t, domain.CompletionStatusNotStarted, view.Items[0].Status)
		assert.Equal(t, domain.CompletionStatusLocked, view.Items[1].Status)
		assert.Equal(t, domain.CompletionStatusLocked, view.Items[2].Status)
		assert.Equal(t, 1, view.CurrentPosition)
		assert.NotEmpty(t, view.Items[0].ContentNodeVersionID)
	})

	t.Run("a student who has completed the first node sees it completed and the second not_started", func(t *testing.T) {
		completion := newFakeCompletionStateReader()
		completion.set("alice", "node-01", domain.CompletionStatusCompleted)

		view := assignAndGetView(t, completion)

		assert.Equal(t, domain.CompletionStatusCompleted, view.Items[0].Status)
		assert.Equal(t, domain.CompletionStatusNotStarted, view.Items[1].Status)
		assert.Equal(t, 2, view.CurrentPosition)
	})

	t.Run("the path view includes each item's title and content_type", func(t *testing.T) {
		view := assignAndGetView(t, newFakeCompletionStateReader())

		assert.Equal(t, "One", view.Items[0].Title)
		assert.Equal(t, domain.ContentTypeVideo, view.Items[0].ContentType)
	})

	t.Run("a student with no current path set gets not found", func(t *testing.T) {
		svc := newStudentPathService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a teacher with no current path set gets not found, not forbidden", func(t *testing.T) {
		svc := newStudentPathService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.GetMyPath(context.Background(), teacherCaller())

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("an admin with no current path set gets not found, not forbidden", func(t *testing.T) {
		svc := newStudentPathService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.GetMyPath(context.Background(), adminCaller())

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a node whose content has no matching-language tag for the student's locale is locked", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths := newFakeLearningPathRepository()
		paths.put(threeItemTemplate())
		studentPaths := newFakeStudentPathRepository()
		versions := publishedVersions("node-01", "node-02", "node-03")
		state := newFakeStudentLearningStateRepository()
		nodes := newFakeContentNodeRepository()
		nodes.put(domain.ContentNode{ID: "node-01", Languages: []domain.Language{{Code: "pt_BR"}}})
		svc := newStudentPathServiceWithContent(users, paths, studentPaths, versions, state, nodes, newFakeExerciseRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-1")
		require.NoError(t, err)

		view, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent, Locale: domain.Language{Code: "en"}})

		require.NoError(t, err)
		assert.Equal(t, domain.CompletionStatusLocked, view.Items[0].Status)
	})

	t.Run("a student whose current pointer is a course enrollment sees that checkpoint's path, with course fields set", func(t *testing.T) {
		users, paths, studentPaths, versions, state := setup()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		enrollments := newFakeCourseEnrollmentRepository()
		checkpointPathID := "checkpoint-path-1"
		checkpointPosition := 1
		enrollments.put(domain.CourseEnrollment{
			ID:                            "enrollment-1",
			StudentID:                     "alice",
			CourseID:                      "course-1",
			Status:                        domain.CourseEnrollmentStatusActive,
			ActiveCheckpointStudentPathID: &checkpointPathID,
			ActiveCheckpointPosition:      &checkpointPosition,
		})
		sp, err := domain.NewStudentPathFromTemplate(checkpointPathID, "alice", threeItemTemplate(), "alice", fixedAssignedAt, map[string]string{
			"node-01": "version-node-01", "node-02": "version-node-02", "node-03": "version-node-03",
		})
		require.NoError(t, err)
		require.NoError(t, studentPaths.Create(context.Background(), sp))
		require.NoError(t, state.Upsert(context.Background(), domain.StudentLearningState{StudentID: "alice"}.WithCurrentCourseEnrollment("enrollment-1")))
		svc := newStudentPathServiceWithContent(users, paths, studentPaths, versions, state, newFakeContentNodeRepository(), newFakeExerciseRepository(), newFakeCompletionStateReader(), enrollments)

		view, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent})

		require.NoError(t, err)
		require.NotNil(t, view.CourseEnrollmentID)
		assert.Equal(t, "enrollment-1", *view.CourseEnrollmentID)
		require.NotNil(t, view.CourseCheckpointPosition)
		assert.Equal(t, 1, *view.CourseCheckpointPosition)
	})
}

func TestStudentPathService_ArchiveStandaloneStudentPath(t *testing.T) {
	t.Run("archiving the student's only current path with nothing else eligible succeeds and clears the current pointer", func(t *testing.T) {
		users, paths, studentPaths, versions, state := newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader())
		sp, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-1")
		require.NoError(t, err)

		archived, err := svc.ArchiveStandaloneStudentPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent}, sp.ID)

		require.NoError(t, err)
		require.NotNil(t, archived.ArchivedAt)
		got, err := state.GetByStudentID(context.Background(), "alice")
		require.NoError(t, err)
		assert.False(t, got.HasCurrent())
	})

	t.Run("archiving the current path while another eligible standalone path exists is a conflict — the student must switch first", func(t *testing.T) {
		users, paths, studentPaths, versions, state := newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03", "node-04"), newFakeStudentLearningStateRepository()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		paths.put(domain.LearningPath{ID: "path-2", Title: "Fingerstyle", Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: "node-04"}}})
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader())
		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-1")
		require.NoError(t, err)
		second, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-2")
		require.NoError(t, err)

		// second is now current (AssignLearningPath sets current unconditionally).
		_, err = svc.ArchiveStandaloneStudentPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent}, second.ID)

		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("archiving one of several non-current paths succeeds without touching the current pointer", func(t *testing.T) {
		users, paths, studentPaths, versions, state := newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03", "node-04"), newFakeStudentLearningStateRepository()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		paths.put(domain.LearningPath{ID: "path-2", Title: "Fingerstyle", Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: "node-04"}}})
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader())
		first, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-1")
		require.NoError(t, err)
		_, err = svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-2")
		require.NoError(t, err)

		archived, err := svc.ArchiveStandaloneStudentPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent}, first.ID)

		require.NoError(t, err)
		require.NotNil(t, archived.ArchivedAt)
	})

	t.Run("archiving the current path while an active course enrollment exists is a conflict — the student must switch first", func(t *testing.T) {
		users, paths, studentPaths, versions, state := newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		enrollments := newFakeCourseEnrollmentRepository()
		enrollments.put(domain.CourseEnrollment{ID: "enrollment-1", StudentID: "alice", CourseID: "course-1", Status: domain.CourseEnrollmentStatusActive})
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader(), enrollments)
		sp, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-1")
		require.NoError(t, err)

		_, err = svc.ArchiveStandaloneStudentPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent}, sp.ID)

		assert.ErrorIs(t, err, domain.ErrConflict)
	})

	t.Run("archiving a path owned by another student returns not found", func(t *testing.T) {
		users, paths, studentPaths, versions, state := newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader())
		sp, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-1")
		require.NoError(t, err)

		_, err = svc.ArchiveStandaloneStudentPath(context.Background(), domain.User{ID: "bob", Role: domain.RoleStudent}, sp.ID)

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestStudentPathService_SetCurrentPath(t *testing.T) {
	t.Run("switches current to a standalone path the student already owns", func(t *testing.T) {
		users, paths, studentPaths, versions, state := newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03", "node-04"), newFakeStudentLearningStateRepository()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		paths.put(domain.LearningPath{ID: "path-2", Title: "Fingerstyle", Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: "node-04"}}})
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader())
		first, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-1")
		require.NoError(t, err)
		_, err = svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-2")
		require.NoError(t, err)

		view, err := svc.SetCurrentPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent}, application.SetCurrentPathInput{StudentPathID: &first.ID})

		require.NoError(t, err)
		assert.Equal(t, first.ID, view.StudentPathID)
		got, err := state.GetByStudentID(context.Background(), "alice")
		require.NoError(t, err)
		require.NotNil(t, got.CurrentStandalonePathID)
		assert.Equal(t, first.ID, *got.CurrentStandalonePathID)
	})

	t.Run("switches current to an active course enrollment the student already owns", func(t *testing.T) {
		users, paths, studentPaths, versions, state := newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		enrollments := newFakeCourseEnrollmentRepository()
		checkpointPathID := "checkpoint-path-1"
		checkpointPosition := 1
		enrollments.put(domain.CourseEnrollment{
			ID: "enrollment-1", StudentID: "alice", CourseID: "course-1", Status: domain.CourseEnrollmentStatusActive,
			ActiveCheckpointStudentPathID: &checkpointPathID, ActiveCheckpointPosition: &checkpointPosition,
		})
		sp, err := domain.NewStudentPathFromTemplate(checkpointPathID, "alice", threeItemTemplate(), "alice", fixedAssignedAt, map[string]string{
			"node-01": "version-node-01", "node-02": "version-node-02", "node-03": "version-node-03",
		})
		require.NoError(t, err)
		require.NoError(t, studentPaths.Create(context.Background(), sp))
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader(), enrollments)
		enrollmentID := "enrollment-1"

		view, err := svc.SetCurrentPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent}, application.SetCurrentPathInput{CourseEnrollmentID: &enrollmentID})

		require.NoError(t, err)
		require.NotNil(t, view.CourseEnrollmentID)
		assert.Equal(t, "enrollment-1", *view.CourseEnrollmentID)
		got, err := state.GetByStudentID(context.Background(), "alice")
		require.NoError(t, err)
		require.NotNil(t, got.CurrentCourseEnrollmentID)
		assert.Equal(t, "enrollment-1", *got.CurrentCourseEnrollmentID)
	})

	t.Run("an admin acting as their own student identity switches current to a standalone path", func(t *testing.T) {
		users, paths, studentPaths, versions, state := newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository()
		users.put(domain.User{ID: "admin-1", Role: domain.RoleAdmin})
		paths.put(threeItemTemplate())
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader())
		sp, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "admin-1", "path-1")
		require.NoError(t, err)

		view, err := svc.SetCurrentPath(context.Background(), domain.User{ID: "admin-1", Role: domain.RoleAdmin}, application.SetCurrentPathInput{StudentPathID: &sp.ID})

		require.NoError(t, err)
		assert.Equal(t, sp.ID, view.StudentPathID)
	})

	t.Run("rejects a request with neither id given", func(t *testing.T) {
		svc := newStudentPathService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())

		_, err := svc.SetCurrentPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent}, application.SetCurrentPathInput{})

		var valErr *domain.ValidationError
		assert.ErrorAs(t, err, &valErr)
	})

	t.Run("rejects a request with both ids given", func(t *testing.T) {
		svc := newStudentPathService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())
		spID, enrollmentID := "sp-1", "enrollment-1"

		_, err := svc.SetCurrentPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent}, application.SetCurrentPathInput{StudentPathID: &spID, CourseEnrollmentID: &enrollmentID})

		var valErr *domain.ValidationError
		assert.ErrorAs(t, err, &valErr)
	})

	t.Run("a standalone path not owned by the caller is not found", func(t *testing.T) {
		users, paths, studentPaths, versions, state := newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), publishedVersions("node-01", "node-02", "node-03"), newFakeStudentLearningStateRepository()
		users.put(domain.User{ID: "alice", Role: domain.RoleStudent})
		paths.put(threeItemTemplate())
		svc := newStudentPathService(users, paths, studentPaths, versions, state, newFakeCompletionStateReader())
		sp, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "alice", "path-1")
		require.NoError(t, err)

		_, err = svc.SetCurrentPath(context.Background(), domain.User{ID: "bob", Role: domain.RoleStudent}, application.SetCurrentPathInput{StudentPathID: &sp.ID})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("only students may switch their current path", func(t *testing.T) {
		svc := newStudentPathService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakeStudentPathRepository(), newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())
		spID := "sp-1"

		_, err := svc.SetCurrentPath(context.Background(), teacherCaller(), application.SetCurrentPathInput{StudentPathID: &spID})

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func TestStudentPathService_ListMyStandalonePaths(t *testing.T) {
	newService := func(studentPaths *fakeStudentPathRepository) *application.StudentPathService {
		return newStudentPathService(newFakeUserRepository(), newFakeLearningPathRepository(), studentPaths,
			newFakeContentNodeVersionRepository(), newFakeStudentLearningStateRepository(), newFakeCompletionStateReader())
	}
	alice := domain.User{ID: "alice", Role: domain.RoleStudent}
	archivedAt := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	enrollmentID := "enrollment-1"

	t.Run("lists the student's standalone paths, active and archived, newest first", func(t *testing.T) {
		studentPaths := newFakeStudentPathRepository()
		require.NoError(t, studentPaths.Create(context.Background(), domain.StudentPath{ID: "older", StudentID: "alice", AssignedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), ArchivedAt: &archivedAt}))
		require.NoError(t, studentPaths.Create(context.Background(), domain.StudentPath{ID: "newer", StudentID: "alice", AssignedAt: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)}))

		got, err := newService(studentPaths).ListMyStandalonePaths(context.Background(), alice)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "newer", got[0].ID)
		assert.Equal(t, "older", got[1].ID)
	})

	t.Run("excludes course checkpoints and other students' paths", func(t *testing.T) {
		studentPaths := newFakeStudentPathRepository()
		require.NoError(t, studentPaths.Create(context.Background(), domain.StudentPath{ID: "mine", StudentID: "alice"}))
		require.NoError(t, studentPaths.Create(context.Background(), domain.StudentPath{ID: "checkpoint", StudentID: "alice", SourceCourseEnrollmentID: &enrollmentID}))
		require.NoError(t, studentPaths.Create(context.Background(), domain.StudentPath{ID: "theirs", StudentID: "bruno"}))

		got, err := newService(studentPaths).ListMyStandalonePaths(context.Background(), alice)

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "mine", got[0].ID)
	})

	t.Run("a student with none gets an empty, non-nil list", func(t *testing.T) {
		got, err := newService(newFakeStudentPathRepository()).ListMyStandalonePaths(context.Background(), alice)

		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("an admin may list their own", func(t *testing.T) {
		_, err := newService(newFakeStudentPathRepository()).ListMyStandalonePaths(context.Background(), adminCaller())

		require.NoError(t, err)
	})

	t.Run("a teacher is forbidden", func(t *testing.T) {
		_, err := newService(newFakeStudentPathRepository()).ListMyStandalonePaths(context.Background(), teacherCaller())

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}
