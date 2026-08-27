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

func registerPathAssignmentSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^student "([^"]+)" is registered in the system$`, func(name string) error { w.ensureRegistered(name, domain.RoleStudent); return nil })
	sc.Step(`^"([^"]+)" is registered as a teacher$`, func(name string) error { w.ensureRegistered(name, domain.RoleTeacher); return nil })
	sc.Step(`^"([^"]+)" already has "([^"]+)" assigned$`, w.alreadyHasAssigned)

	sc.Step(`^"([^"]+)" assigns "([^"]+)" to student "([^"]+)"$`, w.assignsPathToStudent)
	sc.Step(`^"([^"]+)" assigns "([^"]+)" to a student ID that does not exist$`, w.assignsToMissingStudent)
	sc.Step(`^"([^"]+)" assigns a learning path ID that does not exist to student "([^"]+)"$`, w.assignsMissingPath)
	sc.Step(`^"([^"]+)" assigns "([^"]+)" to "([^"]+)"$`, w.assignsPathToStudent)
	sc.Step(`^"([^"]+)" attempts to assign "([^"]+)" to herself$`, w.attemptsAssignToSelf)
	sc.Step(`^an unauthenticated request attempts to assign a learning path$`, w.unauthAssignsPath)

	sc.Step(`^an assignment record is created and returned$`, w.assignmentCreated)
	sc.Step(`^the assignment records "([^"]+)" as the assigner and "([^"]+)" as the student$`, w.assignmentRecordsAssignerAndStudent)
	sc.Step(`^a new assignment record is returned for "([^"]+)"$`, w.newAssignmentReturnedFor)
	sc.Step(`^"([^"]+)"'s active path is now "([^"]+)"$`, w.activePathIsNow)
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
	if _, ok := resp.(generated.AssignLearningPath201JSONResponse); !ok {
		return fmt.Errorf("setup: expected assignment to succeed, got %#v", resp)
	}
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
	resp, err := w.handler.AssignLearningPath(w.ctx(), generated.AssignLearningPathRequestObject{
		StudentId: deterministicUUID("motif-user", "alice"),
		Body:      &generated.AssignLearningPathRequest{LearningPathId: pathID("week-1-path")},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) assignmentCreated() error {
	if _, ok := w.lastResp.(generated.AssignLearningPath201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) assignmentRecordsAssignerAndStudent(assignerName, studentName string) error {
	resp, ok := w.lastResp.(generated.AssignLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.AssignedBy != w.userMotifID[assignerName] {
		return fmt.Errorf("expected assigned_by %s for %q, got %s", w.userMotifID[assignerName], assignerName, resp.AssignedBy)
	}
	expectedStudent := w.userMotifID[studentName]
	if resp.StudentId != expectedStudent {
		return fmt.Errorf("expected student_id %s for %q, got %s", expectedStudent, studentName, resp.StudentId)
	}
	return nil
}

func (w *world) newAssignmentReturnedFor(pathSlug string) error {
	resp, ok := w.lastResp.(generated.AssignLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.LearningPathId != pathID(pathSlug) {
		return fmt.Errorf("expected learning_path_id %s, got %s", pathID(pathSlug), resp.LearningPathId)
	}
	return nil
}

func (w *world) activePathIsNow(studentName, pathSlug string) error {
	studentMotifID := w.userMotifID[studentName]
	active, err := w.assignments.GetActiveByStudentID(w.ctx(), studentMotifID.String())
	if err != nil {
		return err
	}
	if active.LearningPathID != pathID(pathSlug).String() {
		return fmt.Errorf("expected active path %s for %q, got %s", pathID(pathSlug), studentName, active.LearningPathID)
	}
	return nil
}
