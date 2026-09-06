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

// PathItemInput is one item the caller wants in a new learning path: the
// content node it points at and its optional section label.
type PathItemInput struct {
	ContentNodeID string
	SectionLabel  string
}

// CreateLearningPath creates a learning path from the given ordered items.
// Only teachers and admins may create learning paths. A content_node_id
// that doesn't exist is a validation failure (400), not a not-found error —
// the whole request is malformed, not a lookup that simply missed.
func (s *LearningPathService) CreateLearningPath(ctx context.Context, caller domain.User, title string, pathItems []PathItemInput) (domain.LearningPath, error) {
	if !canManageContent(caller.Role) {
		return domain.LearningPath{}, domain.ErrForbidden
	}

	contentNodeIDs := make([]string, len(pathItems))
	for i, item := range pathItems {
		contentNodeIDs[i] = item.ContentNodeID
	}

	found, err := s.nodes.GetByIDs(ctx, contentNodeIDs)
	if err != nil {
		return domain.LearningPath{}, err
	}

	items := make([]domain.NewLearningPathItem, 0, len(pathItems))
	for _, item := range pathItems {
		node, ok := found[item.ContentNodeID]
		if !ok {
			return domain.LearningPath{}, domain.NewValidationError("content_node_id", "references a content node that does not exist: "+item.ContentNodeID)
		}
		items = append(items, domain.NewLearningPathItem{Node: node, SectionLabel: item.SectionLabel})
	}

	path, err := domain.NewLearningPath(s.newID(), caller.ID, title, items, s.now())
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
