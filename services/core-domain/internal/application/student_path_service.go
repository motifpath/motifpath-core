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
	users          ports.UserRepository
	paths          ports.LearningPathRepository
	studentPaths   ports.StudentPathRepository
	versions       ports.ContentNodeVersionRepository
	state          ports.StudentLearningStateRepository
	enrollments    ports.CourseEnrollmentRepository
	courseVersions ports.CourseVersionRepository
	contentNodes   ports.ContentNodeRepository
	exercises      ports.ExerciseRepository
	completion     ports.CompletionStateReader
	newID          func() string
	now            func() time.Time
}

func NewStudentPathService(
	users ports.UserRepository,
	paths ports.LearningPathRepository,
	studentPaths ports.StudentPathRepository,
	versions ports.ContentNodeVersionRepository,
	state ports.StudentLearningStateRepository,
	enrollments ports.CourseEnrollmentRepository,
	courseVersions ports.CourseVersionRepository,
	contentNodes ports.ContentNodeRepository,
	exercises ports.ExerciseRepository,
	completion ports.CompletionStateReader,
	newID func() string,
	now func() time.Time,
) *StudentPathService {
	return &StudentPathService{
		users:          users,
		paths:          paths,
		studentPaths:   studentPaths,
		versions:       versions,
		state:          state,
		enrollments:    enrollments,
		courseVersions: courseVersions,
		contentNodes:   contentNodes,
		exercises:      exercises,
		completion:     completion,
		newID:          newID,
		now:            now,
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
// per-item progress state — the composed result GetMyPath and
// SetCurrentPath return.
type StudentPathView struct {
	StudentPathID    string
	SourceTemplateID string
	Title            string
	CurrentPosition  int
	Items            []domain.StudentPathItem
	// CourseEnrollmentID is the CourseEnrollment this view's StudentPath is
	// a checkpoint of, or nil when the caller's current path is a
	// standalone one. Set together with CourseCheckpointPosition.
	CourseEnrollmentID *string
	// CourseCheckpointPosition is the 1-based position of this checkpoint
	// within its course, or nil for a standalone path.
	CourseCheckpointPosition *int
	// CourseCompleted is true only on the specific response that discovers
	// this checkpoint completing the student's course: every item in Items
	// is complete, and the caller's current pointer has already been
	// cleared as a result of this same call. Always false for a standalone
	// path, and false again on every subsequent read — the pointer is nil
	// afterward, so there is nothing left to signal completion on. A
	// one-time signal, never a recheckable status.
	CourseCompleted bool
}

// CopyTemplateForCheckpoint copies template's items into a new StudentPath
// for studentID, marking it as the checkpoint at checkpointPosition for
// courseEnrollmentID — the same copy-on-assign flow AssignLearningPath
// performs for standalone paths, reused rather than duplicated so a
// self-enrollment's checkpoint 1 StudentPath is built identically to a
// staff-assigned one.
func (s *StudentPathService) CopyTemplateForCheckpoint(ctx context.Context, studentID string, template domain.LearningPath, assignedBy, courseEnrollmentID string, checkpointPosition int) (domain.StudentPath, error) {
	versionIDs, err := s.resolveLatestVersionIDs(ctx, template.Items)
	if err != nil {
		return domain.StudentPath{}, err
	}

	sp, err := domain.NewStudentPathFromTemplate(s.newID(), studentID, template, assignedBy, s.now(), versionIDs)
	if err != nil {
		return domain.StudentPath{}, err
	}
	sp.SourceCourseEnrollmentID = &courseEnrollmentID
	sp.CourseCheckpointPosition = &checkpointPosition

	if err := s.studentPaths.Create(ctx, sp); err != nil {
		return domain.StudentPath{}, err
	}
	return sp, nil
}

// resolveCurrentStudentPath returns the StudentPath the caller's current
// pointer resolves to, plus the CourseEnrollment/checkpoint-position pair
// when that pointer is a course enrollment (both nil for a standalone
// path), plus whether this call is the one that just discovered the
// course completing. Returns domain.ErrNotFound if caller has no current
// path set, or if their current course enrollment has no active checkpoint
// (completed or abandoned on a prior call — GetMyPath/SetCurrentPath have
// nothing resolvable to show in that case).
//
// When the pointer is a course enrollment, this is also where checkpoint
// completion is discovered: checkAndAdvanceCheckpoint runs against the
// enrollment before its StudentPath is resolved, so a just-finished
// checkpoint's items are never shown stale. If that call reports the course
// itself just completed, the view is still built — from the checkpoint that
// was active going into this call, now fully completed — with the
// completion flag set, rather than short-circuiting to domain.ErrNotFound;
// every subsequent call finds no active checkpoint at the guard above and
// falls back to domain.ErrNotFound as before.
func (s *StudentPathService) resolveCurrentStudentPath(ctx context.Context, caller domain.User, state domain.StudentLearningState) (domain.StudentPath, *string, *int, bool, error) {
	switch {
	case state.CurrentStandalonePathID != nil:
		sp, err := s.studentPaths.GetByID(ctx, *state.CurrentStandalonePathID)
		return sp, nil, nil, false, err

	case state.CurrentCourseEnrollmentID != nil:
		enrollment, err := s.enrollments.GetByID(ctx, *state.CurrentCourseEnrollmentID)
		if err != nil {
			return domain.StudentPath{}, nil, nil, false, err
		}
		if enrollment.ActiveCheckpointStudentPathID == nil {
			return domain.StudentPath{}, nil, nil, false, domain.ErrNotFound
		}
		triggeringCheckpointID := *enrollment.ActiveCheckpointStudentPathID
		triggeringPosition := enrollment.ActiveCheckpointPosition

		updated, _, courseCompleted, err := checkAndAdvanceCheckpoint(
			ctx, s.completion, s.studentPaths, s.courseVersions, s.paths, s.enrollments, s.state, s, s.now, enrollment,
		)
		if err != nil {
			return domain.StudentPath{}, nil, nil, false, err
		}
		if courseCompleted {
			sp, err := s.studentPaths.GetByID(ctx, triggeringCheckpointID)
			return sp, &updated.ID, triggeringPosition, true, err
		}

		sp, err := s.studentPaths.GetByID(ctx, *updated.ActiveCheckpointStudentPathID)
		return sp, &updated.ID, updated.ActiveCheckpointPosition, false, err

	default:
		return domain.StudentPath{}, nil, nil, false, domain.ErrNotFound
	}
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

	sp, courseEnrollmentID, checkpointPosition, courseCompleted, err := s.resolveCurrentStudentPath(ctx, caller, state)
	if err != nil {
		return StudentPathView{}, err
	}

	return s.composeView(ctx, caller, sp, courseEnrollmentID, checkpointPosition, courseCompleted)
}

// composeView builds the StudentPathView for sp — the shared "resolve
// items, completion state, and language locks" logic GetMyPath and
// SetCurrentPath both need.
func (s *StudentPathService) composeView(ctx context.Context, caller domain.User, sp domain.StudentPath, courseEnrollmentID *string, checkpointPosition *int, courseCompleted bool) (StudentPathView, error) {
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
		StudentPathID:            sp.ID,
		SourceTemplateID:         sp.SourceTemplateID,
		Title:                    sp.Title,
		CurrentPosition:          currentPosition,
		Items:                    viewItems,
		CourseEnrollmentID:       courseEnrollmentID,
		CourseCheckpointPosition: checkpointPosition,
		CourseCompleted:          courseCompleted,
	}, nil
}

// ArchiveStandaloneStudentPath archives the standalone StudentPath with the
// given id, owned by caller. Refused with domain.ErrNotFound if no such
// non-archived, non-course StudentPath exists for caller. If it is caller's
// current course or path, refused with domain.ErrConflict when another
// eligible course enrollment or standalone path exists — the student must
// explicitly switch to it first via SetCurrentPath — and allowed, clearing
// the current pointer, only when nothing else exists to become current.
func (s *StudentPathService) ArchiveStandaloneStudentPath(ctx context.Context, caller domain.User, studentPathID string) (domain.StudentPath, error) {
	sp, err := s.studentPaths.GetByID(ctx, studentPathID)
	if err != nil {
		return domain.StudentPath{}, err
	}
	if sp.StudentID != caller.ID || sp.ArchivedAt != nil || sp.SourceCourseEnrollmentID != nil {
		return domain.StudentPath{}, domain.ErrNotFound
	}

	state, isCurrent, err := checkCanLeaveCurrent(ctx, s.state, s.studentPaths, s.enrollments, caller.ID, sp.ID, "")
	if err != nil {
		return domain.StudentPath{}, err
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

// SetCurrentPathInput carries exactly one of CourseEnrollmentID or
// StudentPathID — the target to make current, per SetCurrentPath's
// contract.
type SetCurrentPathInput struct {
	CourseEnrollmentID *string
	StudentPathID      *string
}

// SetCurrentPath repoints caller's StudentLearningState to a different
// course enrollment or standalone path they already hold, then returns the
// same composed view GetMyPath does for whatever is now current. Does not
// touch a CourseEnrollment's own checkpoint progress — switching back later
// resumes exactly where that course's checkpoint was left. Only students
// hold a current course or path. Refused with a *domain.ValidationError
// unless exactly one of input's two fields is set, and with
// domain.ErrNotFound if the referenced course enrollment or student path
// does not exist, is not active/non-archived, or does not belong to
// caller — the same not-found-for-ownership pattern
// ArchiveStandaloneStudentPath already uses, rather than forbidden, since
// the target id space (every course enrollment or student path in the
// system) isn't something the caller should be able to distinguish
// "exists but isn't yours" from "doesn't exist" by response code alone.
func (s *StudentPathService) SetCurrentPath(ctx context.Context, caller domain.User, input SetCurrentPathInput) (StudentPathView, error) {
	if caller.Role != domain.RoleStudent {
		return StudentPathView{}, domain.ErrForbidden
	}
	if (input.CourseEnrollmentID == nil) == (input.StudentPathID == nil) {
		return StudentPathView{}, domain.NewValidationError("course_enrollment_id", "exactly one of course_enrollment_id or student_path_id must be given")
	}

	state, err := s.state.GetByStudentID(ctx, caller.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return StudentPathView{}, err
	}
	state.StudentID = caller.ID

	if input.StudentPathID != nil {
		err = s.switchToStandalonePath(ctx, caller.ID, state, *input.StudentPathID)
	} else {
		err = s.switchToCourseEnrollment(ctx, caller.ID, state, *input.CourseEnrollmentID)
	}
	if err != nil {
		return StudentPathView{}, err
	}

	return s.GetMyPath(ctx, caller)
}

// switchToStandalonePath repoints state at studentPathID, a non-archived
// standalone StudentPath that must already belong to callerID. Returns
// domain.ErrNotFound otherwise.
func (s *StudentPathService) switchToStandalonePath(ctx context.Context, callerID string, state domain.StudentLearningState, studentPathID string) error {
	sp, err := s.studentPaths.GetByID(ctx, studentPathID)
	if err != nil {
		return err
	}
	if sp.StudentID != callerID || sp.ArchivedAt != nil || sp.SourceCourseEnrollmentID != nil {
		return domain.ErrNotFound
	}
	return s.state.Upsert(ctx, state.WithCurrentStandalonePath(sp.ID))
}

// switchToCourseEnrollment repoints state at enrollmentID, an active
// CourseEnrollment that must already belong to callerID. Returns
// domain.ErrNotFound otherwise.
func (s *StudentPathService) switchToCourseEnrollment(ctx context.Context, callerID string, state domain.StudentLearningState, enrollmentID string) error {
	enrollment, err := s.enrollments.GetByID(ctx, enrollmentID)
	if err != nil {
		return err
	}
	if enrollment.StudentID != callerID || !enrollment.IsActive() {
		return domain.ErrNotFound
	}
	return s.state.Upsert(ctx, state.WithCurrentCourseEnrollment(enrollment.ID))
}
