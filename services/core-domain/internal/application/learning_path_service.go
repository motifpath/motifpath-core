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
	nodes          ports.ContentNodeRepository
	paths          ports.LearningPathRepository
	courseVersions ports.CourseVersionRepository
	newID          func() string
	now            func() time.Time
}

func NewLearningPathService(nodes ports.ContentNodeRepository, paths ports.LearningPathRepository, courseVersions ports.CourseVersionRepository, newID func() string, now func() time.Time) *LearningPathService {
	return &LearningPathService{nodes: nodes, paths: paths, courseVersions: courseVersions, newID: newID, now: now}
}

// PathItemInput is one item the caller wants in a new learning path: the
// content node it points at and its optional section label.
type PathItemInput struct {
	ContentNodeID string
	SectionLabel  *string
}

// CreateLearningPath creates a learning path from the given ordered items.
// Only teachers and admins may create learning paths. A content_node_id
// that doesn't exist is a validation failure (400), not a not-found error —
// the whole request is malformed, not a lookup that simply missed.
func (s *LearningPathService) CreateLearningPath(ctx context.Context, caller domain.User, title string, pathItems []PathItemInput) (domain.LearningPath, error) {
	if !canManageContent(caller.Role) {
		return domain.LearningPath{}, domain.ErrForbidden
	}

	items, err := s.resolvePathItems(ctx, pathItems)
	if err != nil {
		return domain.LearningPath{}, err
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

// ListLearningPaths returns one page of the learning paths in the library
// matching filter. Teachers and
// admins may list learning paths; students may not browse paths directly —
// their view is through PathAssignmentService.GetMyPath.
func (s *LearningPathService) ListLearningPaths(ctx context.Context, caller domain.User, filter domain.LearningPathFilter, page domain.PageRequest) (domain.Page[domain.LearningPath], error) {
	if !canManageContent(caller.Role) {
		return domain.Page[domain.LearningPath]{}, domain.ErrForbidden
	}
	return s.paths.List(ctx, filter, page)
}

// ReplaceLearningPath replaces the given path's title and items wholesale —
// the same way CreateLearningPath establishes them initially, reusing
// domain.NewLearningPath to recompute item positions from scratch. The
// path's id, owner, and creation time are preserved. A content_node_id that
// doesn't exist is a validation failure (400), matching CreateLearningPath.
// Only the creating teacher or an admin may replace a learning path.
// Returns domain.ErrNotFound if no path exists with the given id.
func (s *LearningPathService) ReplaceLearningPath(ctx context.Context, caller domain.User, id, title string, pathItems []PathItemInput) (domain.LearningPath, error) {
	if !canManageContent(caller.Role) {
		return domain.LearningPath{}, domain.ErrForbidden
	}

	existing, err := s.paths.GetByID(ctx, id)
	if err != nil {
		return domain.LearningPath{}, err
	}
	if err := requireOwner(caller, existing.TeacherID); err != nil {
		return domain.LearningPath{}, err
	}

	items, err := s.resolvePathItems(ctx, pathItems)
	if err != nil {
		return domain.LearningPath{}, err
	}

	replaced, err := domain.NewLearningPath(existing.ID, existing.TeacherID, title, items, existing.CreatedAt)
	if err != nil {
		return domain.LearningPath{}, err
	}
	if err := s.paths.Replace(ctx, replaced); err != nil {
		return domain.LearningPath{}, err
	}
	return replaced, nil
}

// DeleteLearningPath permanently deletes the learning path template with
// the given id. Never touches any StudentPath already copied from this
// template — each is an independent snapshot, unaffected by later changes
// to (or removal of) the template it came from. Only the creating teacher
// or an admin may delete a learning path. Refused with domain.ErrConflict
// if the template is referenced by a checkpoint of any published
// CourseVersion, even one belonging to a since-retired course — a
// published course's checkpoint sequence must always resolve. Returns
// domain.ErrNotFound if no path exists with the given id.
func (s *LearningPathService) DeleteLearningPath(ctx context.Context, caller domain.User, id string) error {
	if !canManageContent(caller.Role) {
		return domain.ErrForbidden
	}

	existing, err := s.paths.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := requireOwner(caller, existing.TeacherID); err != nil {
		return err
	}

	referenced, err := s.courseVersions.IsLearningPathReferenced(ctx, id)
	if err != nil {
		return err
	}
	if referenced {
		return domain.ErrConflict
	}

	return s.paths.Delete(ctx, id)
}

// resolvePathItems turns pathItems into the resolved domain.NewLearningPathItem
// slice domain.NewLearningPath needs, batching the content-node lookup into
// a single GetByIDs call. Returns a domain.ValidationError under
// "content_node_id" naming the first item whose content_node_id doesn't
// exist — shared by CreateLearningPath and ReplaceLearningPath, which
// resolve items identically.
func (s *LearningPathService) resolvePathItems(ctx context.Context, pathItems []PathItemInput) ([]domain.NewLearningPathItem, error) {
	contentNodeIDs := make([]string, len(pathItems))
	for i, item := range pathItems {
		contentNodeIDs[i] = item.ContentNodeID
	}

	found, err := s.nodes.GetByIDs(ctx, contentNodeIDs)
	if err != nil {
		return nil, err
	}

	items := make([]domain.NewLearningPathItem, 0, len(pathItems))
	for _, item := range pathItems {
		node, ok := found[item.ContentNodeID]
		if !ok {
			return nil, domain.NewValidationError("content_node_id", "references a content node that does not exist: "+item.ContentNodeID)
		}
		items = append(items, domain.NewLearningPathItem{Node: node, SectionLabel: item.SectionLabel})
	}
	return items, nil
}
