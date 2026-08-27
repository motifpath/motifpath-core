package application

import (
	"context"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ContentService manages ContentNode and ExpandedContent — content-node
// creation/retrieval and the expositive media items attached to them.
type ContentService struct {
	nodes    ports.ContentNodeRepository
	expanded ports.ExpandedContentRepository
	newID    func() string
	now      func() time.Time
}

func NewContentService(nodes ports.ContentNodeRepository, expanded ports.ExpandedContentRepository, newID func() string, now func() time.Time) *ContentService {
	return &ContentService{nodes: nodes, expanded: expanded, newID: newID, now: now}
}

// CreateContentNode creates a content node owned by caller. Only teachers
// and admins may create content nodes.
func (s *ContentService) CreateContentNode(ctx context.Context, caller domain.User, title string, contentType domain.ContentType, skill, concept string, difficulty domain.DifficultyLevel) (domain.ContentNode, error) {
	if !canManageContent(caller.Role) {
		return domain.ContentNode{}, domain.ErrForbidden
	}

	node, err := domain.NewContentNode(s.newID(), caller.ID, title, contentType, skill, concept, difficulty, s.now())
	if err != nil {
		return domain.ContentNode{}, err
	}
	if err := s.nodes.Create(ctx, node); err != nil {
		return domain.ContentNode{}, err
	}
	return node, nil
}

// GetContentNode returns the content node with the given id. Any
// authenticated user may retrieve a content node.
func (s *ContentService) GetContentNode(ctx context.Context, id string) (domain.ContentNode, error) {
	return s.nodes.GetByID(ctx, id)
}

// CreateExpandedContent attaches an expositive media item to the content
// node identified by contentNodeID. Only teachers and admins may add
// expanded content.
func (s *ContentService) CreateExpandedContent(
	ctx context.Context,
	caller domain.User,
	contentNodeID string,
	contentType domain.ExpandedContentType,
	mediaURL string,
	triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS *int,
	caption *string,
) (domain.ExpandedContent, error) {
	if !canManageContent(caller.Role) {
		return domain.ExpandedContent{}, domain.ErrForbidden
	}

	node, err := s.nodes.GetByID(ctx, contentNodeID)
	if err != nil {
		return domain.ExpandedContent{}, err
	}

	item, err := domain.NewExpandedContent(
		s.newID(), contentNodeID, node.ContentType, contentType, mediaURL,
		triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS, caption, s.now(),
	)
	if err != nil {
		return domain.ExpandedContent{}, err
	}
	if err := s.expanded.Create(ctx, item); err != nil {
		return domain.ExpandedContent{}, err
	}
	return item, nil
}

// ListExpandedContent returns all expanded content items for a content
// node, ordered by trigger position. Any authenticated user may list them.
func (s *ContentService) ListExpandedContent(ctx context.Context, contentNodeID string) ([]domain.ExpandedContent, error) {
	if _, err := s.nodes.GetByID(ctx, contentNodeID); err != nil {
		return nil, err
	}
	return s.expanded.ListByContentNode(ctx, contentNodeID)
}

// GetExpandedContent returns the expanded content item with the given id.
// Any authenticated user may retrieve one.
func (s *ContentService) GetExpandedContent(ctx context.Context, id string) (domain.ExpandedContent, error) {
	return s.expanded.GetByID(ctx, id)
}

// canManageContent reports whether role may create content nodes,
// challenges, exercises, and expanded content, and create/assign learning
// paths — every write endpoint in this service shares the same
// teacher-or-admin gate.
func canManageContent(role domain.Role) bool {
	return role == domain.RoleTeacher || role == domain.RoleAdmin
}
