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
	skills   ports.SkillRepository
	concepts ports.ConceptRepository
	newID    func() string
	now      func() time.Time
}

func NewContentService(nodes ports.ContentNodeRepository, expanded ports.ExpandedContentRepository, skills ports.SkillRepository, concepts ports.ConceptRepository, newID func() string, now func() time.Time) *ContentService {
	return &ContentService{nodes: nodes, expanded: expanded, skills: skills, concepts: concepts, newID: newID, now: now}
}

// CreateContentNode creates a content node owned by caller. Only teachers
// and admins may create content nodes.
func (s *ContentService) CreateContentNode(ctx context.Context, caller domain.User, title string, contentType domain.ContentType, skillIDs, conceptIDs []string, difficulty domain.DifficultyLevel, languages []string) (domain.ContentNode, error) {
	if !canManageContent(caller.Role) {
		return domain.ContentNode{}, domain.ErrForbidden
	}

	node, err := domain.NewContentNode(s.newID(), caller.ID, title, contentType, skillIDs, conceptIDs, difficulty, languages, s.now())
	if err != nil {
		return domain.ContentNode{}, err
	}
	if err := checkSkillsAndConceptsExist(ctx, s.skills, s.concepts, skillIDs, conceptIDs); err != nil {
		return domain.ContentNode{}, err
	}
	if err := s.nodes.Create(ctx, node); err != nil {
		return domain.ContentNode{}, err
	}
	// Re-fetched rather than returned as constructed: node.Languages/Skills/
	// Concepts only carry the request-supplied codes/ids until read back
	// with their rows (and, for Skills/Concepts, Name/ParentID) joined in.
	return s.nodes.GetByID(ctx, node.ID)
}

// GetContentNode returns the content node with the given id. Any
// authenticated user may retrieve a content node.
func (s *ContentService) GetContentNode(ctx context.Context, id string) (domain.ContentNode, error) {
	return s.nodes.GetByID(ctx, id)
}

// ListContentNodes returns content nodes from the library, optionally
// narrowed by contentType, skillID, conceptID, and/or difficulty (any may be
// "" for "no filter"). Only teachers and admins may list content nodes — the
// library is an authoring surface, unlike GetContentNode which any
// authenticated user may call for a specific known id.
func (s *ContentService) ListContentNodes(ctx context.Context, caller domain.User, contentType domain.ContentType, skillID, conceptID string, difficulty domain.DifficultyLevel) ([]domain.ContentNode, error) {
	if !canManageContent(caller.Role) {
		return nil, domain.ErrForbidden
	}
	return s.nodes.List(ctx, contentType, skillID, conceptID, difficulty)
}

// UpdateContentNode replaces the given content node's title and
// classification. content_type and the classification's review state are
// untouched. Only the creating teacher or an admin may update a content
// node. Returns domain.ErrNotFound if no content node exists with the given
// id.
func (s *ContentService) UpdateContentNode(ctx context.Context, caller domain.User, id, title string, skillIDs, conceptIDs []string, difficulty domain.DifficultyLevel, languages []string) (domain.ContentNode, error) {
	if !canManageContent(caller.Role) {
		return domain.ContentNode{}, domain.ErrForbidden
	}

	existing, err := s.nodes.GetByID(ctx, id)
	if err != nil {
		return domain.ContentNode{}, err
	}
	if err := requireOwner(caller, existing.TeacherID); err != nil {
		return domain.ContentNode{}, err
	}

	updated, err := existing.Update(title, skillIDs, conceptIDs, difficulty, languages)
	if err != nil {
		return domain.ContentNode{}, err
	}
	if err := checkSkillsAndConceptsExist(ctx, s.skills, s.concepts, skillIDs, conceptIDs); err != nil {
		return domain.ContentNode{}, err
	}

	if err := s.nodes.Update(ctx, updated); err != nil {
		return domain.ContentNode{}, err
	}
	// Re-fetched rather than returned as updated: updated.Skills/Concepts
	// only carry the request-supplied ids until read back with their rows
	// (Name/ParentID) joined in — same convention CreateContentNode follows.
	return s.nodes.GetByID(ctx, updated.ID)
}

// CreateExpandedContent attaches an expositive media item to the content
// node identified by contentNodeID. Only teachers and admins may add
// expanded content.
func (s *ContentService) CreateExpandedContent(
	ctx context.Context,
	caller domain.User,
	contentNodeID string,
	contentType domain.ExpandedContentType,
	mediaURL *string,
	richContent *domain.PromptDocument,
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
		s.newID(), contentNodeID, node.ContentType, contentType, mediaURL, richContent,
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

// UpdateExpandedContent replaces the given expanded content item's content,
// trigger/hide position, and caption. Only teachers and admins may update an
// expanded content item. Returns domain.ErrNotFound if no item exists with
// the given id.
func (s *ContentService) UpdateExpandedContent(
	ctx context.Context,
	caller domain.User,
	id string,
	contentType domain.ExpandedContentType,
	mediaURL *string,
	richContent *domain.PromptDocument,
	triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS *int,
	caption *string,
) (domain.ExpandedContent, error) {
	if !canManageContent(caller.Role) {
		return domain.ExpandedContent{}, domain.ErrForbidden
	}

	existing, err := s.expanded.GetByID(ctx, id)
	if err != nil {
		return domain.ExpandedContent{}, err
	}
	node, err := s.nodes.GetByID(ctx, existing.ContentNodeID)
	if err != nil {
		return domain.ExpandedContent{}, err
	}
	if err := requireOwner(caller, node.TeacherID); err != nil {
		return domain.ExpandedContent{}, err
	}

	updated, err := existing.Update(node.ContentType, contentType, mediaURL, richContent,
		triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS, caption)
	if err != nil {
		return domain.ExpandedContent{}, err
	}

	if err := s.expanded.Update(ctx, updated); err != nil {
		return domain.ExpandedContent{}, err
	}
	return updated, nil
}

// DeleteExpandedContent permanently removes the given expanded content item.
// Only the creating teacher or an admin may delete an expanded content
// item. Returns domain.ErrNotFound if no item exists with the given id.
func (s *ContentService) DeleteExpandedContent(ctx context.Context, caller domain.User, id string) error {
	if !canManageContent(caller.Role) {
		return domain.ErrForbidden
	}

	existing, err := s.expanded.GetByID(ctx, id)
	if err != nil {
		return err
	}
	node, err := s.nodes.GetByID(ctx, existing.ContentNodeID)
	if err != nil {
		return err
	}
	if err := requireOwner(caller, node.TeacherID); err != nil {
		return err
	}

	return s.expanded.Delete(ctx, id)
}

// canManageContent reports whether role may create content nodes,
// challenges, exercises, and expanded content, and create/assign learning
// paths — every write endpoint in this service shares the same
// teacher-or-admin gate. Creating content needs nothing more than this: the
// caller becomes the resource's owner by definition. Updating, deleting, or
// replacing an existing resource additionally needs requireOwner below,
// since canManageContent alone can't tell one teacher's content from
// another's.
func canManageContent(role domain.Role) bool {
	return role == domain.RoleTeacher || role == domain.RoleAdmin
}

// requireOwner returns domain.ErrForbidden unless caller may modify a
// resource owned by teacherID — an admin may modify any teacher's content;
// a teacher may only modify their own. Callers check canManageContent (or
// equivalent) first to reject a student outright before ever fetching the
// resource whose ownership this checks.
func requireOwner(caller domain.User, teacherID string) error {
	if caller.Role == domain.RoleAdmin {
		return nil
	}
	if caller.Role == domain.RoleTeacher && caller.ID == teacherID {
		return nil
	}
	return domain.ErrForbidden
}
