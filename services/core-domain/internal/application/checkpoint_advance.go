package application

import (
	"context"
	"errors"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// checkAndAdvanceCheckpoint discovers, synchronously and as a side effect of
// whichever read is composing a response around enrollment, whether every
// item of its currently active checkpoint has been completed — per-item
// completion state is never pushed to core-domain, so this is read fresh
// every time rather than triggered by a stored event. When it has: if
// another checkpoint follows, that checkpoint's LearningPath template is
// copied into a new StudentPath (the same copy-on-assign flow
// CreateCourseEnrollment already uses for checkpoint 1) and the enrollment
// advances to it; if it was the last checkpoint, the enrollment is marked
// completed, its active-checkpoint pointer is cleared, and — if this
// enrollment was studentID's current pointer — StudentLearningState's
// current pointer is cleared too, exactly as leaving a course any other way
// already does. GetMyPath, SetCurrentPath, and ListMyCourseEnrollments all
// call this before building their response so a just-finished checkpoint or
// course is reflected immediately, never inferred later from a stale
// pointer or a 404.
//
// Returns the (possibly updated) enrollment, whether it advanced to a new
// checkpoint, and whether the course was just completed. A non-active
// enrollment, or one with no active checkpoint, is returned unchanged with
// both flags false — nothing to check.
func checkAndAdvanceCheckpoint(
	ctx context.Context,
	completion ports.CompletionStateReader,
	studentPaths ports.StudentPathRepository,
	courseVersions ports.CourseVersionRepository,
	paths ports.LearningPathRepository,
	enrollments ports.CourseEnrollmentRepository,
	state ports.StudentLearningStateRepository,
	studentPathSvc *StudentPathService,
	now func() time.Time,
	enrollment domain.CourseEnrollment,
) (domain.CourseEnrollment, bool, bool, error) {
	if !enrollment.IsActive() || enrollment.ActiveCheckpointStudentPathID == nil {
		return enrollment, false, false, nil
	}

	sp, err := studentPaths.GetByID(ctx, *enrollment.ActiveCheckpointStudentPathID)
	if err != nil {
		return domain.CourseEnrollment{}, false, false, err
	}

	allComplete, err := allItemsCompleted(ctx, completion, enrollment.StudentID, sp.Items)
	if err != nil {
		return domain.CourseEnrollment{}, false, false, err
	}
	if !allComplete {
		return enrollment, false, false, nil
	}

	version, err := courseVersions.GetLatestByCourseID(ctx, enrollment.CourseID)
	if err != nil {
		return domain.CourseEnrollment{}, false, false, err
	}

	nextPosition := *enrollment.ActiveCheckpointPosition + 1
	next, hasNext := checkpointAtPosition(version.Checkpoints, nextPosition)
	if !hasNext {
		return completeCourse(ctx, enrollments, state, enrollment)
	}

	template, err := paths.GetByID(ctx, next.LearningPathID)
	if err != nil {
		return domain.CourseEnrollment{}, false, false, err
	}
	newSP, err := studentPathSvc.CopyTemplateForCheckpoint(ctx, enrollment.StudentID, template, enrollment.StudentID, enrollment.ID, nextPosition)
	if err != nil {
		return domain.CourseEnrollment{}, false, false, err
	}

	if err := enrollments.AdvanceCheckpoint(ctx, enrollment.ID, newSP.ID, nextPosition); err != nil {
		return domain.CourseEnrollment{}, false, false, err
	}
	enrollment.ActiveCheckpointStudentPathID = &newSP.ID
	enrollment.ActiveCheckpointPosition = &nextPosition

	return enrollment, true, false, nil
}

// allItemsCompleted reports whether every item in items has completion
// status domain.CompletionStatusCompleted for studentID. An empty items
// slice is never considered complete — a checkpoint with nothing in it
// isn't a state this codebase otherwise produces, so treating it as
// automatically finished would be surprising rather than helpful.
func allItemsCompleted(ctx context.Context, completion ports.CompletionStateReader, studentID string, items []domain.StudentPathItemRecord) (bool, error) {
	if len(items) == 0 {
		return false, nil
	}

	nodeIDs := make([]string, len(items))
	for i, item := range items {
		nodeIDs[i] = item.ContentNodeID
	}
	raw, err := completion.GetStatuses(ctx, studentID, nodeIDs)
	if err != nil {
		return false, err
	}

	for _, item := range items {
		if raw[item.ContentNodeID] != domain.CompletionStatusCompleted {
			return false, nil
		}
	}
	return true, nil
}

// checkpointAtPosition returns the checkpoint at position within
// checkpoints, or false if none exists there — the "was that the last
// checkpoint" test checkAndAdvanceCheckpoint relies on.
func checkpointAtPosition(checkpoints []domain.CourseVersionCheckpoint, position int) (domain.CourseVersionCheckpoint, bool) {
	for _, cp := range checkpoints {
		if cp.Position == position {
			return cp, true
		}
	}
	return domain.CourseVersionCheckpoint{}, false
}

// completeCourse marks enrollment completed, clears its active-checkpoint
// pointer, and — if it was studentID's current pointer — clears
// StudentLearningState's current pointer too, the same unconditional clear
// a student reaching the end of a course gets regardless of what else they
// hold (unlike abandoning or archiving mid-course, which refuse to clear
// the pointer while an eligible alternative exists).
func completeCourse(ctx context.Context, enrollments ports.CourseEnrollmentRepository, state ports.StudentLearningStateRepository, enrollment domain.CourseEnrollment) (domain.CourseEnrollment, bool, bool, error) {
	if err := enrollments.Complete(ctx, enrollment.ID); err != nil {
		return domain.CourseEnrollment{}, false, false, err
	}
	enrollment.Status = domain.CourseEnrollmentStatusCompleted
	enrollment.ActiveCheckpointStudentPathID = nil
	enrollment.ActiveCheckpointPosition = nil

	if err := clearCurrentIfEnrollment(ctx, state, enrollment.StudentID, enrollment.ID); err != nil {
		return domain.CourseEnrollment{}, false, false, err
	}

	return enrollment, false, true, nil
}

// clearCurrentIfEnrollment clears studentID's StudentLearningState current
// pointer if — and only if — it currently points at enrollmentID. Mirrors
// AbandonCourseEnrollment/ArchiveStandaloneStudentPath's own
// state.Cleared() usage, without their "is there an eligible alternative"
// conflict check: a course reaching natural completion always clears the
// pointer, it never refuses to.
func clearCurrentIfEnrollment(ctx context.Context, state ports.StudentLearningStateRepository, studentID, enrollmentID string) error {
	s, err := state.GetByStudentID(ctx, studentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	if s.CurrentCourseEnrollmentID == nil || *s.CurrentCourseEnrollmentID != enrollmentID {
		return nil
	}
	return state.Upsert(ctx, s.Cleared())
}
