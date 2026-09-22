package application

import (
	"context"
	"errors"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// StudentPathService assigns learning paths to students by copying a
// template's items into a new StudentPath, and composes a student's
// current StudentPath with their per-node completion state for GetMyPath.
type StudentPathService struct {
	users        ports.UserRepository
	paths        ports.LearningPathRepository
	studentPaths ports.StudentPathRepository
	versions     ports.ContentNodeVersionRepository
	state        ports.StudentLearningStateRepository
	contentNodes ports.ContentNodeRepository
	exercises    ports.ExerciseRepository
	completion   ports.CompletionStateReader
	newID        func() string
	now          func() time.Time
}

func NewStudentPathService(
	users ports.UserRepository,
	paths ports.LearningPathRepository,
	studentPaths ports.StudentPathRepository,
	versions ports.ContentNodeVersionRepository,
	state ports.StudentLearningStateRepository,
	contentNodes ports.ContentNodeRepository,
	exercises ports.ExerciseRepository,
	completion ports.CompletionStateReader,
	newID func() string,
	now func() time.Time,
) *StudentPathService {
	return &StudentPathService{
		users:        users,
		paths:        paths,
		studentPaths: studentPaths,
		versions:     versions,
		state:        state,
		contentNodes: contentNodes,
		exercises:    exercises,
		completion:   completion,
		newID:        newID,
		now:          now,
	}
}

// resolveLanguageLocks reports, per content_node_id in nodeIDs, whether it
// must be locked because locale matches neither the node's own language
// tags nor those of any exercise linked to it as a path exercise. A node id
// with no matching ContentNode record is left out of the result entirely
// (never locked by this check) rather than treated as a mismatch — path
// items in tests and any other caller that never separately registered the
// referenced ContentNode should not be penalized for data this check simply
// has no visibility into.
func (s *StudentPathService) resolveLanguageLocks(ctx context.Context, nodeIDs []string, locale string) (map[string]bool, error) {
	nodes, err := s.contentNodes.GetByIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}

	langLocked := map[string]bool{}
	for id, node := range nodes {
		if !domain.HasMatchingLanguage(node.Languages, locale) {
			langLocked[id] = true
			continue
		}

		pathExercises, err := s.exercises.ListByContentNodeID(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, ex := range pathExercises {
			if !domain.HasMatchingLanguage(ex.Languages, locale) {
				langLocked[id] = true
				break
			}
		}
	}
	return langLocked, nil
}

// resolveLatestVersionIDs looks up the latest published ContentNodeVersion
// for every content node referenced in items, returning domain.ErrNotFound
// if any one of them has never been published — assigning a path is
// refused wholesale in that case, per the OpenAPI contract, rather than
// copying a partial path.
func (s *StudentPathService) resolveLatestVersionIDs(ctx context.Context, items []domain.LearningPathItem) (map[string]string, error) {
	versionIDs := make(map[string]string, len(items))
	for _, item := range items {
		if _, ok := versionIDs[item.ContentNodeID]; ok {
			continue
		}
		version, err := s.versions.GetLatestByContentNodeID(ctx, item.ContentNodeID)
		if err != nil {
			return nil, err
		}
		versionIDs[item.ContentNodeID] = version.ID
	}
	return versionIDs, nil
}

// AssignLearningPath copies learningPathID's current items into a new,
// standalone StudentPath owned by studentID, and sets it as the student's
// current path unconditionally. Only teachers and admins may assign paths.
// Every content node the template's items reference must already have at
// least one published version.
func (s *StudentPathService) AssignLearningPath(ctx context.Context, caller domain.User, studentID, learningPathID string) (domain.StudentPath, error) {
	if !canManageContent(caller.Role) {
		return domain.StudentPath{}, domain.ErrForbidden
	}

	student, err := s.users.GetByID(ctx, studentID)
	if err != nil {
		return domain.StudentPath{}, err
	}
	if student.Role != domain.RoleStudent {
		// A user that exists but isn't a student is reported as not found,
		// not forbidden — the assignment target space is students, so a
		// teacher_id simply isn't a valid target, same as an id that
		// doesn't exist at all.
		return domain.StudentPath{}, domain.ErrNotFound
	}

	template, err := s.paths.GetByID(ctx, learningPathID)
	if err != nil {
		return domain.StudentPath{}, err
	}

	versionIDs, err := s.resolveLatestVersionIDs(ctx, template.Items)
	if err != nil {
		return domain.StudentPath{}, err
	}

	sp, err := domain.NewStudentPathFromTemplate(s.newID(), studentID, template, caller.ID, s.now(), versionIDs)
	if err != nil {
		return domain.StudentPath{}, err
	}
	if err := s.studentPaths.Create(ctx, sp); err != nil {
		return domain.StudentPath{}, err
	}

	existingState, err := s.state.GetByStudentID(ctx, studentID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.StudentPath{}, err
	}
	existingState.StudentID = studentID
	if err := s.state.Upsert(ctx, existingState.WithCurrentStandalonePath(sp.ID)); err != nil {
		return domain.StudentPath{}, err
	}

	return sp, nil
}

// StudentPathView is the authenticated caller's current learning path with
// per-item progress state — the composed result GetMyPath returns.
type StudentPathView struct {
	StudentPathID    string
	SourceTemplateID string
	Title            string
	CurrentPosition  int
	Items            []domain.StudentPathItem
}

// GetMyPath returns caller's current StudentPath together with per-item
// progress state, composing StudentLearningStateRepository +
// StudentPathRepository + CompletionStateReader. Accessible by any
// authenticated role — each caller only ever sees their own current path,
// keyed by their own id. Returns domain.ErrNotFound if caller has no
// current path set.
func (s *StudentPathService) GetMyPath(ctx context.Context, caller domain.User) (StudentPathView, error) {
	state, err := s.state.GetByStudentID(ctx, caller.ID)
	if err != nil {
		return StudentPathView{}, err
	}
	if state.CurrentStandalonePathID == nil {
		// Course-enrollment checkpoints are not implemented yet — a caller
		// whose only current pointer is a course enrollment has no
		// resolvable StudentPath here.
		return StudentPathView{}, domain.ErrNotFound
	}

	sp, err := s.studentPaths.GetByID(ctx, *state.CurrentStandalonePathID)
	if err != nil {
		return StudentPathView{}, err
	}

	items := make([]domain.LearningPathItem, len(sp.Items))
	versionByNode := make(map[string]string, len(sp.Items))
	nodeIDs := make([]string, len(sp.Items))
	for i, item := range sp.Items {
		nodeIDs[i] = item.ContentNodeID
		versionByNode[item.ContentNodeID] = item.ContentNodeVersionID
	}

	nodes, err := s.contentNodes.GetByIDs(ctx, nodeIDs)
	if err != nil {
		return StudentPathView{}, err
	}
	for i, item := range sp.Items {
		node := nodes[item.ContentNodeID]
		items[i] = domain.LearningPathItem{
			Position:      item.Position,
			ContentNodeID: item.ContentNodeID,
			Title:         node.Title,
			ContentType:   node.ContentType,
			SectionLabel:  item.SectionLabel,
		}
	}

	raw, err := s.completion.GetStatuses(ctx, caller.ID, nodeIDs)
	if err != nil {
		return StudentPathView{}, err
	}

	langLocked, err := s.resolveLanguageLocks(ctx, nodeIDs, caller.Locale.Code)
	if err != nil {
		return StudentPathView{}, err
	}

	viewItems, currentPosition := domain.BuildStudentPathItems(items, raw, langLocked)
	for i := range viewItems {
		viewItems[i].ContentNodeVersionID = versionByNode[viewItems[i].ContentNodeID]
	}

	return StudentPathView{
		StudentPathID:    sp.ID,
		SourceTemplateID: sp.SourceTemplateID,
		Title:            sp.Title,
		CurrentPosition:  currentPosition,
		Items:            viewItems,
	}, nil
}

// ArchiveStandaloneStudentPath archives the standalone StudentPath with the
// given id, owned by caller. Refused with domain.ErrNotFound if no such
// non-archived, non-course StudentPath exists for caller, and with
// domain.ErrConflict if it is caller's only current course or path and no
// other eligible StudentPath exists to become current instead.
func (s *StudentPathService) ArchiveStandaloneStudentPath(ctx context.Context, caller domain.User, studentPathID string) (domain.StudentPath, error) {
	sp, err := s.studentPaths.GetByID(ctx, studentPathID)
	if err != nil {
		return domain.StudentPath{}, err
	}
	if sp.StudentID != caller.ID || sp.ArchivedAt != nil || sp.SourceCourseEnrollmentID != nil {
		return domain.StudentPath{}, domain.ErrNotFound
	}

	state, err := s.state.GetByStudentID(ctx, caller.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.StudentPath{}, err
	}
	isCurrent := state.CurrentStandalonePathID != nil && *state.CurrentStandalonePathID == sp.ID

	if isCurrent {
		hasOtherEligible, err := s.hasOtherEligibleStandalonePath(ctx, caller.ID, sp.ID)
		if err != nil {
			return domain.StudentPath{}, err
		}
		if !hasOtherEligible {
			return domain.StudentPath{}, domain.ErrConflict
		}
	}

	archivedAt := s.now()
	if err := s.studentPaths.Archive(ctx, sp.ID, archivedAt); err != nil {
		return domain.StudentPath{}, err
	}
	sp.ArchivedAt = &archivedAt

	if isCurrent {
		if err := s.state.Upsert(ctx, state.Cleared()); err != nil {
			return domain.StudentPath{}, err
		}
	}

	return sp, nil
}

// hasOtherEligibleStandalonePath reports whether studentID holds a
// non-archived, non-course StudentPath other than excludeID — split out of
// ArchiveStandaloneStudentPath to keep its own cognitive complexity down.
func (s *StudentPathService) hasOtherEligibleStandalonePath(ctx context.Context, studentID, excludeID string) (bool, error) {
	others, err := s.studentPaths.ListActiveStandaloneByStudentID(ctx, studentID)
	if err != nil {
		return false, err
	}
	for _, other := range others {
		if other.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}
