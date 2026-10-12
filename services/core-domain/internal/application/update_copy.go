package application

import (
	"context"
	"errors"
	"sort"

	"github.com/motifpath/core-domain/internal/domain"
)

// UpdateAvailable reports, keyed by StudentPath id, whether each of paths
// can be updated to its learning path's latest content: a standalone copy
// that is not archived and not finished, whose learning path is still
// published and would now give a different copy. A course part's copy is
// never offered an update on its own; its enrollment is updated instead.
// Every path in paths belongs to studentID.
func (s *StudentPathService) UpdateAvailable(ctx context.Context, studentID string, paths []domain.StudentPath) (map[string]bool, error) {
	available := make(map[string]bool, len(paths))
	var candidates []domain.StudentPath
	for _, sp := range paths {
		available[sp.ID] = false
		if sp.SourceCourseEnrollmentID == nil && sp.ArchivedAt == nil {
			candidates = append(candidates, sp)
		}
	}
	if len(candidates) == 0 {
		return available, nil
	}

	completed, err := s.CompletedCounts(ctx, studentID, candidates)
	if err != nil {
		return nil, err
	}
	templates := map[string]*domain.LearningPath{}
	for _, sp := range candidates {
		if isFinished(sp, completed[sp.ID]) {
			continue
		}
		template, err := s.publishedTemplate(ctx, templates, sp.SourceTemplateID)
		if err != nil {
			return nil, err
		}
		available[sp.ID] = template != nil && sp.DiffersFromTemplate(*template)
	}
	return available, nil
}

// UpdateStandalonePathToLatest replaces caller's standalone copy
// studentPathID with its learning path's current lessons and presentation,
// keeping the copy's id, assigner and whether it is current. Progress is
// kept per content node, so finished lessons stay finished. A copy with
// nothing to update is returned as it is. Refused with domain.ErrNotFound
// unless studentPathID is caller's own non-archived standalone copy, and
// with domain.ErrConflict when every lesson is already completed or the
// learning path is no longer published.
func (s *StudentPathService) UpdateStandalonePathToLatest(ctx context.Context, caller domain.User, studentPathID string) (domain.StudentPath, error) {
	sp, err := s.studentPaths.GetByID(ctx, studentPathID)
	if err != nil {
		return domain.StudentPath{}, err
	}
	if sp.StudentID != caller.ID || sp.ArchivedAt != nil || sp.SourceCourseEnrollmentID != nil {
		return domain.StudentPath{}, domain.ErrNotFound
	}

	completed, err := s.CompletedCounts(ctx, caller.ID, []domain.StudentPath{sp})
	if err != nil {
		return domain.StudentPath{}, err
	}
	if isFinished(sp, completed[sp.ID]) {
		return domain.StudentPath{}, domain.ErrConflict
	}
	template, err := s.publishedTemplate(ctx, map[string]*domain.LearningPath{}, sp.SourceTemplateID)
	if err != nil {
		return domain.StudentPath{}, err
	}
	if template == nil {
		return domain.StudentPath{}, domain.ErrConflict
	}
	if !sp.DiffersFromTemplate(*template) {
		return sp, nil
	}
	return s.refreshCopy(ctx, sp, *template)
}

// refreshCopy rebuilds sp from template's current items, each pinned to its
// content node's latest published version, and stores the result.
func (s *StudentPathService) refreshCopy(ctx context.Context, sp domain.StudentPath, template domain.LearningPath) (domain.StudentPath, error) {
	versionIDs, err := s.resolveLatestVersionIDs(ctx, template.Items)
	if err != nil {
		return domain.StudentPath{}, err
	}
	refreshed, err := sp.RefreshedFrom(template, versionIDs)
	if err != nil {
		return domain.StudentPath{}, err
	}
	if err := s.studentPaths.ReplaceContent(ctx, refreshed); err != nil {
		return domain.StudentPath{}, err
	}
	return refreshed, nil
}

// publishedTemplate returns the learning path with id, or nil when it no longer
// exists or is not published, caching each lookup in templates.
func (s *StudentPathService) publishedTemplate(ctx context.Context, templates map[string]*domain.LearningPath, id string) (*domain.LearningPath, error) {
	if template, ok := templates[id]; ok {
		return template, nil
	}
	template, err := s.paths.GetByID(ctx, id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		templates[id] = nil
		return nil, nil
	case err != nil:
		return nil, err
	case template.Status != domain.LearningPathStatusPublished:
		templates[id] = nil
		return nil, nil
	}
	templates[id] = &template
	return &template, nil
}

// isFinished reports whether every lesson of sp is completed.
func isFinished(sp domain.StudentPath, completed int) bool {
	return len(sp.Items) > 0 && completed == len(sp.Items)
}

// UpdateAvailable reports, keyed by CourseEnrollment id, whether each of
// enrollments can be updated to its course's latest content: an active
// enrollment whose course has a newer published version, or whose active
// part's copy differs from that part's learning path.
func (s *CourseEnrollmentService) UpdateAvailable(ctx context.Context, enrollments []domain.CourseEnrollment) (map[string]bool, error) {
	available := make(map[string]bool, len(enrollments))
	templates := map[string]*domain.LearningPath{}
	for _, e := range enrollments {
		behind, _, err := s.behind(ctx, e, templates)
		if err != nil {
			return nil, err
		}
		available[e.ID] = behind
	}
	return available, nil
}

// UpdateCourseEnrollmentToLatest moves caller's active enrollment
// enrollmentID to its course's latest published version and that version's
// title and thumbnail. If the active part's learning path is still in the
// new version, the learner stays on it at its new position and its copy
// takes the path's current lessons. Otherwise that copy is archived and the
// learner continues at the first part of the new version they haven't
// finished; with none left, the enrollment completes. Parts before the
// active one count as passed. Progress is kept per content node, and which
// course or path is current never changes. An enrollment with nothing to
// update is returned as it is. Refused with domain.ErrNotFound unless
// enrollmentID is caller's own active enrollment.
func (s *CourseEnrollmentService) UpdateCourseEnrollmentToLatest(ctx context.Context, caller domain.User, enrollmentID string) (domain.CourseEnrollment, error) {
	enrollment, err := s.enrollments.GetByID(ctx, enrollmentID)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}
	if enrollment.StudentID != caller.ID || !enrollment.IsActive() {
		return domain.CourseEnrollment{}, domain.ErrNotFound
	}

	// A part finished since the last read advances first, so the update
	// starts from where the learner really is.
	enrollment, _, _, err = checkAndAdvanceCheckpoint(
		ctx, s.completion, s.studentPaths, s.courseVersions, s.paths, s.enrollments, s.state, s.studentPathSvc, s.now, enrollment,
	)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}

	behind, latest, err := s.behind(ctx, enrollment, map[string]*domain.LearningPath{})
	if err != nil || !behind {
		return enrollment, err
	}

	if err := s.enrollments.Repin(ctx, enrollment.ID, latest.VersionNumber, latest.TitleSnapshot, latest.ThumbnailURLSnapshot); err != nil {
		return domain.CourseEnrollment{}, err
	}
	enrollment.CourseVersionNumber = latest.VersionNumber
	enrollment.CourseTitle = latest.TitleSnapshot
	enrollment.CourseThumbnailURL = latest.ThumbnailURLSnapshot

	return s.moveActivePart(ctx, enrollment, latest)
}

// behind reports whether enrollment can be updated, and returns its
// course's latest published version. An enrollment that isn't active, or a
// course with no published version, is never behind.
func (s *CourseEnrollmentService) behind(ctx context.Context, enrollment domain.CourseEnrollment, templates map[string]*domain.LearningPath) (bool, domain.CourseVersion, error) {
	if !enrollment.IsActive() || enrollment.ActiveCheckpointStudentPathID == nil {
		return false, domain.CourseVersion{}, nil
	}
	latest, err := s.courseVersions.GetLatestByCourseID(ctx, enrollment.CourseID)
	if errors.Is(err, domain.ErrNotFound) {
		return false, domain.CourseVersion{}, nil
	}
	if err != nil {
		return false, domain.CourseVersion{}, err
	}
	if latest.VersionNumber > enrollment.CourseVersionNumber {
		return true, latest, nil
	}

	part, err := s.studentPaths.GetByID(ctx, *enrollment.ActiveCheckpointStudentPathID)
	if err != nil {
		return false, domain.CourseVersion{}, err
	}
	template, err := s.studentPathSvc.publishedTemplate(ctx, templates, part.SourceTemplateID)
	if err != nil {
		return false, domain.CourseVersion{}, err
	}
	return template != nil && part.DiffersFromTemplate(*template), latest, nil
}

// moveActivePart puts enrollment's active part where it belongs in latest:
// the same copy at its new position, refreshed from its learning path, or —
// when its path is no longer a checkpoint — a copy of the first part the
// learner hasn't finished, completing the course when there is none.
func (s *CourseEnrollmentService) moveActivePart(ctx context.Context, enrollment domain.CourseEnrollment, latest domain.CourseVersion) (domain.CourseEnrollment, error) {
	part, err := s.studentPaths.GetByID(ctx, *enrollment.ActiveCheckpointStudentPathID)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}

	checkpoints := append([]domain.CourseVersionCheckpoint(nil), latest.Checkpoints...)
	sort.Slice(checkpoints, func(i, j int) bool { return checkpoints[i].Position < checkpoints[j].Position })

	for _, cp := range checkpoints {
		if cp.LearningPathID == part.SourceTemplateID {
			return s.stayOnPart(ctx, enrollment, part, cp)
		}
	}

	if err := s.studentPaths.Archive(ctx, part.ID, s.now()); err != nil {
		return domain.CourseEnrollment{}, err
	}
	return s.continueAtFirstUnfinished(ctx, enrollment, checkpoints)
}

// stayOnPart keeps part as enrollment's active part at cp's position,
// refreshed from cp's learning path.
func (s *CourseEnrollmentService) stayOnPart(ctx context.Context, enrollment domain.CourseEnrollment, part domain.StudentPath, cp domain.CourseVersionCheckpoint) (domain.CourseEnrollment, error) {
	template, err := s.paths.GetByID(ctx, cp.LearningPathID)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}
	position := cp.Position
	part.CourseCheckpointPosition = &position
	if _, err := s.studentPathSvc.refreshCopy(ctx, part, template); err != nil {
		return domain.CourseEnrollment{}, err
	}
	return s.setActivePart(ctx, enrollment, part.ID, position)
}

// continueAtFirstUnfinished makes a copy of the first of checkpoints whose
// lessons the learner hasn't all completed enrollment's active part, or
// completes the course when every one is finished.
func (s *CourseEnrollmentService) continueAtFirstUnfinished(ctx context.Context, enrollment domain.CourseEnrollment, checkpoints []domain.CourseVersionCheckpoint) (domain.CourseEnrollment, error) {
	for _, cp := range checkpoints {
		template, err := s.paths.GetByID(ctx, cp.LearningPathID)
		if err != nil {
			return domain.CourseEnrollment{}, err
		}
		finished, err := allItemsCompleted(ctx, s.completion, enrollment.StudentID, templateItemRecords(template))
		if err != nil {
			return domain.CourseEnrollment{}, err
		}
		if finished {
			continue
		}
		next, err := s.studentPathSvc.CopyTemplateForCheckpoint(ctx, enrollment.StudentID, template, enrollment.StudentID, enrollment.ID, cp.Position)
		if err != nil {
			return domain.CourseEnrollment{}, err
		}
		return s.setActivePart(ctx, enrollment, next.ID, cp.Position)
	}

	enrollment, _, _, err := completeCourse(ctx, s.enrollments, s.state, enrollment)
	return enrollment, err
}

func (s *CourseEnrollmentService) setActivePart(ctx context.Context, enrollment domain.CourseEnrollment, studentPathID string, position int) (domain.CourseEnrollment, error) {
	if err := s.enrollments.AdvanceCheckpoint(ctx, enrollment.ID, studentPathID, position); err != nil {
		return domain.CourseEnrollment{}, err
	}
	enrollment.ActiveCheckpointStudentPathID = &studentPathID
	enrollment.ActiveCheckpointPosition = &position
	return enrollment, nil
}

// templateItemRecords lists template's lessons in the shape
// allItemsCompleted reads.
func templateItemRecords(template domain.LearningPath) []domain.StudentPathItemRecord {
	records := make([]domain.StudentPathItemRecord, len(template.Items))
	for i, item := range template.Items {
		records[i] = domain.StudentPathItemRecord{Position: item.Position, ContentNodeID: item.ContentNodeID}
	}
	return records
}
