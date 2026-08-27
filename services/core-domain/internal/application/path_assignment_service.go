package application

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// PathAssignmentService assigns learning paths to students and composes a
// student's active assignment with their per-node completion state for
// GetMyPath.
type PathAssignmentService struct {
	users       ports.UserRepository
	paths       ports.LearningPathRepository
	assignments ports.PathAssignmentRepository
	completion  ports.CompletionStateReader
	newID       func() string
	now         func() time.Time
}

func NewPathAssignmentService(
	users ports.UserRepository,
	paths ports.LearningPathRepository,
	assignments ports.PathAssignmentRepository,
	completion ports.CompletionStateReader,
	newID func() string,
	now func() time.Time,
) *PathAssignmentService {
	return &PathAssignmentService{
		users:       users,
		paths:       paths,
		assignments: assignments,
		completion:  completion,
		newID:       newID,
		now:         now,
	}
}

// AssignLearningPath assigns learningPathID to studentID, replacing any
// existing active assignment. Only teachers and admins may assign paths.
func (s *PathAssignmentService) AssignLearningPath(ctx context.Context, caller domain.User, studentID, learningPathID string) (domain.PathAssignment, error) {
	if !canManageContent(caller.Role) {
		return domain.PathAssignment{}, domain.ErrForbidden
	}

	student, err := s.users.GetByID(ctx, studentID)
	if err != nil {
		return domain.PathAssignment{}, err
	}
	if student.Role != domain.RoleStudent {
		// A user that exists but isn't a student is reported as not found,
		// not forbidden: per the Gherkin scenario "Assigning a path to a
		// user with role teacher returns not found," the assignment target
		// space is students — a teacher_id simply isn't a valid target,
		// same as an id that doesn't exist at all.
		return domain.PathAssignment{}, domain.ErrNotFound
	}

	if _, err := s.paths.GetByID(ctx, learningPathID); err != nil {
		return domain.PathAssignment{}, err
	}

	assignment := domain.PathAssignment{
		ID:             s.newID(),
		StudentID:      studentID,
		LearningPathID: learningPathID,
		AssignedBy:     caller.ID,
		AssignedAt:     s.now(),
	}
	if err := s.assignments.ReplaceActive(ctx, assignment); err != nil {
		return domain.PathAssignment{}, err
	}
	return assignment, nil
}

// StudentPathView is the authenticated student's active learning path with
// per-item progress state — the composed result GetMyPath returns.
type StudentPathView struct {
	AssignmentID    string
	LearningPathID  string
	Title           string
	CurrentPosition int
	Items           []domain.StudentPathItem
}

// GetMyPath returns caller's active learning path assignment together with
// per-item progress state, composing PathAssignmentRepository +
// LearningPathRepository + CompletionStateReader per ADR-011/Phase 4.4.
// Only students may access this. Returns domain.ErrNotFound if caller has
// no active assignment.
func (s *PathAssignmentService) GetMyPath(ctx context.Context, caller domain.User) (StudentPathView, error) {
	if caller.Role != domain.RoleStudent {
		return StudentPathView{}, domain.ErrForbidden
	}

	assignment, err := s.assignments.GetActiveByStudentID(ctx, caller.ID)
	if err != nil {
		return StudentPathView{}, err
	}

	path, err := s.paths.GetByID(ctx, assignment.LearningPathID)
	if err != nil {
		return StudentPathView{}, err
	}

	nodeIDs := make([]string, len(path.Items))
	for i, item := range path.Items {
		nodeIDs[i] = item.ContentNodeID
	}

	raw, err := s.completion.GetStatuses(ctx, caller.ID, nodeIDs)
	if err != nil {
		return StudentPathView{}, err
	}

	items, currentPosition := domain.BuildStudentPathItems(path.Items, raw)

	return StudentPathView{
		AssignmentID:    assignment.ID,
		LearningPathID:  path.ID,
		Title:           path.Title,
		CurrentPosition: currentPosition,
		Items:           items,
	}, nil
}
