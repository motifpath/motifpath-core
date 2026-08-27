package application

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// LearningPathService manages LearningPath — ordered sequences of content
// nodes that students follow.
type LearningPathService struct {
	nodes ports.ContentNodeRepository
	paths ports.LearningPathRepository
	newID func() string
	now   func() time.Time
}

func NewLearningPathService(nodes ports.ContentNodeRepository, paths ports.LearningPathRepository, newID func() string, now func() time.Time) *LearningPathService {
	return &LearningPathService{nodes: nodes, paths: paths, newID: newID, now: now}
}

// CreateLearningPath creates a learning path with the given ordered content
// node ids. Only teachers and admins may create learning paths. A
// content_node_id that doesn't exist is a validation failure (400), not a
// not-found error — the whole request is malformed, not a lookup that
// simply missed.
func (s *LearningPathService) CreateLearningPath(ctx context.Context, caller domain.User, title string, contentNodeIDs []string) (domain.LearningPath, error) {
	if !canManageContent(caller.Role) {
		return domain.LearningPath{}, domain.ErrForbidden
	}

	found, err := s.nodes.GetByIDs(ctx, contentNodeIDs)
	if err != nil {
		return domain.LearningPath{}, err
	}

	nodes := make([]domain.ContentNode, 0, len(contentNodeIDs))
	for _, id := range contentNodeIDs {
		node, ok := found[id]
		if !ok {
			return domain.LearningPath{}, domain.NewValidationError("content_node_id", "references a content node that does not exist: "+id)
		}
		nodes = append(nodes, node)
	}

	path, err := domain.NewLearningPath(s.newID(), caller.ID, title, nodes, s.now())
	if err != nil {
		return domain.LearningPath{}, err
	}
	if err := s.paths.Create(ctx, path); err != nil {
		return domain.LearningPath{}, err
	}
	return path, nil
}

// GetLearningPath returns the learning path with the given id. Teachers and
// admins may retrieve any path; students may not browse paths directly —
// their view is through PathAssignmentService.GetMyPath.
func (s *LearningPathService) GetLearningPath(ctx context.Context, caller domain.User, id string) (domain.LearningPath, error) {
	if !canManageContent(caller.Role) {
		return domain.LearningPath{}, domain.ErrForbidden
	}
	return s.paths.GetByID(ctx, id)
}
