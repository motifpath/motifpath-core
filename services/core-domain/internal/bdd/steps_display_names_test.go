//go:build integration

package bdd

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerDisplayNameSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" is named "([^"]*)"$`, w.isNamed)
	sc.Step(`^"([^"]+)" is named with (\d+) characters$`, w.isNamedWithLength)
	sc.Step(`^"([^"]+)" is now named "([^"]*)"$`, w.isNowNamed)
	sc.Step(`^the identity token of "([^"]+)" (?:now )?carries no name$`, w.carriesNoName)

	sc.Step(`^the response includes the display name "([^"]+)"$`, w.responseIncludesDisplayName)
	sc.Step(`^the response includes a display name of exactly (\d+) characters$`, w.responseIncludesDisplayNameOfLength)
	sc.Step(`^no user record exists for "([^"]+)"$`, w.noUserRecordExists)

	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", "([^"]+)", "([^"]+)", created by "([^"]+)"$`, w.putLearningPathThreeItemsCreatedBy)

	sc.Step(`^catalog entry "([^"]+)" names its creator as "([^"]+)", "([^"]+)"$`, w.catalogEntryNamesCreator)
	sc.Step(`^the course names its creator as "([^"]+)", "([^"]+)"$`, w.courseNamesCreator)
	sc.Step(`^the content node names its teacher as "([^"]+)", "([^"]+)"$`, w.contentNodeNamesTeacher)
	sc.Step(`^the learning path names its teacher as "([^"]+)", "([^"]+)"$`, w.learningPathNamesTeacher)
	sc.Step(`^the diagram names its creator as "([^"]+)", "([^"]+)"$`, w.diagramNamesCreator)
	sc.Step(`^the student path names its student as "([^"]+)", "([^"]+)"$`, w.studentPathNamesStudent)
	sc.Step(`^the student path names its assigner as "([^"]+)", "([^"]+)"$`, w.studentPathNamesAssigner)
	sc.Step(`^standalone path "([^"]+)" names its assigner as "([^"]+)", "([^"]+)"$`, w.standalonePathNamesAssigner)
	sc.Step(`^enrollment "([^"]+)" names its student as "([^"]+)", "([^"]+)"$`, w.enrollmentNamesStudent)
}

// isNamed sets the name persona's session token carries. It describes who
// persona is from the start of the scenario, so an already-registered
// persona makes one request with it straight away — which is what stores a
// name — before any later step reads it back.
func (w *world) isNamed(persona, name string) error {
	w.nameClaims[persona] = name
	if _, registered := w.userMotifID[persona]; !registered {
		return nil
	}
	resp, err := w.handler.GetMyProfile(w.identityCtx(persona), generated.GetMyProfileRequestObject{})
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.GetMyProfile200JSONResponse); !ok {
		return fmt.Errorf("setup: expected %q's profile request to succeed, got %#v", persona, resp)
	}
	return nil
}

func (w *world) isNamedWithLength(persona string, length int) error {
	return w.isNamed(persona, strings.Repeat("a", length))
}

// isNowNamed changes the name persona's token carries without making a
// request — a rename in Clerk — so a scenario can show the stored name only
// follows on persona's next request.
func (w *world) isNowNamed(persona, name string) error {
	w.nameClaims[persona] = name
	return nil
}

func (w *world) carriesNoName(persona string) error {
	w.nameClaims[persona] = ""
	return nil
}

// lastProfileDisplayName reads display_name from whichever profile-shaped
// response the last step got: a registration or a profile request.
func (w *world) lastProfileDisplayName() (string, error) {
	switch resp := w.lastResp.(type) {
	case generated.RegisterUser201JSONResponse:
		return resp.DisplayName, nil
	case generated.GetMyProfile200JSONResponse:
		return resp.DisplayName, nil
	default:
		return "", fmt.Errorf("expected a registration or profile response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) responseIncludesDisplayName(want string) error {
	got, err := w.lastProfileDisplayName()
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("expected display name %q, got %q", want, got)
	}
	return nil
}

func (w *world) responseIncludesDisplayNameOfLength(length int) error {
	got, err := w.lastProfileDisplayName()
	if err != nil {
		return err
	}
	if n := utf8.RuneCountInString(got); n != length {
		return fmt.Errorf("expected a display name of %d characters, got %d", length, n)
	}
	return nil
}

func (w *world) noUserRecordExists(persona string) error {
	user, err := w.users.GetByClerkUserID(w.ctx(), clerkSub(persona))
	if err == nil {
		return fmt.Errorf("expected no user record for %q, found %s", persona, user.ID)
	}
	return nil
}

func (w *world) putLearningPathThreeItemsCreatedBy(slug, n1, n2, n3, creator string) error {
	if err := w.putLearningPathThreeItems(slug, n1, n2, n3); err != nil {
		return err
	}
	path, err := w.paths.GetByID(w.ctx(), pathID(slug).String())
	if err != nil {
		return err
	}
	path.TeacherID = w.ensureRegistered(creator, domain.RoleTeacher).String()
	w.paths.put(path)
	return nil
}

// expectUserRef checks that ref points at persona and carries name.
func (w *world) expectUserRef(what string, ref generated.UserRef, persona, name string) error {
	wantID, ok := w.userMotifID[persona]
	if !ok {
		return fmt.Errorf("setup: %q was never registered", persona)
	}
	want := generated.UserRef{UserId: wantID, DisplayName: name}
	if ref != want {
		return fmt.Errorf("expected %s to be %s (%q), got %s (%q)", what, want.UserId, want.DisplayName, ref.UserId, ref.DisplayName)
	}
	return nil
}

func (w *world) catalogEntryNamesCreator(slug, persona, name string) error {
	resp, ok := w.lastResp.(generated.ListCourses200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a course list response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, entry := range resp.Items {
		if w.courseMatchesSlug(entry, slug) {
			return w.expectUserRef(slug+"'s creator", entry.CreatedBy, persona, name)
		}
	}
	return fmt.Errorf("expected the catalog to include %q", slug)
}

func (w *world) courseNamesCreator(persona, name string) error {
	resp, ok := w.lastResp.(generated.GetCourse200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a course response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return w.expectUserRef("the course's creator", resp.CreatedBy, persona, name)
}

func (w *world) contentNodeNamesTeacher(persona, name string) error {
	resp, ok := w.lastResp.(generated.GetContentNode200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a content node response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return w.expectUserRef("the content node's teacher", resp.Teacher, persona, name)
}

func (w *world) learningPathNamesTeacher(persona, name string) error {
	resp, ok := w.lastResp.(generated.GetLearningPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a learning path response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return w.expectUserRef("the learning path's teacher", resp.Teacher, persona, name)
}

func (w *world) diagramNamesCreator(persona, name string) error {
	resp, ok := w.lastResp.(generated.GetDiagram200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a diagram response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return w.expectUserRef("the diagram's creator", resp.CreatedBy, persona, name)
}

func (w *world) lastAssignedStudentPath() (generated.StudentPath, error) {
	resp, ok := w.lastResp.(generated.AssignLearningPath201JSONResponse)
	if !ok {
		return generated.StudentPath{}, fmt.Errorf("expected an assignment response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return generated.StudentPath(resp), nil
}

func (w *world) studentPathNamesStudent(persona, name string) error {
	sp, err := w.lastAssignedStudentPath()
	if err != nil {
		return err
	}
	return w.expectUserRef("the student path's student", sp.Student, persona, name)
}

func (w *world) studentPathNamesAssigner(persona, name string) error {
	sp, err := w.lastAssignedStudentPath()
	if err != nil {
		return err
	}
	return w.expectUserRef("the student path's assigner", sp.AssignedBy, persona, name)
}

func (w *world) standalonePathNamesAssigner(templateSlug, persona, name string) error {
	resp, ok := w.lastResp.(generated.ListMyStandalonePaths200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a standalone path list response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, sp := range resp {
		if sp.SourceTemplateId == pathID(templateSlug) {
			return w.expectUserRef(templateSlug+"'s assigner", sp.AssignedBy, persona, name)
		}
	}
	return fmt.Errorf("expected a standalone path copied from %q", templateSlug)
}

func (w *world) enrollmentNamesStudent(courseSlug, persona, name string) error {
	resp, ok := w.lastResp.(generated.ListMyCourseEnrollments200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a course enrollment list response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	courseID, ok := w.courseIDBySlug[courseSlug]
	if !ok {
		return fmt.Errorf("setup: course %q was never created", courseSlug)
	}
	for _, e := range resp {
		if e.CourseId == uuid.UUID(courseID) {
			return w.expectUserRef(courseSlug+"'s enrolled student", e.Student, persona, name)
		}
	}
	return fmt.Errorf("expected an enrollment in %q", courseSlug)
}
