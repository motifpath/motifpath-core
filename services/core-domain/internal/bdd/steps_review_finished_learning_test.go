//go:build integration

package bdd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerReviewFinishedLearningSteps(sc *godog.ScenarioContext, w *world) {
	// ── Seeding ─────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" has completed course "([^"]+)"$`, w.completesCourse)
	sc.Step(`^student "([^"]+)" has completed course "([^"]+)"$`, w.completesCourse)
	sc.Step(`^a newer version of "([^"]+)" with a third checkpoint "([^"]+)" has since been published$`, w.republishesWithThirdCheckpoint)
	sc.Step(`^"([^"]+)" holds a standalone path "([^"]+)" and has completed every item in it$`, w.holdsCompletedStandalonePath)
	sc.Step(`^student "([^"]+)" holds a standalone path "([^"]+)"$`, func(name, slug string) error {
		return w.holdsStandalonePath(name, slug, "active")
	})

	// ── Reads ───────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" lists the parts of her "([^"]+)" enrollment$`, func(name, courseSlug string) error {
		return w.listsEnrollmentParts(name, courseSlug)
	})
	sc.Step(`^"([^"]+)" lists the parts of "([^"]+)"'s "([^"]+)" enrollment$`, func(_, owner, courseSlug string) error {
		return w.listsEnrollmentParts(owner, courseSlug)
	})
	sc.Step(`^an unauthenticated request lists the parts of an enrollment$`, func() error {
		w.noAuthToken() //nolint:errcheck // never errors
		resp, err := w.handler.ListMyCourseEnrollmentParts(w.ctx(), generated.ListMyCourseEnrollmentPartsRequestObject{CourseEnrollmentId: uuid.New()})
		w.lastResp, w.lastErr = resp, err
		return err
	})
	sc.Step(`^"([^"]+)" reads her "([^"]+)" student path$`, func(name, slug string) error {
		return w.readsStudentPath(w.standalonePathIDByKey[name+"|"+slug])
	})
	sc.Step(`^"([^"]+)" reads "([^"]+)"'s "([^"]+)" student path$`, func(_, owner, slug string) error {
		return w.readsStudentPath(w.standalonePathIDByKey[owner+"|"+slug])
	})
	sc.Step(`^"([^"]+)" reads the student path of part (\d+) of her "([^"]+)" enrollment$`, w.readsEnrollmentPart)
	sc.Step(`^"([^"]+)" reads the student path "([^"]+)"$`, w.readsStudentPathThroughRouter)

	// ── Outcomes ────────────────────────────────────────────────────────────
	sc.Step(`^the response holds the student paths of "([^"]+)" and "([^"]+)", in that order$`, func(first, second string) error {
		return w.partsAre(first, second)
	})
	sc.Step(`^the response holds only "([^"]+)" and "([^"]+)"$`, func(first, second string) error {
		return w.partsAre(first, second)
	})
	sc.Step(`^the response holds only the student path of "([^"]+)"$`, func(slug string) error {
		return w.partsAre(slug)
	})
	sc.Step(`^every item of both parts has status "([^"]+)"$`, func(status string) error {
		parts, err := w.enrollmentParts()
		if err != nil {
			return err
		}
		for _, part := range parts {
			if err := itemsAllHaveStatus(part.Items, status); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^no part reports course_completed$`, func() error {
		parts, err := w.enrollmentParts()
		if err != nil {
			return err
		}
		for _, part := range parts {
			if part.CourseCompleted {
				return fmt.Errorf("expected no part to report course_completed, got %s", part.StudentPathId)
			}
		}
		return nil
	})
	sc.Step(`^the enrollment status is still "([^"]+)"$`, func(status string) error {
		e, err := w.courseEnrollments.GetByID(context.Background(), w.courseEnrollmentIDByKey[w.lastCourseEnrollmentKey])
		if err != nil {
			return err
		}
		if string(e.Status) != status {
			return fmt.Errorf("expected the enrollment %s, got %s", status, e.Status)
		}
		return nil
	})
	sc.Step(`^the response holds every item of "([^"]+)" in order, each with status "([^"]+)"$`, func(slug, status string) error {
		view, err := w.studentPathRead()
		if err != nil {
			return err
		}
		if err := w.itemsMatchTemplate(view.Items, slug); err != nil {
			return err
		}
		return itemsAllHaveStatus(view.Items, status)
	})
	sc.Step(`^the response holds every item of "([^"]+)" in order$`, func(slug string) error {
		view, err := w.studentPathRead()
		if err != nil {
			return err
		}
		return w.itemsMatchTemplate(view.Items, slug)
	})
	sc.Step(`^the response is "([^"]+)" with course checkpoint position (\d+)$`, func(slug string, position int) error {
		view, err := w.studentPathRead()
		if err != nil {
			return err
		}
		if view.SourceTemplateId != pathID(slug) {
			return fmt.Errorf("expected a copy of %q, got %s", slug, view.SourceTemplateId)
		}
		if view.CourseCheckpointPosition == nil || *view.CourseCheckpointPosition != position {
			return fmt.Errorf("expected checkpoint position %d, got %v", position, view.CourseCheckpointPosition)
		}
		return nil
	})
	sc.Step(`^the response does not report course_completed$`, func() error {
		view, err := w.studentPathRead()
		if err != nil {
			return err
		}
		if view.CourseCompleted {
			return fmt.Errorf("expected course_completed false")
		}
		return nil
	})
}

// completesCourse enrolls name in courseSlug and completes every
// checkpoint in turn, letting the course-enrollment list discover each
// advance, until the enrollment is completed.
func (w *world) completesCourse(name, courseSlug string) error {
	created, err := w.enrollAsIsolated(name, courseSlug)
	if err != nil {
		return err
	}
	studentID := w.userMotifID[name].String()
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
	for {
		e, err := w.courseEnrollments.GetByID(context.Background(), created.CourseEnrollmentId.String())
		if err != nil {
			return err
		}
		if e.Status == domain.CourseEnrollmentStatusCompleted {
			w.lastCourseEnrollmentKey = courseEnrollKey(name, courseSlug)
			return nil
		}
		if e.ActiveCheckpointStudentPathID == nil {
			return fmt.Errorf("setup: enrollment of %q in %q has no active checkpoint and is %s", name, courseSlug, e.Status)
		}
		sp, err := w.studentPaths.GetByID(context.Background(), *e.ActiveCheckpointStudentPathID)
		if err != nil {
			return err
		}
		for _, item := range sp.Items {
			w.completion.set(studentID, item.ContentNodeID, domain.CompletionStatusCompleted)
		}
		if _, err := w.handler.ListMyCourseEnrollments(ctx, generated.ListMyCourseEnrollmentsRequestObject{}); err != nil {
			return err
		}
	}
}

// republishesWithThirdCheckpoint publishes a new version of courseSlug that
// appends thirdSlug as a checkpoint, as the course's creator.
func (w *world) republishesWithThirdCheckpoint(courseSlug, thirdSlug string) error {
	if err := w.ensurePaths([]string{thirdSlug}); err != nil {
		return err
	}
	courseID := w.courseIDBySlug[courseSlug]
	course, err := w.courses.GetByID(context.Background(), courseID.String())
	if err != nil {
		return err
	}
	checkpoints := make([]courseCheckpointBody, 0, len(course.Checkpoints)+1)
	for _, checkpoint := range course.Checkpoints {
		checkpoints = append(checkpoints, courseCheckpointBody{LearningPathId: uuid.MustParse(checkpoint.LearningPathID), Title: checkpoint.Title})
	}
	checkpoints = append(checkpoints, courseCheckpointBody{LearningPathId: pathID(thirdSlug)})

	teacherCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub("bob"))
	replaced, err := w.handler.ReplaceCourse(teacherCtx, generated.ReplaceCourseRequestObject{
		CourseId: courseID,
		Body: &generated.ReplaceCourseRequest{
			Language: "en", Title: course.Title, Summary: "A third part.",
			Level: generated.ReplaceCourseRequestLevelBeginner, Checkpoints: checkpoints,
		},
	})
	if err != nil {
		return err
	}
	if _, ok := replaced.(generated.ReplaceCourse200JSONResponse); !ok {
		return fmt.Errorf("setup: expected course replacement to succeed, got %#v", replaced)
	}

	adminName := "review-republish-admin"
	w.ensureRegistered(adminName, domain.RoleAdmin)
	adminCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub(adminName))
	published, err := w.handler.PublishCourse(adminCtx, generated.PublishCourseRequestObject{CourseId: courseID})
	if err != nil {
		return err
	}
	if _, ok := published.(generated.PublishCourse201JSONResponse); !ok {
		return fmt.Errorf("setup: expected course republish to succeed, got %#v", published)
	}
	return nil
}

func (w *world) holdsCompletedStandalonePath(name, slug string) error {
	if err := w.holdsStandalonePath(name, slug, "active"); err != nil {
		return err
	}
	sp, err := w.studentPaths.GetByID(context.Background(), w.standalonePathIDByKey[name+"|"+slug])
	if err != nil {
		return err
	}
	for _, item := range sp.Items {
		w.completion.set(w.userMotifID[name].String(), item.ContentNodeID, domain.CompletionStatusCompleted)
	}
	return nil
}

func (w *world) listsEnrollmentParts(owner, courseSlug string) error {
	enrollmentID, ok := w.courseEnrollmentIDByKey[courseEnrollKey(owner, courseSlug)]
	if !ok {
		return fmt.Errorf("no enrollment of %q in %q was created", owner, courseSlug)
	}
	resp, err := w.handler.ListMyCourseEnrollmentParts(w.ctx(), generated.ListMyCourseEnrollmentPartsRequestObject{CourseEnrollmentId: uuid.MustParse(enrollmentID)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) readsStudentPath(studentPathID string) error {
	id, err := uuid.Parse(studentPathID)
	if err != nil {
		return fmt.Errorf("setup: no student path was recorded for this step: %w", err)
	}
	resp, err := w.handler.GetMyStudentPath(w.ctx(), generated.GetMyStudentPathRequestObject{StudentPathId: id})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) readsEnrollmentPart(name string, position int, courseSlug string) error {
	parts, err := w.studentPaths.ListByCourseEnrollmentID(context.Background(), w.courseEnrollmentIDByKey[courseEnrollKey(name, courseSlug)])
	if err != nil {
		return err
	}
	for _, sp := range parts {
		if sp.CourseCheckpointPosition != nil && *sp.CourseCheckpointPosition == position {
			return w.readsStudentPath(sp.ID)
		}
	}
	return fmt.Errorf("setup: %q has no part %d in %q", name, position, courseSlug)
}

// readsStudentPathThroughRouter sends the read through the generated
// router, since only the router can receive an id that is not a UUID.
func (w *world) readsStudentPathThroughRouter(_, rawID string) error {
	router := generated.HandlerWithOptions(generated.NewStrictHandler(w.handler, nil), generated.ChiServerOptions{})
	req := httptest.NewRequestWithContext(w.ctx(), http.MethodGet, "/students/me/student-paths/"+rawID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	w.lastResp, w.lastErr = nil, nil
	w.lastRouterStatus = rec.Code
	return nil
}

func (w *world) enrollmentParts() (generated.ListMyCourseEnrollmentParts200JSONResponse, error) {
	parts, ok := w.lastResp.(generated.ListMyCourseEnrollmentParts200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("expected the parts of an enrollment, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return parts, nil
}

func (w *world) studentPathRead() (generated.GetMyStudentPath200JSONResponse, error) {
	view, ok := w.lastResp.(generated.GetMyStudentPath200JSONResponse)
	if !ok {
		return generated.GetMyStudentPath200JSONResponse{}, fmt.Errorf("expected a student path, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return view, nil
}

func (w *world) partsAre(slugs ...string) error {
	parts, err := w.enrollmentParts()
	if err != nil {
		return err
	}
	if len(parts) != len(slugs) {
		return fmt.Errorf("expected %d parts, got %d", len(slugs), len(parts))
	}
	for i, slug := range slugs {
		if parts[i].SourceTemplateId != pathID(slug) {
			return fmt.Errorf("expected part %d to be a copy of %q, got %s", i+1, slug, parts[i].SourceTemplateId)
		}
	}
	return nil
}

func (w *world) itemsMatchTemplate(items []generated.StudentPathItem, slug string) error {
	template, err := w.paths.GetByID(context.Background(), pathID(slug).String())
	if err != nil {
		return err
	}
	if len(items) != len(template.Items) {
		return fmt.Errorf("expected %d items, got %d", len(template.Items), len(items))
	}
	for i, item := range template.Items {
		if items[i].ContentNodeId.String() != item.ContentNodeID {
			return fmt.Errorf("expected item %d to be %s, got %s", i+1, item.ContentNodeID, items[i].ContentNodeId)
		}
	}
	return nil
}

func itemsAllHaveStatus(items []generated.StudentPathItem, status string) error {
	for _, item := range items {
		if string(item.Status) != status {
			return fmt.Errorf("expected every item %s, got %s for %s", status, item.Status, item.ContentNodeId)
		}
	}
	return nil
}
