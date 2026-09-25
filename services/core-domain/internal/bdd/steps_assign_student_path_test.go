//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerAssignStudentPathSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^student "([^"]+)" is registered in the system$`, func(name string) error { w.ensureRegistered(name, domain.RoleStudent); return nil })
	sc.Step(`^"([^"]+)" is registered as a teacher$`, func(name string) error { w.ensureRegistered(name, domain.RoleTeacher); return nil })
	sc.Step(`^"([^"]+)" already has "([^"]+)" assigned as her current path$`, w.alreadyHasAssigned)

	sc.Step(`^"([^"]+)" assigns "([^"]+)" to student "([^"]+)"$`, w.assignsPathToStudent)
	sc.Step(`^"([^"]+)" assigns "([^"]+)" to a student ID that does not exist$`, w.assignsToMissingStudent)
	sc.Step(`^"([^"]+)" assigns a learning path ID that does not exist to student "([^"]+)"$`, w.assignsMissingPath)
	sc.Step(`^"([^"]+)" assigns "([^"]+)" to "([^"]+)"$`, w.assignsPathToStudent)
	sc.Step(`^"([^"]+)" attempts to assign "([^"]+)" to herself$`, w.attemptsAssignToSelf)
	sc.Step(`^an unauthenticated request attempts to assign a learning path$`, w.unauthAssignsPath)
	sc.Step(`^"([^"]+)" edits "([^"]+)"'s copy of the path$`, w.editsStudentsCopyOfPath)

	sc.Step(`^a student path is created and returned, copied from "([^"]+)"$`, w.studentPathCreatedCopiedFrom)
	sc.Step(`^the student path records "([^"]+)" as the assigner and "([^"]+)" as the owner$`, w.studentPathRecordsAssignerAndOwner)
	sc.Step(`^the student path becomes "([^"]+)"'s current path$`, w.studentPathBecomesCurrentPath)
	sc.Step(`^a new student path is returned, copied from "([^"]+)"$`, w.studentPathCreatedCopiedFrom)
	sc.Step(`^"([^"]+)"'s current path is now the new copy of "([^"]+)"$`, w.currentPathIsNowNewCopyOf)
	sc.Step(`^"([^"]+)"'s earlier copy of "([^"]+)" still exists and is not archived$`, w.earlierCopyStillExistsNotArchived)
	sc.Step(`^"([^"]+)" and any other student's copy of it are unchanged$`, w.templateUnchangedAfterEdit)
}

func (w *world) alreadyHasAssigned(name, pathSlug string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent)
	teacherName := "seed-teacher-for-" + name
	w.ensureRegistered(teacherName, domain.RoleTeacher)

	// Uses a context authenticated as the seed teacher, independent of the
	// scenario's own current w.hasToken/w.clerkSub state, so this setup
	// helper doesn't disturb whichever identity the scenario itself is
	// about to authenticate as.
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
	// Recorded so a later assign in the same scenario (which overwrites
	// w.lastResp) can still be compared against this earlier copy.
	w.priorStudentPathID = created.StudentPathId.String()
	return nil
}

func (w *world) assignsPathToStudent(name, pathSlug, studentName string) error {
	studentMotifID, ok := w.userMotifID[studentName]
	if !ok {
		studentMotifID = deterministicUUID("motif-user", studentName)
	}
	resp, err := w.handler.AssignLearningPath(w.ctx(), generated.AssignLearningPathRequestObject{
		StudentId: studentMotifID,
		Body:      &generated.AssignLearningPathRequest{LearningPathId: pathID(pathSlug)},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) assignsToMissingStudent(name, pathSlug string) error {
	resp, err := w.handler.AssignLearningPath(w.ctx(), generated.AssignLearningPathRequestObject{
		StudentId: deterministicUUID("motif-user", "does-not-exist"),
		Body:      &generated.AssignLearningPathRequest{LearningPathId: pathID(pathSlug)},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) assignsMissingPath(name, studentName string) error {
	studentMotifID, ok := w.userMotifID[studentName]
	if !ok {
		studentMotifID = deterministicUUID("motif-user", studentName)
	}
	resp, err := w.handler.AssignLearningPath(w.ctx(), generated.AssignLearningPathRequestObject{
		StudentId: studentMotifID,
		Body:      &generated.AssignLearningPathRequest{LearningPathId: deterministicUUID("path", "does-not-exist")},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsAssignToSelf(name, pathSlug string) error {
	studentMotifID, ok := w.userMotifID[name]
	if !ok {
		studentMotifID = deterministicUUID("motif-user", name)
	}
	resp, err := w.handler.AssignLearningPath(w.ctx(), generated.AssignLearningPathRequestObject{
		StudentId: studentMotifID,
		Body:      &generated.AssignLearningPathRequest{LearningPathId: pathID(pathSlug)},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthAssignsPath() error {
	w.noAuthToken() //nolint:errcheck // never errors
	// The step this backs — "an unauthenticated request attempts to assign a
	// learning path" — names no path, and authentication is refused before
	// any path lookup, so the id is deliberately one no scenario seeds.
	resp, err := w.handler.AssignLearningPath(w.ctx(), generated.AssignLearningPathRequestObject{
		StudentId: deterministicUUID("motif-user", "alice"),
		Body:      &generated.AssignLearningPathRequest{LearningPathId: pathID("unauthenticated-request-path")},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) studentPathCreatedCopiedFrom(pathSlug string) error {
	resp, ok := w.lastResp.(generated.AssignLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.SourceTemplateId != pathID(pathSlug) {
		return fmt.Errorf("expected source_template_id %s, got %s", pathID(pathSlug), resp.SourceTemplateId)
	}
	return nil
}

func (w *world) studentPathRecordsAssignerAndOwner(assignerName, studentName string) error {
	resp, ok := w.lastResp.(generated.AssignLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.AssignedBy.UserId != w.userMotifID[assignerName] {
		return fmt.Errorf("expected assigned_by %s for %q, got %s", w.userMotifID[assignerName], assignerName, resp.AssignedBy.UserId)
	}
	expectedStudent := w.userMotifID[studentName]
	if resp.Student.UserId != expectedStudent {
		return fmt.Errorf("expected student_id %s for %q, got %s", expectedStudent, studentName, resp.Student.UserId)
	}
	return nil
}

func (w *world) studentPathBecomesCurrentPath(studentName string) error {
	resp, ok := w.lastResp.(generated.AssignLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	studentMotifID := w.userMotifID[studentName]
	state, err := w.learningState.GetByStudentID(context.Background(), studentMotifID.String())
	if err != nil {
		return err
	}
	if state.CurrentStandalonePathID == nil || *state.CurrentStandalonePathID != resp.StudentPathId.String() {
		return fmt.Errorf("expected %q's current path to be %s, got %+v", studentName, resp.StudentPathId, state.CurrentStandalonePathID)
	}
	return nil
}

func (w *world) currentPathIsNowNewCopyOf(studentName, pathSlug string) error {
	if err := w.studentPathBecomesCurrentPath(studentName); err != nil {
		return err
	}
	return w.studentPathCreatedCopiedFrom(pathSlug)
}

func (w *world) earlierCopyStillExistsNotArchived(studentName, pathSlug string) error {
	// The scenario this backs assigns two paths in sequence; w.lastResp at
	// this point holds the second (current) response, so the earlier copy
	// must be found via the student's learning state history instead — the
	// fake in-memory repo has no "list all copies ever made" query, so this
	// asserts indirectly: the earlier StudentPath id was captured by
	// studentPathCreatedCopiedFrom's caller into w.priorStudentPathID.
	if w.priorStudentPathID == "" {
		return fmt.Errorf("no earlier student path id was recorded to check")
	}
	earlier, err := w.studentPaths.GetByID(context.Background(), w.priorStudentPathID)
	if err != nil {
		return fmt.Errorf("expected the earlier copy of %q to still exist: %w", pathSlug, err)
	}
	if earlier.ArchivedAt != nil {
		return fmt.Errorf("expected the earlier copy of %q to not be archived", pathSlug)
	}
	return nil
}

func (w *world) editsStudentsCopyOfPath(teacherName, studentName string) error {
	studentMotifID := w.userMotifID[studentName]
	state, err := w.learningState.GetByStudentID(context.Background(), studentMotifID.String())
	if err != nil {
		return err
	}
	if state.CurrentStandalonePathID == nil {
		return fmt.Errorf("%q has no current standalone path to edit", studentName)
	}
	// Remember the pre-edit copy so templateUnchangedAfterEdit can compare
	// the template against it, and so an "assigning a new path" scenario
	// elsewhere can find "the earlier copy" via w.priorStudentPathID.
	w.priorStudentPathID = *state.CurrentStandalonePathID
	sp, err := w.studentPaths.GetByID(context.Background(), *state.CurrentStandalonePathID)
	if err != nil {
		return err
	}
	sp.Title = "Edited copy title"
	return w.studentPaths.Create(context.Background(), sp)
}

func (w *world) templateUnchangedAfterEdit(pathSlug string) error {
	template, err := w.paths.GetByID(context.Background(), pathID(pathSlug).String())
	if err != nil {
		return err
	}
	if template.Title != pathSlug {
		return fmt.Errorf("expected template %q title to remain %q, got %q", pathSlug, pathSlug, template.Title)
	}
	return nil
}
