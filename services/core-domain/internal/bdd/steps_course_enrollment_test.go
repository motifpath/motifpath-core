//go:build integration

package bdd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerCourseEnrollmentSteps(sc *godog.ScenarioContext, w *world) {
	// ── Seeding an existing enrollment ──────────────────────────────────
	sc.Step(`^"([^"]+)" is enrolled in "([^"]+)" as her current course$`, w.studentEnrolledInCourseSetup)
	sc.Step(`^"([^"]+)" is enrolled in "([^"]+)"$`, w.studentEnrolledInCourseSetup)
	sc.Step(`^"([^"]+)" is already enrolled in "([^"]+)" as her current course$`, w.studentEnrolledInCourseSetup)
	sc.Step(`^"([^"]+)" is already enrolled in "([^"]+)" as her only current course$`, w.studentEnrolledInCourseSetup)
	sc.Step(`^"([^"]+)" is enrolled in "([^"]+)" as her only current course$`, w.studentEnrolledInCourseSetup)
	sc.Step(`^"([^"]+)" is enrolled in "([^"]+)" with checkpoint 1 active$`, w.enrolledWithCheckpoint1Active)
	sc.Step(`^"([^"]+)" is enrolled in "([^"]+)" with checkpoint 2 active as her current path$`, w.enrolledWithCheckpoint2ActiveAsCurrent)
	sc.Step(`^"([^"]+)" is enrolled in "([^"]+)" with an active checkpoint$`, w.enrolledWithActiveCheckpoint)
	sc.Step(`^"([^"]+)" enrolled in "([^"]+)" and then abandoned it$`, w.enrolledThenAbandoned)
	sc.Step(`^checkpoint (\d+) is the last checkpoint of "([^"]+)"$`, func(int, string) error { return nil })
	sc.Step(`^"([^"]+)"'s latest published version is closed to new enrollments$`, w.courseVersionClosedToEnrollments)
	sc.Step(`^"([^"]+)" has no current course or path$`, w.hasNoCurrentCourseOrPath)

	// ── Self-enrolling ───────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" enrolls in course "([^"]+)"$`, w.enrollsInCourse)
	sc.Step(`^"([^"]+)" attempts to enroll in course "([^"]+)" again$`, w.attemptsEnrollInCourse)
	sc.Step(`^"([^"]+)" attempts to enroll in course "([^"]+)"$`, w.attemptsEnrollInCourse)
	sc.Step(`^"([^"]+)" attempts to enroll in a course ID that does not exist$`, w.attemptsEnrollMissingCourse)
	sc.Step(`^an unauthenticated request attempts to enroll in a course$`, w.unauthEnrolls)

	sc.Step(`^an enrollment is created and returned, pinned to the course's latest published version$`, w.enrollmentPinnedToLatestVersion)
	sc.Step(`^checkpoint (\d+)'s student path is created and becomes the enrollment's active checkpoint$`, w.checkpointCreatedAndActive)
	sc.Step(`^the enrollment status is "([^"]+)"$`, w.enrollmentStatusIsOrBecomes)
	sc.Step(`^the enrollment status becomes "([^"]+)"$`, w.enrollmentStatusIsOrBecomes)
	sc.Step(`^"([^"]+)"'s current path is now checkpoint (\d+) of "([^"]+)"$`, w.currentPathIsCheckpointOfCourse)
	sc.Step(`^a new enrollment for "([^"]+)" is created$`, w.newEnrollmentForCourseIsCreated)
	sc.Step(`^"([^"]+)"'s current course remains "([^"]+)"$`, w.currentCourseRemains)
	sc.Step(`^a fresh enrollment is created, starting again at checkpoint 1$`, w.freshEnrollmentStartingAtCheckpoint1)

	// ── Completing checkpoints ───────────────────────────────────────────
	sc.Step(`^"([^"]+)" completes every item in checkpoint (\d+)'s student path$`, w.completesCheckpointItems)
	sc.Step(`^checkpoint (\d+)'s student path is created$`, w.checkpointStudentPathIsCreated)
	sc.Step(`^the enrollment's active checkpoint becomes checkpoint (\d+)$`, w.enrollmentActiveCheckpointBecomes)
	sc.Step(`^the response shows checkpoint (\d+)'s items, all completed, with course_completed true$`, w.responseShowsCheckpointItemsAllCompletedWithCourseCompleted)
	sc.Step(`^"([^"]+)" retrieves her current path again$`, w.retrievesCurrentPath)

	// ── Course enrollment listing / congrats page ───────────────────────
	sc.Step(`^"([^"]+)" lists her course enrollments$`, w.listsCourseEnrollments)
	sc.Step(`^the response includes the active "([^"]+)" enrollment with its current checkpoint and progress$`, w.responseIncludesActiveEnrollmentWithProgress)
	sc.Step(`^the response includes no other active enrollment$`, w.responseIncludesNoOtherActiveEnrollment)
	sc.Step(`^the response includes both the completed "([^"]+)" enrollment and the active "([^"]+)" enrollment$`, w.responseIncludesCompletedAndActive)

	// ── Abandoning ───────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" abandons her "([^"]+)" enrollment$`, w.abandonsEnrollment)
	sc.Step(`^"([^"]+)" attempts to abandon her "([^"]+)" enrollment$`, w.attemptsAbandonEnrollment)
}

func courseEnrollKey(name, courseSlug string) string { return name + "|" + courseSlug }

// trackEnrollment records enrollmentID under both the (student, course)
// composite key — for steps that name both — and the student-only key —
// for steps that name only whose enrollment it is (e.g. "carol"'s
// enrollment) — and marks it as the enrollment a following Then step whose
// own wording names neither should resolve.
func (w *world) trackEnrollment(name, courseSlug, enrollmentID string) {
	key := courseEnrollKey(name, courseSlug)
	w.courseEnrollmentIDByKey[key] = enrollmentID
	w.courseEnrollmentIDByStudent[name] = enrollmentID
	w.lastCourseEnrollmentKey = key
}

// enrollAsIsolated performs a real self-enrollment for name in courseSlug
// under an isolated context (ensureRegistered + a Clerk sub scoped to
// name), independent of whichever identity the scenario itself is
// currently authenticated as — the same isolation
// alreadyHasAssigned/seedCourseWithCheckpoint already use elsewhere. Used
// by every Given step that needs a student already enrolled before the
// scenario's own authentication and action steps run. Always expects
// success; any other outcome is a setup bug, not a scenario under test.
func (w *world) enrollAsIsolated(name, courseSlug string) (generated.CreateCourseEnrollment201JSONResponse, error) {
	w.ensureRegistered(name, domain.RoleStudent)
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
	resp, err := w.handler.CreateCourseEnrollment(ctx, generated.CreateCourseEnrollmentRequestObject{
		Body: &generated.CreateCourseEnrollmentRequest{CourseId: w.courseIDBySlug[courseSlug]},
	})
	if err != nil {
		return generated.CreateCourseEnrollment201JSONResponse{}, err
	}
	created, ok := resp.(generated.CreateCourseEnrollment201JSONResponse)
	if !ok {
		return generated.CreateCourseEnrollment201JSONResponse{}, fmt.Errorf("setup: expected enrollment to succeed, got %#v", resp)
	}
	w.trackEnrollment(name, courseSlug, created.CourseEnrollmentId.String())
	w.lastResp, w.lastErr = resp, nil
	return created, nil
}

func (w *world) studentEnrolledInCourseSetup(name, courseSlug string) error {
	_, err := w.enrollAsIsolated(name, courseSlug)
	return err
}

// enrollAndAdvanceToCheckpoint enrolls name in courseSlug (isolated
// context) and, if targetPosition > 1, marks every item of each
// intervening checkpoint complete and triggers the synchronous advance
// check (ListMyCourseEnrollments, which advances every active enrollment
// regardless of whether it's the student's current one — see
// CourseEnrollmentService.ListMyCourseEnrollments) until the enrollment's
// active checkpoint reaches targetPosition.
func (w *world) enrollAndAdvanceToCheckpoint(name, courseSlug string, targetPosition int) error {
	created, err := w.enrollAsIsolated(name, courseSlug)
	if err != nil {
		return err
	}
	studentID := w.userMotifID[name]
	enrollmentID := created.CourseEnrollmentId.String()
	for {
		e, err := w.courseEnrollments.GetByID(context.Background(), enrollmentID)
		if err != nil {
			return err
		}
		if e.ActiveCheckpointPosition == nil {
			return fmt.Errorf("enrollment for %q has no active checkpoint before reaching position %d", name, targetPosition)
		}
		if *e.ActiveCheckpointPosition >= targetPosition {
			return nil
		}
		sp, err := w.studentPaths.GetByID(context.Background(), *e.ActiveCheckpointStudentPathID)
		if err != nil {
			return err
		}
		for _, item := range sp.Items {
			w.completion.set(studentID.String(), item.ContentNodeID, domain.CompletionStatusCompleted)
		}
		ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
		if _, err := w.handler.ListMyCourseEnrollments(ctx, generated.ListMyCourseEnrollmentsRequestObject{}); err != nil {
			return err
		}
	}
}

func (w *world) enrolledWithCheckpoint1Active(name, courseSlug string) error {
	return w.enrollAndAdvanceToCheckpoint(name, courseSlug, 1)
}

func (w *world) enrolledWithCheckpoint2ActiveAsCurrent(name, courseSlug string) error {
	return w.enrollAndAdvanceToCheckpoint(name, courseSlug, 2)
}

func (w *world) enrolledWithActiveCheckpoint(name, courseSlug string) error {
	return w.enrollAndAdvanceToCheckpoint(name, courseSlug, 1)
}

// enrolledThenAbandoned enrolls name in courseSlug and immediately abandons
// that enrollment, both under an isolated context — the precondition
// behind both "may re-enroll after abandoning" and "switching to an
// abandoned enrollment returns not found".
func (w *world) enrolledThenAbandoned(name, courseSlug string) error {
	created, err := w.enrollAsIsolated(name, courseSlug)
	if err != nil {
		return err
	}
	w.priorCourseEnrollmentID = created.CourseEnrollmentId.String()
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
	resp, err := w.handler.AbandonCourseEnrollment(ctx, generated.AbandonCourseEnrollmentRequestObject{CourseEnrollmentId: created.CourseEnrollmentId})
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.AbandonCourseEnrollment200JSONResponse); !ok {
		return fmt.Errorf("setup: expected abandon to succeed, got %#v", resp)
	}
	return nil
}

// courseVersionClosedToEnrollments publishes a new CourseVersion for
// courseSlug identical to the current latest except
// AvailableForNewEnrollments: false — the fake CourseVersionRepository has
// no in-place update, so a strictly newer version is the way to make it
// the new "latest" GetLatestByCourseID resolves.
func (w *world) courseVersionClosedToEnrollments(courseSlug string) error {
	courseID, ok := w.courseIDBySlug[courseSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", courseSlug)
	}
	latest, err := w.courseVersions.GetLatestByCourseID(context.Background(), courseID.String())
	if err != nil {
		return err
	}
	closed := latest
	closed.VersionNumber = latest.VersionNumber + 1
	closed.AvailableForNewEnrollments = false
	return w.courseVersions.Create(context.Background(), closed)
}

func (w *world) hasNoCurrentCourseOrPath(name string) error {
	studentID, ok := w.userMotifID[name]
	if !ok {
		return nil
	}
	state, err := w.learningState.GetByStudentID(context.Background(), studentID.String())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	if state.HasCurrent() {
		return fmt.Errorf("expected %q to have no current course or path, got %+v", name, state)
	}
	return nil
}

// enrollsInCourse is the real "alice enrolls in course X" action step — it
// uses the scenario's currently authenticated identity (w.ctx()) rather
// than an isolated one, so unauthenticated/wrong-role/not-found/conflict
// scenarios are exercised for real.
func (w *world) enrollsInCourse(name, courseSlug string) error {
	resp, err := w.handler.CreateCourseEnrollment(w.ctx(), generated.CreateCourseEnrollmentRequestObject{
		Body: &generated.CreateCourseEnrollmentRequest{CourseId: w.courseIDBySlug[courseSlug]},
	})
	w.lastResp, w.lastErr = resp, err
	if created, ok := resp.(generated.CreateCourseEnrollment201JSONResponse); ok {
		w.trackEnrollment(name, courseSlug, created.CourseEnrollmentId.String())
	}
	return err
}

func (w *world) attemptsEnrollInCourse(name, courseSlug string) error {
	return w.enrollsInCourse(name, courseSlug)
}

func (w *world) attemptsEnrollMissingCourse(string) error {
	resp, err := w.handler.CreateCourseEnrollment(w.ctx(), generated.CreateCourseEnrollmentRequestObject{
		Body: &generated.CreateCourseEnrollmentRequest{CourseId: deterministicUUID("course", "does-not-exist")},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthEnrolls() error {
	w.noAuthToken() //nolint:errcheck // never errors
	resp, err := w.handler.CreateCourseEnrollment(w.ctx(), generated.CreateCourseEnrollmentRequestObject{
		Body: &generated.CreateCourseEnrollmentRequest{CourseId: deterministicUUID("course", "unauthenticated-request-course")},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) enrollmentPinnedToLatestVersion() error {
	resp, ok := w.lastResp.(generated.CreateCourseEnrollment201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	latest, err := w.courseVersions.GetLatestByCourseID(context.Background(), resp.CourseId.String())
	if err != nil {
		return err
	}
	if resp.CourseVersionNumber != latest.VersionNumber {
		return fmt.Errorf("expected enrollment pinned to version %d, got %d", latest.VersionNumber, resp.CourseVersionNumber)
	}
	return nil
}

// resolveEnrollmentForAssertion resolves the CourseEnrollment a following
// Then step should check: from w.lastResp directly when the most recent
// handler call was enrollment-shaped, otherwise from
// w.lastCourseEnrollmentKey — set by whichever action (self-enroll,
// complete-checkpoint, abandon) most recently touched an enrollment.
func (w *world) resolveEnrollmentForAssertion() (domain.CourseEnrollment, error) {
	switch resp := w.lastResp.(type) {
	case generated.CreateCourseEnrollment201JSONResponse:
		return w.courseEnrollments.GetByID(context.Background(), resp.CourseEnrollmentId.String())
	case generated.AbandonCourseEnrollment200JSONResponse:
		return w.courseEnrollments.GetByID(context.Background(), resp.CourseEnrollmentId.String())
	}
	if w.lastCourseEnrollmentKey == "" {
		return domain.CourseEnrollment{}, fmt.Errorf("no enrollment tracked to assert against")
	}
	id, ok := w.courseEnrollmentIDByKey[w.lastCourseEnrollmentKey]
	if !ok {
		return domain.CourseEnrollment{}, fmt.Errorf("no enrollment tracked under key %q", w.lastCourseEnrollmentKey)
	}
	return w.courseEnrollments.GetByID(context.Background(), id)
}

func (w *world) enrollmentStatusIsOrBecomes(status string) error {
	e, err := w.resolveEnrollmentForAssertion()
	if err != nil {
		return err
	}
	if string(e.Status) != status {
		return fmt.Errorf("expected enrollment status %q, got %q", status, e.Status)
	}
	return nil
}

func (w *world) checkpointStudentPathIsCreated(position int) error {
	if err := w.triggerCheckpointDiscoveryForLastEnrollment(); err != nil {
		return err
	}
	e, err := w.resolveEnrollmentForAssertion()
	if err != nil {
		return err
	}
	if e.ActiveCheckpointStudentPathID == nil {
		return fmt.Errorf("expected an active checkpoint student path")
	}
	sp, err := w.studentPaths.GetByID(context.Background(), *e.ActiveCheckpointStudentPathID)
	if err != nil {
		return err
	}
	if sp.SourceCourseEnrollmentID == nil || *sp.SourceCourseEnrollmentID != e.ID {
		return fmt.Errorf("expected the active checkpoint's student path to belong to enrollment %s", e.ID)
	}
	if sp.CourseCheckpointPosition == nil || *sp.CourseCheckpointPosition != position {
		return fmt.Errorf("expected checkpoint position %d, got %+v", position, sp.CourseCheckpointPosition)
	}
	return nil
}

func (w *world) enrollmentActiveCheckpointBecomes(position int) error {
	e, err := w.resolveEnrollmentForAssertion()
	if err != nil {
		return err
	}
	if e.ActiveCheckpointPosition == nil || *e.ActiveCheckpointPosition != position {
		return fmt.Errorf("expected active checkpoint position %d, got %+v", position, e.ActiveCheckpointPosition)
	}
	return nil
}

func (w *world) checkpointCreatedAndActive(position int) error {
	if err := w.checkpointStudentPathIsCreated(position); err != nil {
		return err
	}
	return w.enrollmentActiveCheckpointBecomes(position)
}

func (w *world) currentPathIsCheckpointOfCourse(name string, position int, courseSlug string) error {
	studentID := w.userMotifID[name]
	state, err := w.learningState.GetByStudentID(context.Background(), studentID.String())
	if err != nil {
		return err
	}
	if state.CurrentCourseEnrollmentID == nil {
		return fmt.Errorf("expected %q to have a current course enrollment", name)
	}
	e, err := w.courseEnrollments.GetByID(context.Background(), *state.CurrentCourseEnrollmentID)
	if err != nil {
		return err
	}
	wantCourseID, ok := w.courseIDBySlug[courseSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", courseSlug)
	}
	if e.CourseID != wantCourseID.String() {
		return fmt.Errorf("expected %q's current course to be %q, got course id %s", name, courseSlug, e.CourseID)
	}
	if e.ActiveCheckpointPosition == nil || *e.ActiveCheckpointPosition != position {
		return fmt.Errorf("expected %q's current checkpoint position %d, got %+v", name, position, e.ActiveCheckpointPosition)
	}
	return nil
}

func (w *world) newEnrollmentForCourseIsCreated(courseSlug string) error {
	resp, ok := w.lastResp.(generated.CreateCourseEnrollment201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	wantCourseID, ok := w.courseIDBySlug[courseSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", courseSlug)
	}
	if resp.CourseId != wantCourseID {
		return fmt.Errorf("expected a new enrollment for %q, got course id %s", courseSlug, resp.CourseId)
	}
	return nil
}

func (w *world) currentCourseRemains(name, courseSlug string) error {
	studentID := w.userMotifID[name]
	state, err := w.learningState.GetByStudentID(context.Background(), studentID.String())
	if err != nil {
		return err
	}
	if state.CurrentCourseEnrollmentID == nil {
		return fmt.Errorf("expected %q to still have a current course enrollment", name)
	}
	e, err := w.courseEnrollments.GetByID(context.Background(), *state.CurrentCourseEnrollmentID)
	if err != nil {
		return err
	}
	wantCourseID, ok := w.courseIDBySlug[courseSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", courseSlug)
	}
	if e.CourseID != wantCourseID.String() {
		return fmt.Errorf("expected %q's current course to remain %q, got course id %s", name, courseSlug, e.CourseID)
	}
	return nil
}

func (w *world) freshEnrollmentStartingAtCheckpoint1() error {
	resp, ok := w.lastResp.(generated.CreateCourseEnrollment201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.ActiveCheckpointPosition == nil || *resp.ActiveCheckpointPosition != 1 {
		return fmt.Errorf("expected the fresh enrollment to start at checkpoint 1, got %+v", resp.ActiveCheckpointPosition)
	}
	if w.priorCourseEnrollmentID != "" && resp.CourseEnrollmentId.String() == w.priorCourseEnrollmentID {
		return fmt.Errorf("expected a fresh enrollment id distinct from the abandoned one %s", w.priorCourseEnrollmentID)
	}
	return nil
}

// findActiveEnrollmentAtCheckpoint returns studentID's sole active
// CourseEnrollment currently at checkpoint position — every scenario using
// "completes every item in checkpoint N's student path" has at most one
// such enrollment at a time, since only one course in play ever reaches
// position > 1.
func (w *world) findActiveEnrollmentAtCheckpoint(studentID string, position int) (domain.CourseEnrollment, error) {
	all, err := w.courseEnrollments.ListByStudentID(context.Background(), studentID)
	if err != nil {
		return domain.CourseEnrollment{}, err
	}
	var match *domain.CourseEnrollment
	for i := range all {
		e := all[i]
		if e.IsActive() && e.ActiveCheckpointPosition != nil && *e.ActiveCheckpointPosition == position {
			if match != nil {
				return domain.CourseEnrollment{}, fmt.Errorf("multiple active enrollments found at checkpoint %d for this student", position)
			}
			match = &e
		}
	}
	if match == nil {
		return domain.CourseEnrollment{}, fmt.Errorf("no active enrollment found at checkpoint %d for this student", position)
	}
	return *match, nil
}

// completesCheckpointItems marks every item of name's active checkpoint at
// position complete on the fake CompletionStateReader — course-domain has no
// separate "mark item complete" endpoint of its own; completion is pushed by
// the Aggregation Worker in production and discovered lazily on the next
// relevant read (GetMyPath, SetCurrentPath, or ListMyCourseEnrollments, per
// checkAndAdvanceCheckpoint's doc comment), so this step deliberately leaves
// that discovery to whichever read step the scenario itself performs next,
// rather than triggering it here. A course completing on the discovering
// call is a one-shot signal (GetMyPath/SetCurrentPath surface it once, then
// the pointer is gone); triggering it here as a side effect of setup would
// silently consume that signal before the scenario's own read gets to
// observe it. Scenarios that assert directly against fake repository state
// with no read step of their own (e.g. checkpointStudentPathIsCreated) must
// trigger discovery themselves.
func (w *world) completesCheckpointItems(name string, position int) error {
	studentID, ok := w.userMotifID[name]
	if !ok {
		return fmt.Errorf("no user %q has been registered", name)
	}
	enrollment, err := w.findActiveEnrollmentAtCheckpoint(studentID.String(), position)
	if err != nil {
		return err
	}
	sp, err := w.studentPaths.GetByID(context.Background(), *enrollment.ActiveCheckpointStudentPathID)
	if err != nil {
		return err
	}
	for _, item := range sp.Items {
		w.completion.set(studentID.String(), item.ContentNodeID, domain.CompletionStatusCompleted)
	}
	for slug, id := range w.courseIDBySlug {
		if id.String() == enrollment.CourseID {
			w.lastCourseEnrollmentKey = courseEnrollKey(name, slug)
			break
		}
	}
	return nil
}

// triggerCheckpointDiscoveryForLastEnrollment calls GetMyPath as the
// student named in w.lastCourseEnrollmentKey, whose only purpose here is
// its checkAndAdvanceCheckpoint side effect — this persists a checkpoint
// advance that has already happened (per completed item state) but has not
// yet been discovered by any read. Used by Then steps that assert directly
// against fake repository state, with no read step of the scenario's own to
// have triggered discovery already.
func (w *world) triggerCheckpointDiscoveryForLastEnrollment() error {
	name, _, ok := strings.Cut(w.lastCourseEnrollmentKey, "|")
	if !ok {
		return fmt.Errorf("no enrollment tracked to trigger discovery for")
	}
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
	_, err := w.handler.GetMyPath(ctx, generated.GetMyPathRequestObject{})
	return err
}

func (w *world) responseShowsCheckpointItemsAllCompletedWithCourseCompleted(position int) error {
	resp, ok := w.lastResp.(generated.GetMyPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.CourseCheckpointPosition == nil || *resp.CourseCheckpointPosition != position {
		return fmt.Errorf("expected checkpoint position %d, got %+v", position, resp.CourseCheckpointPosition)
	}
	if !resp.CourseCompleted {
		return fmt.Errorf("expected course_completed true")
	}
	for _, item := range resp.Items {
		if string(item.Status) != "completed" {
			return fmt.Errorf("expected every item completed, got %+v", item)
		}
	}
	return nil
}

func (w *world) listsCourseEnrollments(name string) error {
	resp, err := w.handler.ListMyCourseEnrollments(w.ctx(), generated.ListMyCourseEnrollmentsRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) responseIncludesActiveEnrollmentWithProgress(courseSlug string) error {
	resp, ok := w.lastResp.(generated.ListMyCourseEnrollments200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	wantCourseID, ok := w.courseIDBySlug[courseSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", courseSlug)
	}
	for _, e := range resp {
		if e.CourseId == wantCourseID {
			if e.Status != generated.CourseEnrollmentStatus(domain.CourseEnrollmentStatusActive) {
				return fmt.Errorf("expected %q's enrollment to be active, got %q", courseSlug, e.Status)
			}
			if e.ActiveCheckpointPosition == nil {
				return fmt.Errorf("expected %q's enrollment to carry its current checkpoint", courseSlug)
			}
			return nil
		}
	}
	return fmt.Errorf("expected the response to include an enrollment for %q, got %+v", courseSlug, resp)
}

func (w *world) responseIncludesNoOtherActiveEnrollment() error {
	resp, ok := w.lastResp.(generated.ListMyCourseEnrollments200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, e := range resp {
		if e.Status == generated.CourseEnrollmentStatus(domain.CourseEnrollmentStatusActive) {
			return fmt.Errorf("expected no active enrollment in the response, found %+v", e)
		}
	}
	return nil
}

func (w *world) responseIncludesCompletedAndActive(completedSlug, activeSlug string) error {
	resp, ok := w.lastResp.(generated.ListMyCourseEnrollments200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	wantCompleted, ok := w.courseIDBySlug[completedSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", completedSlug)
	}
	wantActive, ok := w.courseIDBySlug[activeSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", activeSlug)
	}
	var sawCompleted, sawActive bool
	for _, e := range resp {
		if e.CourseId == wantCompleted && e.Status == generated.CourseEnrollmentStatus(domain.CourseEnrollmentStatusCompleted) {
			sawCompleted = true
		}
		if e.CourseId == wantActive && e.Status == generated.CourseEnrollmentStatus(domain.CourseEnrollmentStatusActive) {
			sawActive = true
		}
	}
	if !sawCompleted {
		return fmt.Errorf("expected a completed enrollment for %q", completedSlug)
	}
	if !sawActive {
		return fmt.Errorf("expected an active enrollment for %q", activeSlug)
	}
	return nil
}

func (w *world) abandonsEnrollment(name, courseSlug string) error {
	id, ok := w.courseEnrollmentIDByKey[courseEnrollKey(name, courseSlug)]
	if !ok {
		return fmt.Errorf("no enrollment tracked for %q in %q", name, courseSlug)
	}
	eid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	resp, err := w.handler.AbandonCourseEnrollment(w.ctx(), generated.AbandonCourseEnrollmentRequestObject{CourseEnrollmentId: eid})
	w.lastResp, w.lastErr = resp, err
	w.lastCourseEnrollmentKey = courseEnrollKey(name, courseSlug)
	return err
}

func (w *world) attemptsAbandonEnrollment(name, courseSlug string) error {
	return w.abandonsEnrollment(name, courseSlug)
}
