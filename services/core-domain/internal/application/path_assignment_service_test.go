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

func newPathAssignmentService(
	users *fakeUserRepository,
	paths *fakeLearningPathRepository,
	assignments *fakePathAssignmentRepository,
	completion *fakeCompletionStateReader,
) *application.PathAssignmentService {
	return application.NewPathAssignmentService(users, paths, assignments, completion, idSequence(), func() time.Time { return fixedAssignedAt })
}

func TestPathAssignmentService_AssignLearningPath(t *testing.T) {
	t.Run("a teacher assigns a learning path to a student", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1"})
		svc := newPathAssignmentService(users, paths, newFakePathAssignmentRepository(), newFakeCompletionStateReader())

		assignment, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-1")

		require.NoError(t, err)
		assert.Equal(t, "student-1", assignment.StudentID)
		assert.Equal(t, "teacher-1", assignment.AssignedBy)
		assert.Equal(t, "path-1", assignment.LearningPathID)
	})

	t.Run("an admin assigns a learning path to a student", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1"})
		svc := newPathAssignmentService(users, paths, newFakePathAssignmentRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), adminCaller(), "student-1", "path-1")

		require.NoError(t, err)
	})

	t.Run("assigning a new path to a student who already has an active assignment replaces it", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1"})
		paths.put(domain.LearningPath{ID: "path-2"})
		assignments := newFakePathAssignmentRepository()
		svc := newPathAssignmentService(users, paths, assignments, newFakeCompletionStateReader())

		first, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-1")
		require.NoError(t, err)

		second, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "path-2")
		require.NoError(t, err)

		assert.NotEqual(t, first.ID, second.ID)
		assert.Equal(t, "path-2", second.LearningPathID)

		active, err := assignments.GetActiveByStudentID(context.Background(), "student-1")
		require.NoError(t, err)
		assert.Equal(t, "path-2", active.LearningPathID)
	})

	t.Run("assigning a path to a non-existent student returns not found", func(t *testing.T) {
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1"})
		svc := newPathAssignmentService(newFakeUserRepository(), paths, newFakePathAssignmentRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "missing", "path-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("assigning a non-existent path to a student returns not found", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "student-1", Role: domain.RoleStudent})
		svc := newPathAssignmentService(users, newFakeLearningPathRepository(), newFakePathAssignmentRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "student-1", "missing")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("assigning a path to a user with role teacher returns not found", func(t *testing.T) {
		users := newFakeUserRepository()
		users.put(domain.User{ID: "carol-1", Role: domain.RoleTeacher})
		paths := newFakeLearningPathRepository()
		paths.put(domain.LearningPath{ID: "path-1"})
		svc := newPathAssignmentService(users, paths, newFakePathAssignmentRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), teacherCaller(), "carol-1", "path-1")

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a student cannot assign a learning path", func(t *testing.T) {
		svc := newPathAssignmentService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakePathAssignmentRepository(), newFakeCompletionStateReader())

		_, err := svc.AssignLearningPath(context.Background(), studentCaller(), "student-1", "path-1")

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}

func threeNodePath() domain.LearningPath {
	return domain.LearningPath{
		ID:    "path-1",
		Title: "week-1-path",
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: "node-01", Title: "One", ContentType: domain.ContentTypeVideo},
			{Position: 2, ContentNodeID: "node-02", Title: "Two", ContentType: domain.ContentTypeVideo},
			{Position: 3, ContentNodeID: "node-03", Title: "Three", ContentType: domain.ContentTypeArticle},
		},
	}
}

func TestPathAssignmentService_GetMyPath(t *testing.T) {
	t.Run("a fresh assignment shows all items as not_started except the first, which is unlocked", func(t *testing.T) {
		users := newFakeUserRepository()
		paths := newFakeLearningPathRepository()
		paths.put(threeNodePath())
		assignments := newFakePathAssignmentRepository()
		require.NoError(t, assignments.ReplaceActive(context.Background(), domain.PathAssignment{ID: "a-1", StudentID: "alice", LearningPathID: "path-1"}))
		svc := newPathAssignmentService(users, paths, assignments, newFakeCompletionStateReader())

		view, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent})

		require.NoError(t, err)
		require.Len(t, view.Items, 3)
		assert.Equal(t, domain.CompletionStatusNotStarted, view.Items[0].Status)
		assert.Equal(t, domain.CompletionStatusLocked, view.Items[1].Status)
		assert.Equal(t, domain.CompletionStatusLocked, view.Items[2].Status)
		assert.Equal(t, 1, view.CurrentPosition)
	})

	t.Run("a student who has completed the first node sees it completed and the second not_started", func(t *testing.T) {
		users := newFakeUserRepository()
		paths := newFakeLearningPathRepository()
		paths.put(threeNodePath())
		assignments := newFakePathAssignmentRepository()
		require.NoError(t, assignments.ReplaceActive(context.Background(), domain.PathAssignment{ID: "a-1", StudentID: "alice", LearningPathID: "path-1"}))
		completion := newFakeCompletionStateReader()
		completion.set("alice", "node-01", domain.CompletionStatusCompleted)
		svc := newPathAssignmentService(users, paths, assignments, completion)

		view, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent})

		require.NoError(t, err)
		assert.Equal(t, domain.CompletionStatusCompleted, view.Items[0].Status)
		assert.Equal(t, domain.CompletionStatusNotStarted, view.Items[1].Status)
		assert.Equal(t, domain.CompletionStatusLocked, view.Items[2].Status)
		assert.Equal(t, 2, view.CurrentPosition)
	})

	t.Run("a student who has started but not finished the second node sees it in_progress", func(t *testing.T) {
		users := newFakeUserRepository()
		paths := newFakeLearningPathRepository()
		paths.put(threeNodePath())
		assignments := newFakePathAssignmentRepository()
		require.NoError(t, assignments.ReplaceActive(context.Background(), domain.PathAssignment{ID: "a-1", StudentID: "alice", LearningPathID: "path-1"}))
		completion := newFakeCompletionStateReader()
		completion.set("alice", "node-01", domain.CompletionStatusCompleted)
		completion.set("alice", "node-02", domain.CompletionStatusInProgress)
		svc := newPathAssignmentService(users, paths, assignments, completion)

		view, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent})

		require.NoError(t, err)
		assert.Equal(t, domain.CompletionStatusCompleted, view.Items[0].Status)
		assert.Equal(t, domain.CompletionStatusInProgress, view.Items[1].Status)
		assert.Equal(t, domain.CompletionStatusLocked, view.Items[2].Status)
		assert.Equal(t, 2, view.CurrentPosition)
	})

	t.Run("a student who has completed all nodes sees the full path as completed", func(t *testing.T) {
		users := newFakeUserRepository()
		paths := newFakeLearningPathRepository()
		paths.put(threeNodePath())
		assignments := newFakePathAssignmentRepository()
		require.NoError(t, assignments.ReplaceActive(context.Background(), domain.PathAssignment{ID: "a-1", StudentID: "alice", LearningPathID: "path-1"}))
		completion := newFakeCompletionStateReader()
		completion.set("alice", "node-01", domain.CompletionStatusCompleted)
		completion.set("alice", "node-02", domain.CompletionStatusCompleted)
		completion.set("alice", "node-03", domain.CompletionStatusCompleted)
		svc := newPathAssignmentService(users, paths, assignments, completion)

		view, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent})

		require.NoError(t, err)
		for _, item := range view.Items {
			assert.Equal(t, domain.CompletionStatusCompleted, item.Status)
		}
		assert.Equal(t, 3, view.CurrentPosition)
	})

	t.Run("the path view includes each item's title and content_type", func(t *testing.T) {
		users := newFakeUserRepository()
		paths := newFakeLearningPathRepository()
		paths.put(threeNodePath())
		assignments := newFakePathAssignmentRepository()
		require.NoError(t, assignments.ReplaceActive(context.Background(), domain.PathAssignment{ID: "a-1", StudentID: "alice", LearningPathID: "path-1"}))
		svc := newPathAssignmentService(users, paths, assignments, newFakeCompletionStateReader())

		view, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent})

		require.NoError(t, err)
		assert.Equal(t, "One", view.Items[0].Title)
		assert.Equal(t, domain.ContentTypeVideo, view.Items[0].ContentType)
	})

	t.Run("a student with no active path assignment gets not found", func(t *testing.T) {
		svc := newPathAssignmentService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakePathAssignmentRepository(), newFakeCompletionStateReader())

		_, err := svc.GetMyPath(context.Background(), domain.User{ID: "alice", Role: domain.RoleStudent})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a teacher cannot access the student path view", func(t *testing.T) {
		svc := newPathAssignmentService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakePathAssignmentRepository(), newFakeCompletionStateReader())

		_, err := svc.GetMyPath(context.Background(), teacherCaller())

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("an admin cannot access the student path view", func(t *testing.T) {
		svc := newPathAssignmentService(newFakeUserRepository(), newFakeLearningPathRepository(), newFakePathAssignmentRepository(), newFakeCompletionStateReader())

		_, err := svc.GetMyPath(context.Background(), adminCaller())

		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
}
