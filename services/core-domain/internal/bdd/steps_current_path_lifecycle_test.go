//go:build integration

package bdd

import (
	"context"
	"errors"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerCurrentPathLifecycleSteps(sc *godog.ScenarioContext, w *world) {
	// ── Switching current ────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" switches her current path to her "([^"]+)" enrollment$`, w.switchesCurrentPathToEnrollment)
	sc.Step(`^"([^"]+)" switches her current path back to her "([^"]+)" enrollment$`, w.switchesCurrentPathToEnrollment)
	sc.Step(`^"([^"]+)"'s current path is checkpoint (\d+) of "([^"]+)", unchanged from where she left it$`, w.currentPathIsCheckpointOfCourse)

	sc.Step(`^"([^"]+)" has "([^"]+)" assigned as a standalone path, not current$`, w.hasStandalonePathAssignedNotCurrent)
	sc.Step(`^"([^"]+)" has "([^"]+)" assigned as a standalone path$`, w.hasStandalonePathAssigned)
	sc.Step(`^"([^"]+)" has "([^"]+)" assigned as her only current path$`, w.alreadyHasAssigned)
	sc.Step(`^"([^"]+)" has a course enrollment as her current course$`, w.hasACourseEnrollmentAsCurrent)
	sc.Step(`^"([^"]+)" switches her current path to that standalone path$`, w.switchesCurrentPathToLastStandalonePath)
	sc.Step(`^"([^"]+)"'s current path is now the standalone path$`, w.currentPathIsNowTheStandalonePath)

	// ── Abandoning ───────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" has no other active enrollment or standalone path$`, func(string) error { return nil })

	// ── Archiving a standalone path ──────────────────────────────────────
	sc.Step(`^"([^"]+)" archives her standalone "([^"]+)" copy$`, w.archivesStandaloneCopy)
	sc.Step(`^that student path becomes archived$`, w.lastStudentPathBecomesArchived)
	sc.Step(`^"([^"]+)" attempts to archive her current standalone path$`, w.attemptsArchiveCurrentStandalonePath)
	sc.Step(`^"([^"]+)" archives her current standalone path$`, w.attemptsArchiveCurrentStandalonePath)
	sc.Step(`^"([^"]+)" attempts to archive her "([^"]+)" checkpoint's student path via the standalone archive action$`, w.attemptsArchiveCheckpointStudentPath)

	// ── Not found ────────────────────────────────────────────────────────
	sc.Step(`^another student "([^"]+)" has a course enrollment$`, w.anotherStudentHasACourseEnrollment)
	sc.Step(`^"([^"]+)" attempts to switch her current path to "([^"]+)"'s enrollment$`, w.attemptsSwitchToNamedStudentsEnrollment)
	sc.Step(`^"([^"]+)" attempts to switch her current path to the abandoned "([^"]+)" enrollment$`, w.attemptsSwitchToAbandonedEnrollment)

	// ── Validation failures ──────────────────────────────────────────────
	sc.Step(`^"([^"]+)" submits a switch current path request with neither course_enrollment_id nor student_path_id$`, w.submitsSwitchCurrentPathNeither)
	sc.Step(`^"([^"]+)" submits a switch current path request with both course_enrollment_id and student_path_id set$`, w.submitsSwitchCurrentPathBoth)

	// ── Authorisation failures ───────────────────────────────────────────
	sc.Step(`^an unauthenticated request attempts to switch current path$`, w.unauthSwitchCurrentPath)
}

// switchesCurrentPathToEnrollment is the real "alice switches her current
// path to her X enrollment" action — it uses the scenario's currently
// authenticated identity (w.ctx()), so wrong-caller/not-found scenarios
// built around it are exercised for real.
func (w *world) switchesCurrentPathToEnrollment(name, courseSlug string) error {
	id, ok := w.courseEnrollmentIDByKey[courseEnrollKey(name, courseSlug)]
	if !ok {
		return fmt.Errorf("no enrollment tracked for %q in %q", name, courseSlug)
	}
	eid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	resp, err := w.handler.SetCurrentPath(w.ctx(), generated.SetCurrentPathRequestObject{
		Body: &generated.SetCurrentPathRequest{CourseEnrollmentId: &eid},
	})
	w.lastResp, w.lastErr = resp, err
	w.lastCourseEnrollmentKey = courseEnrollKey(name, courseSlug)
	return err
}

// hasStandalonePathAssigned assigns pathSlug to name as a standalone path,
// under an isolated seed-teacher identity — mirrors alreadyHasAssigned
// (steps_assign_student_path_test.go), but additionally tracks the created
// StudentPath's id by (name, pathSlug), since current-path-lifecycle
// scenarios sometimes hold more than one standalone path at once and need
// to refer back to a specific one by slug rather than only "the most
// recent one" (priorStudentPathID). AssignLearningPath always sets the new
// path current unconditionally; a scenario wanting it non-current relies on
// a later Given step (another assign, or an enrollment forced current) to
// overwrite that pointer before its own action runs.
func (w *world) hasStandalonePathAssigned(name, pathSlug string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent)
	teacherName := "seed-teacher-for-" + name
	w.ensureRegistered(teacherName, domain.RoleTeacher)
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(teacherName))
	resp, err := w.handler.AssignLearningPath(ctx, generated.AssignLearningPathRequestObject{
		StudentId: studentID,
		Body:      &generated.AssignLearningPathRequest{LearningPathId: pathID(pathSlug)},
	})
	if err != nil {
		return err
	}
	created, ok := resp.(generated.AssignLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("setup: expected assignment to succeed, got %#v", resp)
	}
	w.priorStudentPathID = created.StudentPathId.String()
	w.standalonePathIDByKey[name+"|"+pathSlug] = created.StudentPathId.String()
	return nil
}

// hasStandalonePathAssignedNotCurrent assigns pathSlug to name the same way
// hasStandalonePathAssigned does, then restores whichever course/path was
// current beforehand — AssignLearningPath always sets the new path current
// unconditionally, so a scenario that wants it created but left non-current
// needs that pointer put back explicitly, rather than relying on a later
// Given step to happen to overwrite it.
func (w *world) hasStandalonePathAssignedNotCurrent(name, pathSlug string) error {
	var priorState domain.StudentLearningState
	hadPrior := false
	if studentID, ok := w.userMotifID[name]; ok {
		st, err := w.learningState.GetByStudentID(context.Background(), studentID.String())
		switch {
		case err == nil && st.HasCurrent():
			priorState, hadPrior = st, true
		case err != nil && !errors.Is(err, domain.ErrNotFound):
			return err
		}
	}

	if err := w.hasStandalonePathAssigned(name, pathSlug); err != nil {
		return err
	}

	if hadPrior {
		return w.learningState.Upsert(context.Background(), priorState)
	}
	return nil
}

// hasACourseEnrollmentAsCurrent enrolls name, isolated, in a throwaway
// course seeded just for this call, then explicitly switches current to
// it — a plain self-enrollment only sets current if nothing is already
// set, which isn't strong enough when this step runs after the student
// already has something else current.
func (w *world) hasACourseEnrollmentAsCurrent(name string) error {
	courseSlug := "throwaway-course-for-" + name
	if _, err := w.seedCourse(courseSlug, []string{"throwaway-path-for-" + name}, "seed-teacher-for-"+courseSlug, true); err != nil {
		return err
	}
	created, err := w.enrollAsIsolated(name, courseSlug)
	if err != nil {
		return err
	}
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
	resp, err := w.handler.SetCurrentPath(ctx, generated.SetCurrentPathRequestObject{
		Body: &generated.SetCurrentPathRequest{CourseEnrollmentId: &created.CourseEnrollmentId},
	})
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.SetCurrentPath200JSONResponse); !ok {
		return fmt.Errorf("setup: expected switch to succeed, got %#v", resp)
	}
	return nil
}

func (w *world) switchesCurrentPathToLastStandalonePath(name string) error {
	id, err := uuid.Parse(w.priorStudentPathID)
	if err != nil {
		return err
	}
	resp, err := w.handler.SetCurrentPath(w.ctx(), generated.SetCurrentPathRequestObject{
		Body: &generated.SetCurrentPathRequest{StudentPathId: &id},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) currentPathIsNowTheStandalonePath(string) error {
	resp, ok := w.lastResp.(generated.SetCurrentPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.CourseEnrollmentId != nil {
		return fmt.Errorf("expected a standalone path, got course_enrollment_id %v", resp.CourseEnrollmentId)
	}
	if resp.StudentPathId.String() != w.priorStudentPathID {
		return fmt.Errorf("expected current path %s, got %s", w.priorStudentPathID, resp.StudentPathId)
	}
	return nil
}

// archivesStandaloneCopy is the real "alice archives her standalone X copy"
// action — resolves the target by (name, pathSlug) via
// standalonePathIDByKey rather than priorStudentPathID, since a scenario
// using this wording always holds more than one standalone path at once.
func (w *world) archivesStandaloneCopy(name, pathSlug string) error {
	idStr, ok := w.standalonePathIDByKey[name+"|"+pathSlug]
	if !ok {
		return fmt.Errorf("no standalone path tracked for %q under slug %q", name, pathSlug)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return err
	}
	resp, err := w.handler.ArchiveStandaloneStudentPath(w.ctx(), generated.ArchiveStandaloneStudentPathRequestObject{StudentPathId: id})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) lastStudentPathBecomesArchived() error {
	resp, ok := w.lastResp.(generated.ArchiveStandaloneStudentPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.ArchivedAt == nil {
		return fmt.Errorf("expected the student path to carry an archived_at timestamp")
	}
	return nil
}

// attemptsArchiveCurrentStandalonePath attempts to archive whichever
// standalone path was most recently assigned (priorStudentPathID) — used
// by scenarios where that path is still current at the time of the attempt.
func (w *world) attemptsArchiveCurrentStandalonePath(string) error {
	id, err := uuid.Parse(w.priorStudentPathID)
	if err != nil {
		return err
	}
	resp, err := w.handler.ArchiveStandaloneStudentPath(w.ctx(), generated.ArchiveStandaloneStudentPathRequestObject{StudentPathId: id})
	w.lastResp, w.lastErr = resp, err
	return err
}

// attemptsArchiveCheckpointStudentPath attempts to archive the StudentPath
// backing name's active checkpoint in courseSlug through the standalone
// archive action — expected to be refused not-found, since a checkpoint's
// StudentPath is never a standalone one.
func (w *world) attemptsArchiveCheckpointStudentPath(name, courseSlug string) error {
	enrollmentIDStr, ok := w.courseEnrollmentIDByKey[courseEnrollKey(name, courseSlug)]
	if !ok {
		return fmt.Errorf("no enrollment tracked for %q in %q", name, courseSlug)
	}
	enrollment, err := w.courseEnrollments.GetByID(context.Background(), enrollmentIDStr)
	if err != nil {
		return err
	}
	if enrollment.ActiveCheckpointStudentPathID == nil {
		return fmt.Errorf("enrollment %s has no active checkpoint student path", enrollmentIDStr)
	}
	id, err := uuid.Parse(*enrollment.ActiveCheckpointStudentPathID)
	if err != nil {
		return err
	}
	resp, err := w.handler.ArchiveStandaloneStudentPath(w.ctx(), generated.ArchiveStandaloneStudentPathRequestObject{StudentPathId: id})
	w.lastResp, w.lastErr = resp, err
	return err
}

// anotherStudentHasACourseEnrollment seeds a throwaway course and enrolls
// carol in it, isolated — the target enrollment "belongs to someone else"
// scenarios attempt to switch to.
func (w *world) anotherStudentHasACourseEnrollment(name string) error {
	courseSlug := "throwaway-course-for-" + name
	if _, err := w.seedCourse(courseSlug, []string{"throwaway-path-for-" + name}, "seed-teacher-for-"+courseSlug, true); err != nil {
		return err
	}
	_, err := w.enrollAsIsolated(name, courseSlug)
	return err
}

func (w *world) attemptsSwitchToNamedStudentsEnrollment(callerName, targetName string) error {
	id, ok := w.courseEnrollmentIDByStudent[targetName]
	if !ok {
		return fmt.Errorf("no enrollment tracked for %q", targetName)
	}
	eid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	resp, err := w.handler.SetCurrentPath(w.ctx(), generated.SetCurrentPathRequestObject{
		Body: &generated.SetCurrentPathRequest{CourseEnrollmentId: &eid},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

// attemptsSwitchToAbandonedEnrollment targets priorCourseEnrollmentID —
// set by enrolledThenAbandoned (steps_course_enrollment_test.go) for the
// enrollment name abandoned earlier in the same scenario.
func (w *world) attemptsSwitchToAbandonedEnrollment(name, courseSlug string) error {
	id, err := uuid.Parse(w.priorCourseEnrollmentID)
	if err != nil {
		return err
	}
	resp, err := w.handler.SetCurrentPath(w.ctx(), generated.SetCurrentPathRequestObject{
		Body: &generated.SetCurrentPathRequest{CourseEnrollmentId: &id},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsSwitchCurrentPathNeither(string) error {
	resp, err := w.handler.SetCurrentPath(w.ctx(), generated.SetCurrentPathRequestObject{
		Body: &generated.SetCurrentPathRequest{},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsSwitchCurrentPathBoth(name string) error {
	enrollmentIDStr, ok := w.courseEnrollmentIDByStudent[name]
	if !ok {
		return fmt.Errorf("no enrollment tracked for %q", name)
	}
	eid, err := uuid.Parse(enrollmentIDStr)
	if err != nil {
		return err
	}
	if w.priorStudentPathID == "" {
		return fmt.Errorf("no standalone path tracked for %q", name)
	}
	spID, err := uuid.Parse(w.priorStudentPathID)
	if err != nil {
		return err
	}
	resp, err := w.handler.SetCurrentPath(w.ctx(), generated.SetCurrentPathRequestObject{
		Body: &generated.SetCurrentPathRequest{CourseEnrollmentId: &eid, StudentPathId: &spID},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthSwitchCurrentPath() error {
	w.noAuthToken() //nolint:errcheck // never errors
	resp, err := w.handler.SetCurrentPath(w.ctx(), generated.SetCurrentPathRequestObject{
		Body: &generated.SetCurrentPathRequest{},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}
