//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerPathEnrollmentSteps(sc *godog.ScenarioContext, w *world) {
	// ── Seeding ─────────────────────────────────────────────────────────────
	sc.Step(`^a learning path "([^"]+)" exists, published, created by "([^"]+)", with a summary, level "([^"]+)" and a thumbnail$`, w.putPresentedPath)
	sc.Step(`^a learning path "([^"]+)" exists, created by "([^"]+)", with a summary, level "([^"]+)" and a thumbnail$`, w.putPresentedPath)
	sc.Step(`^learning path "([^"]+)" has a summary, level "([^"]+)", a thumbnail and was created by "([^"]+)"$`, func(slug, level, creator string) error {
		return w.putPresentedPath(slug, creator, level)
	})
	sc.Step(`^a learning path "([^"]+)" exists with summary "([^"]+)"$`, func(slug, summary string) error {
		return w.putPublishedPathEdited(slug, func(p *domain.LearningPath) { p.Summary = &summary })
	})

	// ── Enrolling ───────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" enrolls in learning path "([^"]+)"$`, w.enrollsInPath)
	sc.Step(`^"([^"]+)" has enrolled in learning path "([^"]+)"$`, w.hasEnrolledIn)
	sc.Step(`^student "([^"]+)" has enrolled in "([^"]+)"$`, w.hasEnrolledIn)
	sc.Step(`^"([^"]+)" enrolls in a learning path ID that does not exist$`, func(name string) error {
		return w.enrollWithBody(name, &generated.EnrollInLearningPathRequest{LearningPathId: uuid.New()})
	})
	sc.Step(`^"([^"]+)" submits a path enrollment request with the learning_path_id field omitted$`, func(name string) error {
		return w.enrollWithBody(name, &generated.EnrollInLearningPathRequest{})
	})
	sc.Step(`^an unauthenticated request attempts to enroll in learning path "([^"]+)"$`, func(slug string) error {
		resp, err := w.handler.EnrollInLearningPath(context.Background(), generated.EnrollInLearningPathRequestObject{Body: &generated.EnrollInLearningPathRequest{LearningPathId: pathID(slug)}})
		w.lastResp, w.lastErr = resp, err
		return nil
	})
	sc.Step(`^"([^"]+)" has assigned "([^"]+)" to student "([^"]+)"$`, func(teacher, slug, name string) error {
		w.ensureRegistered(teacher, domain.RoleTeacher)
		resp, err := w.handler.AssignLearningPath(w.identityCtx(teacher), generated.AssignLearningPathRequestObject{
			StudentId: w.ensureRegistered(name, domain.RoleStudent),
			Body:      &generated.AssignLearningPathRequest{LearningPathId: pathID(slug)},
		})
		w.lastResp, w.lastErr = resp, err
		if err != nil {
			return err
		}
		if _, err := w.returnedCopy(); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
		return nil
	})
	sc.Step(`^"([^"]+)" has since switched her current path to another path$`, w.switchedToAnotherPath)
	sc.Step(`^"([^"]+)" has archived her copy of "([^"]+)"$`, w.archivedHerCopy)
	sc.Step(`^student "([^"]+)" is enrolled in "([^"]+)" and it is her current course$`, w.studentEnrolledInCourseSetup)
	sc.Step(`^"([^"]+)" holds a standalone path "([^"]+)" that is her current path$`, func(name, slug string) error {
		if err := w.ensurePaths([]string{slug}); err != nil {
			return err
		}
		return w.hasStandalonePathAssigned(name, slug)
	})
	sc.Step(`^"([^"]+)" has completed the first lesson of "([^"]+)" in that course$`, func(name, slug string) error {
		return w.completeFirstLesson(name, slug)
	})
	sc.Step(`^"([^"]+)" has since added a published lesson "([^"]+)" to learning path "([^"]+)"$`, w.addPublishedLesson)
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" adding a published lesson "([^"]+)"$`, func(name, slug, nodeSlug string) error {
		if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
			return err
		}
		w.autoPublishNode(nodeSlug)
		return w.replaceSeededPath(name, slug, func(body *generated.ReplaceLearningPathRequest) {
			body.Items = append(body.Items, replaceItem(nodeSlug))
		})
	})

	// ── Outcomes ────────────────────────────────────────────────────────────
	sc.Step(`^a new student path is created and returned, copied from "([^"]+)"$`, func(slug string) error {
		return w.enrollmentCreated(slug, true)
	})
	sc.Step(`^a new standalone student path is created, copied from "([^"]+)"$`, func(slug string) error {
		if err := w.enrollmentCreated(slug, true); err != nil {
			return err
		}
		card, _ := w.returnedCopy()
		if card.SourceCourseEnrollmentId != nil {
			return fmt.Errorf("expected a standalone copy, got one belonging to enrollment %s", card.SourceCourseEnrollmentId)
		}
		return nil
	})
	sc.Step(`^the student path records "([^"]+)" as both the owner and the assigner$`, func(name string) error {
		card, err := w.returnedCopy()
		if err != nil {
			return err
		}
		want := w.userMotifID[name]
		if card.Student.UserId != want || card.AssignedBy.UserId != want {
			return fmt.Errorf("expected %q as owner and assigner, got %s and %s", name, card.Student.UserId, card.AssignedBy.UserId)
		}
		return nil
	})
	sc.Step(`^the new copy of "([^"]+)" becomes "([^"]+)"'s current path$`, func(_, name string) error {
		return w.returnedCopyIsCurrent(name)
	})
	sc.Step(`^the student path records the summary, level "([^"]+)", thumbnail and creator "([^"]+)" of "([^"]+)"$`, func(level, creator, slug string) error {
		card, err := w.returnedCopy()
		if err != nil {
			return err
		}
		return w.cardRecordsPresentation(card, level, creator)
	})
	sc.Step(`^"([^"]+)"'s enrollment in "([^"]+)" is still active at the same checkpoint$`, w.enrollmentStillActiveAtCheckpoint1)
	sc.Step(`^"([^"]+)"'s copy of "([^"]+)" still exists and is not archived$`, func(name, slug string) error {
		return w.copyState(name, slug, false)
	})
	sc.Step(`^"([^"]+)"'s copy of "([^"]+)" still exists and is still her current path$`, w.copyStillCurrent)
	sc.Step(`^the first lesson of her new copy of "([^"]+)" shows as completed$`, w.firstLessonShowsCompleted)
	sc.Step(`^her existing copy of "([^"]+)" is returned and no new copy is created$`, w.existingCopyReturned)
	sc.Step(`^"([^"]+)"'s existing copy of "([^"]+)" is returned and no new copy is created$`, func(_, slug string) error {
		return w.existingCopyReturned(slug)
	})
	sc.Step(`^her existing copy of "([^"]+)" becomes her current path$`, func(slug string) error {
		return w.returnedCopyIsCurrent(w.lastEnroller)
	})
	sc.Step(`^"([^"]+)"'s existing copy of "([^"]+)" becomes her current path$`, func(name, _ string) error {
		return w.returnedCopyIsCurrent(name)
	})
	sc.Step(`^her archived copy of "([^"]+)" stays archived$`, func(slug string) error {
		return w.copyState(w.lastEnroller, slug, true)
	})
	sc.Step(`^"([^"]+)"'s archived copy of "([^"]+)" stays archived$`, func(name, slug string) error {
		return w.copyState(name, slug, true)
	})
	sc.Step(`^exactly one active copy of "([^"]+)" is listed$`, w.exactlyOneActiveCopyListed)
	sc.Step(`^her new copy of "([^"]+)" includes "([^"]+)"$`, func(_, nodeSlug string) error {
		return w.returnedCopyIncludes(nodeSlug)
	})
	sc.Step(`^a learner who enrolls in "([^"]+)" afterwards gets a copy that includes "([^"]+)"$`, func(slug, nodeSlug string) error {
		w.ensureRegistered("later-learner", domain.RoleStudent)
		if err := w.enrollsInPath("later-learner", slug); err != nil {
			return err
		}
		return w.returnedCopyIncludes(nodeSlug)
	})

	// ── Listing a learner's paths ───────────────────────────────────────────
	sc.Step(`^"([^"]+)" holds a standalone path copied from "([^"]+)"$`, func(name, slug string) error {
		return w.hasStandalonePathAssigned(name, slug)
	})
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" with summary "([^"]+)"$`, func(name, slug, summary string) error {
		return w.replaceSeededPath(name, slug, func(body *generated.ReplaceLearningPathRequest) { body.Summary = &summary })
	})
	sc.Step(`^learning path "([^"]+)" has been deleted$`, func(slug string) error {
		return w.paths.Delete(context.Background(), pathID(slug).String())
	})
	sc.Step(`^"([^"]+)" holds a standalone path "([^"]+)" copied before presentations were recorded$`, w.holdsCopyWithoutSnapshots)
	sc.Step(`^"([^"]+)" holds a standalone path "([^"]+)" with (\d+) lessons$`, func(name, slug string, lessons int) error {
		if err := w.putPublishedPathWithLessons(slug, "bob", lessons); err != nil {
			return err
		}
		return w.hasStandalonePathAssigned(name, slug)
	})
	sc.Step(`^"([^"]+)" has completed (\d+) of those lessons$`, w.completedLessonsOfLastCopy)
	sc.Step(`^"([^"]+)" holds a standalone path "([^"]+)" whose first lesson also appears in course "([^"]+)"$`, w.holdsPathSharingLessonWithCourse)
	sc.Step(`^"([^"]+)" has completed that lesson in "([^"]+)"$`, func(name, _ string) error {
		return w.completeFirstLesson(name, w.sharedLessonPath)
	})
	sc.Step(`^her copy of "([^"]+)" reports the summary, level "([^"]+)", thumbnail and creator "([^"]+)" it was copied with$`, func(slug, level, creator string) error {
		card, err := w.listedCopy(slug)
		if err != nil {
			return err
		}
		return w.cardRecordsPresentation(card, level, creator)
	})
	sc.Step(`^her copy of "([^"]+)" still reports summary "([^"]+)"$`, func(slug, summary string) error {
		card, err := w.listedCopy(slug)
		if err != nil {
			return err
		}
		if card.Summary == nil || *card.Summary != summary {
			return fmt.Errorf("expected summary %q, got %v", summary, card.Summary)
		}
		return nil
	})
	sc.Step(`^her copy of "([^"]+)" reports its title and no summary, level, thumbnail or creator$`, func(slug string) error {
		card, err := w.listedCopy(slug)
		if err != nil {
			return err
		}
		if card.Title == "" || card.Summary != nil || card.Level != nil || card.ThumbnailUrl != nil || card.CreatedBy != nil {
			return fmt.Errorf("expected a title-only copy, got %#v", card)
		}
		return nil
	})
	sc.Step(`^her copy of "([^"]+)" reports (\d+) lessons and (\d+) completed$`, func(slug string, lessons, completed int) error {
		card, err := w.listedCopy(slug)
		if err != nil {
			return err
		}
		if card.LessonCount != lessons || card.CompletedCount != completed {
			return fmt.Errorf("expected %d lessons and %d completed, got %d and %d", lessons, completed, card.LessonCount, card.CompletedCount)
		}
		return nil
	})
	sc.Step(`^her copy of "([^"]+)" reports (\d+) completed$`, func(slug string, completed int) error {
		card, err := w.listedCopy(slug)
		if err != nil {
			return err
		}
		if card.CompletedCount != completed {
			return fmt.Errorf("expected %d completed, got %d", completed, card.CompletedCount)
		}
		return nil
	})

	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)" at level "([^"]+)" with items in order: "([^"]+)"$`, func(name, title, level, nodeSlug string) error {
		return w.createsPathWith(name, title, nil, nil, level, nodeSlug)
	})
}

func (w *world) putPresentedPath(slug, creator, level string) error {
	if err := w.ensurePaths([]string{slug}); err != nil {
		return err
	}
	thumbnail := "https://cdn.motifpath.io/thumbnails/" + slug + ".png"
	summary := "A summary of " + slug
	teacher := w.ensureRegistered(creator, domain.RoleTeacher).String()
	return w.editSeededPath(slug, func(p *domain.LearningPath) {
		l := domain.DifficultyLevel(level)
		p.TeacherID, p.Summary, p.Level, p.ThumbnailURL = teacher, &summary, &l, &thumbnail
		p.Status = domain.LearningPathStatusPublished
	})
}

func (w *world) enrollWithBody(name string, body *generated.EnrollInLearningPathRequest) error {
	w.ensureRegistered(name, domain.RoleStudent)
	w.lastEnroller = name
	resp, err := w.handler.EnrollInLearningPath(w.identityCtx(name), generated.EnrollInLearningPathRequestObject{Body: body})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) enrollsInPath(name, slug string) error {
	if err := w.enrollWithBody(name, &generated.EnrollInLearningPathRequest{LearningPathId: pathID(slug)}); err != nil {
		return err
	}
	if card, err := w.returnedCopy(); err == nil {
		key := name + "|" + slug
		if _, held := w.standalonePathIDByKey[key]; !held {
			w.standalonePathIDByKey[key] = card.StudentPathId.String()
		}
	}
	return nil
}

func (w *world) hasEnrolledIn(name, slug string) error {
	if err := w.enrollsInPath(name, slug); err != nil {
		return err
	}
	if _, err := w.returnedCopy(); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	return nil
}

// returnedCopy is the student path the last enrollment or assignment
// returned, whether it was just created or reused.
func (w *world) returnedCopy() (generated.StudentPath, error) {
	switch resp := w.lastResp.(type) {
	case generated.EnrollInLearningPath201JSONResponse:
		return generated.StudentPath(resp), nil
	case generated.EnrollInLearningPath200JSONResponse:
		return generated.StudentPath(resp), nil
	case generated.AssignLearningPath201JSONResponse:
		return generated.StudentPath(resp), nil
	case generated.AssignLearningPath200JSONResponse:
		return generated.StudentPath(resp), nil
	default:
		return generated.StudentPath{}, fmt.Errorf("expected a student path, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) enrollmentCreated(slug string, wantCreated bool) error {
	card, err := w.returnedCopy()
	if err != nil {
		return err
	}
	_, isEnroll201 := w.lastResp.(generated.EnrollInLearningPath201JSONResponse)
	_, isAssign201 := w.lastResp.(generated.AssignLearningPath201JSONResponse)
	if created := isEnroll201 || isAssign201; created != wantCreated {
		return fmt.Errorf("expected created=%v, got %#v", wantCreated, w.lastResp)
	}
	if card.SourceTemplateId != pathID(slug) {
		return fmt.Errorf("expected a copy of %q, got one of %s", slug, card.SourceTemplateId)
	}
	return nil
}

func (w *world) returnedCopyIsCurrent(name string) error {
	card, err := w.returnedCopy()
	if err != nil {
		return err
	}
	state, err := w.learningState.GetByStudentID(context.Background(), w.userMotifID[name].String())
	if err != nil {
		return err
	}
	if state.CurrentStandalonePathID == nil || *state.CurrentStandalonePathID != card.StudentPathId.String() {
		return fmt.Errorf("expected %q's current path to be %s, got %+v", name, card.StudentPathId, state)
	}
	return nil
}

func (w *world) cardRecordsPresentation(card generated.StudentPath, level, creator string) error {
	if card.Summary == nil || card.ThumbnailUrl == nil || card.Level == nil || string(*card.Level) != level {
		return fmt.Errorf("expected a summary, a thumbnail and level %q, got %#v", level, card)
	}
	if want := w.ensureRegistered(creator, domain.RoleTeacher); card.CreatedBy == nil || card.CreatedBy.UserId != want {
		return fmt.Errorf("expected creator %q, got %#v", creator, card.CreatedBy)
	}
	return nil
}

func (w *world) enrollmentStillActiveAtCheckpoint1(name, courseSlug string) error {
	id, ok := w.courseEnrollmentIDByKey[courseEnrollKey(name, courseSlug)]
	if !ok {
		return fmt.Errorf("no enrollment recorded for %q in %q", name, courseSlug)
	}
	enrollment, err := w.courseEnrollments.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	if !enrollment.IsActive() || enrollment.ActiveCheckpointPosition == nil || *enrollment.ActiveCheckpointPosition != 1 {
		return fmt.Errorf("expected the enrollment still active at checkpoint 1, got %+v", enrollment)
	}
	return nil
}

func (w *world) heldCopy(name, slug string) (domain.StudentPath, error) {
	id, ok := w.standalonePathIDByKey[name+"|"+slug]
	if !ok {
		return domain.StudentPath{}, fmt.Errorf("no copy of %q recorded for %q", slug, name)
	}
	return w.studentPaths.GetByID(context.Background(), id)
}

func (w *world) copyState(name, slug string, wantArchived bool) error {
	sp, err := w.heldCopy(name, slug)
	if err != nil {
		return err
	}
	if archived := sp.ArchivedAt != nil; archived != wantArchived {
		return fmt.Errorf("expected %q's copy of %q archived=%v, got archived_at %v", name, slug, wantArchived, sp.ArchivedAt)
	}
	return nil
}

func (w *world) copyStillCurrent(name, slug string) error {
	if err := w.copyState(name, slug, false); err != nil {
		return err
	}
	sp, err := w.heldCopy(name, slug)
	if err != nil {
		return err
	}
	state, err := w.learningState.GetByStudentID(context.Background(), sp.StudentID)
	if err != nil {
		return err
	}
	if state.CurrentStandalonePathID == nil || *state.CurrentStandalonePathID != sp.ID {
		return fmt.Errorf("expected %q's copy of %q to be current, got %+v", name, slug, state)
	}
	return nil
}

func (w *world) switchedToAnotherPath(name string) error {
	if err := w.ensurePaths([]string{"another-path"}); err != nil {
		return err
	}
	return w.hasStandalonePathAssigned(name, "another-path")
}

func (w *world) archivedHerCopy(name, slug string) error {
	sp, err := w.heldCopy(name, slug)
	if err != nil {
		return err
	}
	return w.studentPaths.Archive(context.Background(), sp.ID, fixedNow)
}

// completeFirstLesson records the path's first lesson completed for name —
// progress is per content node, so it counts in every path that holds it.
func (w *world) completeFirstLesson(name, slug string) error {
	path, err := w.seededPath(slug)
	if err != nil {
		return err
	}
	w.completion.set(w.ensureRegistered(name, domain.RoleStudent).String(), path.Items[0].ContentNodeID, domain.CompletionStatusCompleted)
	return nil
}

func (w *world) firstLessonShowsCompleted(slug string) error {
	card, err := w.returnedCopy()
	if err != nil {
		return err
	}
	if err := w.switchCurrentTo(w.lastEnroller, card.StudentPathId); err != nil {
		return err
	}
	view, ok := w.lastResp.(generated.SetCurrentPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected the copy's path view, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(view.Items) == 0 || view.Items[0].Status != generated.StudentPathItemStatusCompleted {
		return fmt.Errorf("expected the first lesson of %q completed, got %#v", slug, view.Items)
	}
	return nil
}

func (w *world) switchCurrentTo(name string, studentPathID uuid.UUID) error {
	resp, err := w.handler.SetCurrentPath(w.identityCtx(name), generated.SetCurrentPathRequestObject{Body: &generated.SetCurrentPathRequest{StudentPathId: &studentPathID}})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) existingCopyReturned(slug string) error {
	if err := w.enrollmentCreated(slug, false); err != nil {
		return err
	}
	card, err := w.returnedCopy()
	if err != nil {
		return err
	}
	earlier, err := w.heldCopy(w.userNameByID(card.Student.UserId), slug)
	if err != nil {
		return err
	}
	if earlier.ID != card.StudentPathId.String() {
		return fmt.Errorf("expected the existing copy %s, got %s", earlier.ID, card.StudentPathId)
	}
	return nil
}

// userNameByID is the persona registered with id.
func (w *world) userNameByID(id uuid.UUID) string {
	for name, registered := range w.userMotifID {
		if registered == id {
			return name
		}
	}
	return ""
}

func (w *world) exactlyOneActiveCopyListed(slug string) error {
	cards, ok := w.lastResp.(generated.ListMyStandalonePaths200JSONResponse)
	if !ok {
		return fmt.Errorf("expected the learner's standalone paths, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	active := 0
	for _, card := range cards {
		if card.SourceTemplateId == pathID(slug) && card.ArchivedAt == nil {
			active++
		}
	}
	if active != 1 {
		return fmt.Errorf("expected exactly one active copy of %q, got %d", slug, active)
	}
	return nil
}

func (w *world) addPublishedLesson(_, nodeSlug, slug string) error {
	if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
		return err
	}
	w.autoPublishNode(nodeSlug)
	return w.editSeededPath(slug, func(p *domain.LearningPath) {
		p.Items = append(p.Items, domain.LearningPathItem{Position: len(p.Items) + 1, ContentNodeID: nodeID(nodeSlug).String(), Title: nodeSlug, ContentType: domain.ContentTypeVideo})
	})
}

func (w *world) returnedCopyIncludes(nodeSlug string) error {
	card, err := w.returnedCopy()
	if err != nil {
		return err
	}
	sp, err := w.studentPaths.GetByID(context.Background(), card.StudentPathId.String())
	if err != nil {
		return err
	}
	for _, item := range sp.Items {
		if item.ContentNodeID == nodeID(nodeSlug).String() {
			return nil
		}
	}
	return fmt.Errorf("expected the copy to include %q", nodeSlug)
}

func (w *world) listedCopy(slug string) (generated.StudentPath, error) {
	cards, ok := w.lastResp.(generated.ListMyStandalonePaths200JSONResponse)
	if !ok {
		return generated.StudentPath{}, fmt.Errorf("expected the learner's standalone paths, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, card := range cards {
		if card.SourceTemplateId == pathID(slug) {
			return card, nil
		}
	}
	return generated.StudentPath{}, fmt.Errorf("expected a copy of %q to be listed", slug)
}

// holdsCopyWithoutSnapshots seeds a copy made before copies recorded their
// template's presentation.
func (w *world) holdsCopyWithoutSnapshots(name, slug string) error {
	if err := w.ensurePaths([]string{slug}); err != nil {
		return err
	}
	if err := w.hasStandalonePathAssigned(name, slug); err != nil {
		return err
	}
	sp, err := w.heldCopy(name, slug)
	if err != nil {
		return err
	}
	sp.SummarySnapshot, sp.LevelSnapshot, sp.ThumbnailURLSnapshot, sp.CreatedBySnapshot = nil, nil, nil, nil
	w.studentPaths.put(sp)
	return nil
}

func (w *world) completedLessonsOfLastCopy(name string, count int) error {
	id := w.priorStudentPathID
	sp, err := w.studentPaths.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	for i := 0; i < count && i < len(sp.Items); i++ {
		w.completion.set(studentID, sp.Items[i].ContentNodeID, domain.CompletionStatusCompleted)
	}
	return nil
}

// holdsPathSharingLessonWithCourse seeds a standalone copy of slug for name
// and a published course whose only checkpoint path starts with the same
// lesson.
func (w *world) holdsPathSharingLessonWithCourse(name, slug, courseSlug string) error {
	if err := w.putPublishedPathWithLessons(slug, "bob", 2); err != nil {
		return err
	}
	path, err := w.seededPath(slug)
	if err != nil {
		return err
	}
	coursePath := courseSlug + "-path"
	if err := w.putPublishedPathWithLessons(coursePath, "bob", 1); err != nil {
		return err
	}
	if err := w.editSeededPath(coursePath, func(p *domain.LearningPath) { p.Items[0] = path.Items[0] }); err != nil {
		return err
	}
	if err := w.courseExistsPublishedWithCheckpoint(courseSlug, coursePath); err != nil {
		return err
	}
	w.sharedLessonPath = slug
	return w.hasStandalonePathAssigned(name, slug)
}
