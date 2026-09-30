package application

import (
	"context"
	"errors"
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
	versions       ports.ContentNodeVersionRepository
	languages      ports.LanguageRepository
	newID          func() string
	now            func() time.Time
	instruments    ports.InstrumentRepository
}

func NewLearningPathService(nodes ports.ContentNodeRepository, paths ports.LearningPathRepository, courseVersions ports.CourseVersionRepository, versions ports.ContentNodeVersionRepository, languages ports.LanguageRepository, instruments ports.InstrumentRepository, newID func() string, now func() time.Time) *LearningPathService {
	return &LearningPathService{nodes: nodes, paths: paths, courseVersions: courseVersions, versions: versions, languages: languages, newID: newID, now: now, instruments: instruments}
}

// PathItemInput is one item the caller wants in a new learning path: the
// content node it points at and its optional section label.
type PathItemInput struct {
	ContentNodeID string
	SectionLabel  *string
}

// LearningPathInput is what a caller writes to create or replace a learning
// path.
type LearningPathInput struct {
	Title string
	// Summary and Language may be nil while the path is a draft; publishing
	// requires both.
	Summary  *string
	Language *string
	Level    domain.DifficultyLevel
	// InstrumentIDs are the instruments the path is for; empty means every
	// instrument.
	InstrumentIDs []string
	// ThumbnailURL is the image shown for the path; nil means none.
	ThumbnailURL *string
	Items        []PathItemInput
}

// fields resolves input into the domain's LearningPathFields, given its items
// already resolved against their content nodes.
func (input LearningPathInput) fields(items []domain.NewLearningPathItem) domain.LearningPathFields {
	return domain.LearningPathFields{Title: input.Title, Summary: input.Summary, Language: input.Language, Level: input.Level, InstrumentIDs: input.InstrumentIDs, ThumbnailURL: input.ThumbnailURL, Items: items}
}

// CreateLearningPath creates a learning path from the given ordered items.
// Only teachers and admins may create learning paths. A content_node_id
// that doesn't exist is a validation failure (400), not a not-found error —
// the whole request is malformed, not a lookup that simply missed.
func (s *LearningPathService) CreateLearningPath(ctx context.Context, caller domain.User, input LearningPathInput) (domain.LearningPath, error) {
	if !canManageContent(caller.Role) {
		return domain.LearningPath{}, domain.ErrForbidden
	}

	items, err := s.resolvePathItems(ctx, input.Items)
	if err != nil {
		return domain.LearningPath{}, err
	}

	offered, err := offeredLanguages(ctx, s.languages)
	if err != nil {
		return domain.LearningPath{}, err
	}

	path, err := domain.NewLearningPath(s.newID(), caller.ID, input.fields(items), offered, s.now())
	if err != nil {
		return domain.LearningPath{}, err
	}
	if err := checkInstrumentsExist(ctx, s.instruments, input.InstrumentIDs); err != nil {
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
	if !filter.Sort.Valid() {
		return domain.Page[domain.LearningPath]{}, domain.NewValidationError("sort", "must be one of: title, updated")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return domain.Page[domain.LearningPath]{}, domain.NewValidationError("status", "must be one of: draft, published")
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
//
// Replacing never changes the path's status. A published path's edits are
// live — the catalog shows them and the next learner copies them — so a
// replace that would leave a published path unpublishable is refused with a
// *domain.LearningPathNotPublishableError and changes nothing.
func (s *LearningPathService) ReplaceLearningPath(ctx context.Context, caller domain.User, id string, input LearningPathInput) (domain.LearningPath, error) {
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

	items, err := s.resolvePathItems(ctx, input.Items)
	if err != nil {
		return domain.LearningPath{}, err
	}

	offered, err := offeredLanguages(ctx, s.languages)
	if err != nil {
		return domain.LearningPath{}, err
	}

	replaced, err := domain.NewLearningPath(existing.ID, existing.TeacherID, input.fields(items), offered, existing.CreatedAt)
	if err != nil {
		return domain.LearningPath{}, err
	}
	if err := checkInstrumentsExist(ctx, s.instruments, input.InstrumentIDs); err != nil {
		return domain.LearningPath{}, err
	}
	replaced.Status = existing.Status
	if replaced.Status == domain.LearningPathStatusPublished {
		if err := s.checkPublishable(ctx, replaced); err != nil {
			return domain.LearningPath{}, err
		}
	}
	replaced.UpdatedAt = s.now()
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
// published course's checkpoint sequence must always resolve — and while
// the path is published, so a path never vanishes from the catalog under a
// learner: it must be unpublished first. Returns domain.ErrNotFound if no
// path exists with the given id.
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
	if existing.Status == domain.LearningPathStatusPublished {
		return domain.ErrConflict
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

// PublishLearningPath lists the path in the catalog, where learners can find
// and enroll in it. Admin-only. Refused with a
// *domain.LearningPathNotPublishableError listing everything the path lacks.
// Publishing a published path returns it unchanged. Returns
// domain.ErrNotFound if no path exists with the given id.
func (s *LearningPathService) PublishLearningPath(ctx context.Context, caller domain.User, id string) (domain.LearningPath, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.LearningPath{}, domain.ErrForbidden
	}
	path, err := s.paths.GetByID(ctx, id)
	if err != nil {
		return domain.LearningPath{}, err
	}
	if path.Status == domain.LearningPathStatusPublished {
		return path, nil
	}
	if err := s.checkPublishable(ctx, path); err != nil {
		return domain.LearningPath{}, err
	}
	if err := s.paths.UpdateStatus(ctx, id, domain.LearningPathStatusPublished); err != nil {
		return domain.LearningPath{}, err
	}
	path.Status = domain.LearningPathStatusPublished
	return path, nil
}

// UnpublishLearningPath takes the path out of the catalog; no one else can
// enroll in it, and every copy already made is untouched. Admin-only.
// Refused with domain.ErrConflict while a checkpoint of any published
// CourseVersion uses the path — learners enrolled in that version still
// unlock its later checkpoints and copy their paths. Unpublishing a draft
// returns it unchanged. Returns domain.ErrNotFound if no path exists with
// the given id.
func (s *LearningPathService) UnpublishLearningPath(ctx context.Context, caller domain.User, id string) (domain.LearningPath, error) {
	if caller.Role != domain.RoleAdmin {
		return domain.LearningPath{}, domain.ErrForbidden
	}
	path, err := s.paths.GetByID(ctx, id)
	if err != nil {
		return domain.LearningPath{}, err
	}
	if path.Status == domain.LearningPathStatusDraft {
		return path, nil
	}
	referenced, err := s.courseVersions.IsLearningPathReferenced(ctx, id)
	if err != nil {
		return domain.LearningPath{}, err
	}
	if referenced {
		return domain.LearningPath{}, domain.ErrConflict
	}
	if err := s.paths.UpdateStatus(ctx, id, domain.LearningPathStatusDraft); err != nil {
		return domain.LearningPath{}, err
	}
	path.Status = domain.LearningPathStatusDraft
	return path, nil
}

// checkPublishable returns a *domain.LearningPathNotPublishableError when
// path lacks anything publishing requires, including items whose content
// node has never been published.
func (s *LearningPathService) checkPublishable(ctx context.Context, path domain.LearningPath) error {
	unpublished, err := s.unpublishedContentNodeIDs(ctx, path.Items)
	if err != nil {
		return err
	}
	if problems := path.PublishingProblems(unpublished); problems != nil {
		return problems
	}
	return nil
}

// unpublishedContentNodeIDs returns, in item order and without repeats, the
// items' content nodes that have no published version.
func (s *LearningPathService) unpublishedContentNodeIDs(ctx context.Context, items []domain.LearningPathItem) ([]string, error) {
	var unpublished []string
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if seen[item.ContentNodeID] {
			continue
		}
		seen[item.ContentNodeID] = true
		_, err := s.versions.GetLatestByContentNodeID(ctx, item.ContentNodeID)
		if errors.Is(err, domain.ErrNotFound) {
			unpublished = append(unpublished, item.ContentNodeID)
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return unpublished, nil
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
