package application

import (
	"context"
	"errors"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ContentService manages ContentNode and ExpandedContent — content-node
// creation/retrieval, the expositive media items attached to them, and
// publishing a content node's draft into an immutable ContentNodeVersion.
type ContentService struct {
	nodes       ports.ContentNodeRepository
	expanded    ports.ExpandedContentRepository
	skills      ports.SkillRepository
	concepts    ports.ConceptRepository
	versions    ports.ContentNodeVersionRepository
	diagrams    ports.DiagramRepository
	newID       func() string
	now         func() time.Time
	instruments ports.InstrumentRepository
}

func NewContentService(nodes ports.ContentNodeRepository, expanded ports.ExpandedContentRepository, skills ports.SkillRepository, concepts ports.ConceptRepository, versions ports.ContentNodeVersionRepository, diagrams ports.DiagramRepository, instruments ports.InstrumentRepository, newID func() string, now func() time.Time) *ContentService {
	return &ContentService{nodes: nodes, expanded: expanded, skills: skills, concepts: concepts, versions: versions, diagrams: diagrams, newID: newID, now: now, instruments: instruments}
}

// PublishContentNode snapshots the content node identified by id into a new,
// immutable ContentNodeVersion — the node's next version_number, one
// greater than whatever was last published (or 1 if this is the first
// publish). Only the creating teacher or an admin may publish it. A
// StudentPath copied afterwards pins to this version; editing or
// republishing the node later never retargets it.
func (s *ContentService) PublishContentNode(ctx context.Context, caller domain.User, id string) (domain.ContentNodeVersion, error) {
	if !canManageContent(caller.Role) {
		return domain.ContentNodeVersion{}, domain.ErrForbidden
	}

	node, err := s.nodes.GetByID(ctx, id)
	if err != nil {
		return domain.ContentNodeVersion{}, err
	}
	if err := requireOwner(caller, node.TeacherID); err != nil {
		return domain.ContentNodeVersion{}, err
	}

	nextVersionNumber := 1
	if latest, err := s.versions.GetLatestByContentNodeID(ctx, id); err == nil {
		nextVersionNumber = latest.VersionNumber + 1
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.ContentNodeVersion{}, err
	}

	version := domain.NewContentNodeVersionSnapshot(s.newID(), node, nextVersionNumber, caller.ID, s.now())
	if err := s.versions.Create(ctx, version); err != nil {
		return domain.ContentNodeVersion{}, err
	}
	return version, nil
}

// ListContentNodeVersions returns every published version of the content
// node identified by id, newest first, or an empty list if it has never been
// published. A version published before classification and language
// snapshots were stored carries the node's current ones instead. Only the
// creating teacher or an admin may view a node's history — the same rule
// as publishing it. Returns domain.ErrNotFound if no
// content node exists with the given id.
func (s *ContentService) ListContentNodeVersions(ctx context.Context, caller domain.User, id string) ([]domain.ContentNodeVersion, error) {
	if !canManageContent(caller.Role) {
		return nil, domain.ErrForbidden
	}

	node, err := s.nodes.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := requireOwner(caller, node.TeacherID); err != nil {
		return nil, err
	}

	versions, err := s.versions.ListByContentNodeID(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range versions {
		if versions[i].Classification.DifficultyLevel != "" {
			continue
		}
		// Published before classification and language snapshots were
		// stored, so the node's current values are the best record left.
		versions[i].Classification = node.Classification
		versions[i].Languages = node.Languages
	}
	return versions, nil
}

// ContentNodeInput is what a caller writes to create or update a content
// node. ContentType is read on create only: it can't change afterwards.
type ContentNodeInput struct {
	Title       string
	ContentType domain.ContentType
	SkillIDs    []string
	ConceptIDs  []string
	Difficulty  domain.DifficultyLevel
	Languages   []string
	MediaURL    *string
	RichContent *domain.PromptDocument
	// InstrumentIDs are the instruments the node is for; empty means every
	// instrument.
	InstrumentIDs []string
}

func (input ContentNodeInput) fields() domain.ContentNodeFields {
	return domain.ContentNodeFields{
		Title: input.Title, SkillIDs: input.SkillIDs, ConceptIDs: input.ConceptIDs, Difficulty: input.Difficulty,
		LanguageCodes: input.Languages, MediaURL: input.MediaURL, RichContent: input.RichContent, InstrumentIDs: input.InstrumentIDs,
	}
}

// CreateContentNode creates a content node owned by caller. Only teachers
// and admins may create content nodes.
func (s *ContentService) CreateContentNode(ctx context.Context, caller domain.User, input ContentNodeInput) (domain.ContentNode, error) {
	if !canManageContent(caller.Role) {
		return domain.ContentNode{}, domain.ErrForbidden
	}

	node, err := domain.NewContentNode(s.newID(), caller.ID, input.ContentType, input.fields(), s.now())
	if err != nil {
		return domain.ContentNode{}, err
	}
	if err := checkSkillsAndConceptsExist(ctx, s.skills, s.concepts, input.SkillIDs, input.ConceptIDs); err != nil {
		return domain.ContentNode{}, err
	}
	if err := checkInstrumentsExist(ctx, s.instruments, input.InstrumentIDs); err != nil {
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

// ListContentNodes returns one page of the content nodes from the library
// matching filter (a zero-valued field means "no filter"). Only teachers and admins may list content nodes — the
// library is an authoring surface, unlike GetContentNode which any
// authenticated user may call for a specific known id.
func (s *ContentService) ListContentNodes(ctx context.Context, caller domain.User, filter domain.ContentNodeFilter, page domain.PageRequest) (domain.Page[domain.ContentNode], error) {
	if !canManageContent(caller.Role) {
		return domain.Page[domain.ContentNode]{}, domain.ErrForbidden
	}
	return s.nodes.List(ctx, filter, page)
}

// UpdateContentNode replaces the given content node's title and
// classification. content_type and the classification's review state are
// untouched. Only the creating teacher or an admin may update a content
// node. Returns domain.ErrNotFound if no content node exists with the given
// id.
func (s *ContentService) UpdateContentNode(ctx context.Context, caller domain.User, id string, input ContentNodeInput) (domain.ContentNode, error) {
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

	updated, err := existing.Update(input.fields())
	if err != nil {
		return domain.ContentNode{}, err
	}
	if err := checkSkillsAndConceptsExist(ctx, s.skills, s.concepts, input.SkillIDs, input.ConceptIDs); err != nil {
		return domain.ContentNode{}, err
	}
	if err := checkInstrumentsExist(ctx, s.instruments, input.InstrumentIDs); err != nil {
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
	diagramRef *domain.DiagramRef,
	diagramStackRef *domain.DiagramStackRef,
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
		s.newID(), contentNodeID, node.ContentType, contentType, mediaURL, richContent, diagramRef, diagramStackRef,
		triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS, caption, s.now(),
	)
	if err != nil {
		return domain.ExpandedContent{}, err
	}
	if err := checkDiagramRefsExist(ctx, s.diagrams, diagramRef, diagramStackRef); err != nil {
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
	diagramRef *domain.DiagramRef,
	diagramStackRef *domain.DiagramStackRef,
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

	updated, err := existing.Update(node.ContentType, contentType, mediaURL, richContent, diagramRef, diagramStackRef,
		triggerAtSeconds, hideAtSeconds, triggerAtParagraph, durationMS, caption)
	if err != nil {
		return domain.ExpandedContent{}, err
	}
	if err := checkDiagramRefsExist(ctx, s.diagrams, diagramRef, diagramStackRef); err != nil {
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
