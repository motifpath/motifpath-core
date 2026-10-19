//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// updateCopyWorld is what the update-a-copy steps remember between steps:
// the lesson an author last added, and the copy or enrollment as it was
// just before the learner updated it.
type updateCopyWorld struct {
	addedLesson         string
	copyBeforeUpdate    domain.StudentPath
	activePartBeforeID  string
	activePartBeforePos int
}

func registerUpdateCopySteps(sc *godog.ScenarioContext, w *world) {
	u := &updateCopyWorld{}

	// ── Seeding ─────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" has enrolled in learning path "([^"]+)" and it is her current path$`, w.hasEnrolledIn)
	sc.Step(`^student "([^"]+)" has enrolled in learning path "([^"]+)"$`, w.hasEnrolledIn)
	sc.Step(`^"([^"]+)" has enrolled in learning path "([^"]+)" with lessons "([^"]+)", "([^"]+)", "([^"]+)"$`, func(name, slug, l1, l2, l3 string) error {
		if err := w.setPathLessons(slug, l1, l2, l3); err != nil {
			return err
		}
		return w.hasEnrolledIn(name, slug)
	})
	sc.Step(`^"([^"]+)" has completed the first lesson of her copy of "([^"]+)"$`, w.completeFirstLesson)
	sc.Step(`^"([^"]+)" has completed every lesson of her copy of "([^"]+)"$`, func(name, slug string) error {
		sp, err := w.heldCopy(name, slug)
		if err != nil {
			return err
		}
		return w.completeItems(name, sp.Items)
	})
	sc.Step(`^"([^"]+)" has completed every lesson of "([^"]+)"$`, func(name, slug string) error {
		path, err := w.seededPath(slug)
		if err != nil {
			return err
		}
		for _, item := range path.Items {
			w.completion.set(w.ensureRegistered(name, domain.RoleStudent).String(), item.ContentNodeID, domain.CompletionStatusCompleted)
		}
		return nil
	})
	sc.Step(`^a course "([^"]+)" exists, published, with checkpoints "([^"]+)", "([^"]+)", "([^"]+)"$`, func(courseSlug, p1, p2, p3 string) error {
		_, err := w.seedCourse(courseSlug, []string{p1, p2, p3}, "bob", true)
		return err
	})
	sc.Step(`^"([^"]+)" is enrolled in "([^"]+)" with checkpoint 2 active$`, func(name, courseSlug string) error {
		return w.enrollAndAdvanceToCheckpoint(name, courseSlug, 2)
	})
	sc.Step(`^"([^"]+)" has abandoned her enrollment in "([^"]+)"$`, w.enrolledThenAbandoned)
	sc.Step(`^"([^"]+)"'s current path is the standalone path "([^"]+)"$`, func(name, slug string) error {
		if err := w.ensurePaths([]string{slug}); err != nil {
			return err
		}
		return w.hasStandalonePathAssigned(name, slug)
	})

	// ── What authors and admins change ──────────────────────────────────────
	sc.Step(`^the author adds a lesson to "([^"]+)"$`, func(slug string) error {
		u.addedLesson = slug + "-added-lesson"
		return w.addPublishedLesson("", u.addedLesson, slug)
	})
	sc.Step(`^the author adds the lesson "([^"]+)" to "([^"]+)"$`, func(lesson, slug string) error {
		u.addedLesson = lesson
		return w.addPublishedLesson("", lesson, slug)
	})
	sc.Step(`^the author removes "([^"]+)" from "([^"]+)"$`, func(lesson, slug string) error {
		return w.editSeededPath(slug, func(p *domain.LearningPath) {
			p.Items = slices.DeleteFunc(p.Items, func(item domain.LearningPathItem) bool { return item.ContentNodeID == nodeID(lesson).String() })
			for i := range p.Items {
				p.Items[i].Position = i + 1
			}
		})
	})
	sc.Step(`^the author changes the thumbnail of "([^"]+)"$`, func(slug string) error {
		thumbnail := "https://cdn.motifpath.io/thumbnails/" + slug + "-new.png"
		return w.editSeededPath(slug, func(p *domain.LearningPath) { p.ThumbnailURL = &thumbnail })
	})
	sc.Step(`^the author changes the title, summary, level and thumbnail of "([^"]+)"$`, func(slug string) error {
		summary, thumbnail, level := "A new summary of "+slug, "https://cdn.motifpath.io/thumbnails/"+slug+"-new.png", domain.DifficultyLevelIntermediate
		return w.editSeededPath(slug, func(p *domain.LearningPath) {
			p.Title, p.Summary, p.ThumbnailURL, p.Level = slug+" (revised)", &summary, &thumbnail, &level
		})
	})
	sc.Step(`^an admin unpublishes "([^"]+)"$`, func(slug string) error {
		return w.setPathStatus(slug, domain.LearningPathStatusDraft)
	})
	sc.Step(`^an admin publishes a new version of "([^"]+)"$`, func(courseSlug string) error {
		return w.publishCourseVersion(courseSlug, nil, nil)
	})
	sc.Step(`^an admin publishes a new version of "([^"]+)" with a new title and thumbnail$`, func(courseSlug string) error {
		return w.publishCourseVersion(courseSlug, func(v *domain.CourseVersion) {
			thumbnail := "https://cdn.motifpath.io/thumbnails/" + courseSlug + "-new.png"
			v.TitleSnapshot, v.ThumbnailURLSnapshot = courseSlug+" (revised)", &thumbnail
		}, nil)
	})
	sc.Step(`^an admin publishes a new version of "([^"]+)" with checkpoints "([^"]+)"$`, func(courseSlug, p1 string) error {
		return w.publishCourseVersion(courseSlug, nil, []string{p1})
	})
	sc.Step(`^an admin publishes a new version of "([^"]+)" with checkpoints "([^"]+)", "([^"]+)"$`, func(courseSlug, p1, p2 string) error {
		return w.publishCourseVersion(courseSlug, nil, []string{p1, p2})
	})
	sc.Step(`^an admin publishes a new version of "([^"]+)" with checkpoints "([^"]+)", "([^"]+)", "([^"]+)"$`, func(courseSlug, p1, p2, p3 string) error {
		return w.publishCourseVersion(courseSlug, nil, []string{p1, p2, p3})
	})

	// ── Actions ─────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" lists her standalone paths$`, func(string) error {
		resp, err := w.handler.ListMyStandalonePaths(w.ctx(), generated.ListMyStandalonePathsRequestObject{})
		w.lastResp, w.lastErr = resp, err
		return err
	})
	updatePath := func(owner, slug string) error {
		sp, err := w.heldCopy(owner, slug)
		if err != nil {
			return err
		}
		u.copyBeforeUpdate = sp
		resp, err := w.handler.UpdateStandalonePathToLatest(w.ctx(), generated.UpdateStandalonePathToLatestRequestObject{StudentPathId: uuid.MustParse(sp.ID)})
		w.lastResp, w.lastErr = resp, err
		return err
	}
	sc.Step(`^"([^"]+)" updates her copy of "([^"]+)"$`, updatePath)
	sc.Step(`^"([^"]+)" tries to update her copy of "([^"]+)"$`, updatePath)
	sc.Step(`^"[^"]+" tries to update "([^"]+)"'s copy of "([^"]+)"$`, updatePath)
	updateEnrollment := func(name, courseSlug string) error {
		id, ok := w.courseEnrollmentIDByKey[courseEnrollKey(name, courseSlug)]
		if !ok {
			return fmt.Errorf("no enrollment of %q in %q recorded", name, courseSlug)
		}
		if e, err := w.courseEnrollments.GetByID(context.Background(), id); err == nil && e.ActiveCheckpointStudentPathID != nil {
			u.activePartBeforeID, u.activePartBeforePos = *e.ActiveCheckpointStudentPathID, *e.ActiveCheckpointPosition
		}
		resp, err := w.handler.UpdateCourseEnrollmentToLatest(w.ctx(), generated.UpdateCourseEnrollmentToLatestRequestObject{CourseEnrollmentId: uuid.MustParse(id)})
		w.lastResp, w.lastErr = resp, err
		return err
	}
	sc.Step(`^"([^"]+)" updates her enrollment in "([^"]+)"$`, updateEnrollment)
	sc.Step(`^"([^"]+)" tries to update her enrollment in "([^"]+)"$`, updateEnrollment)

	// ── Outcomes: standalone paths ──────────────────────────────────────────
	sc.Step(`^her copy of "([^"]+)" shows that an update is available$`, func(slug string) error { return w.copyUpdateAvailable(slug, true) })
	sc.Step(`^her copy of "([^"]+)" shows no update available$`, func(slug string) error { return w.copyUpdateAvailable(slug, false) })
	sc.Step(`^her copy of "([^"]+)" contains "([^"]+)"$`, w.copyContains)
	sc.Step(`^her copy of "([^"]+)" contains the new lesson$`, func(slug string) error { return w.copyContains(slug, u.addedLesson) })
	sc.Step(`^her copy of "([^"]+)" records the template's new title, summary, level and thumbnail$`, func(slug string) error {
		card, err := w.updatedPath()
		if err != nil {
			return err
		}
		path, err := w.seededPath(slug)
		if err != nil {
			return err
		}
		if card.Title != path.Title || *card.Summary != *path.Summary || *card.ThumbnailUrl != *path.ThumbnailURL || string(*card.Level) != string(*path.Level) {
			return fmt.Errorf("expected the copy to record %q's new presentation, got %+v", slug, card)
		}
		return nil
	})
	sc.Step(`^the first lesson of her copy of "([^"]+)" shows as completed$`, func(slug string) error {
		view, err := w.copyView(slug)
		if err != nil {
			return err
		}
		if len(view.Items) == 0 || view.Items[0].Status != generated.StudentPathItemStatusCompleted {
			return fmt.Errorf("expected the first lesson completed, got %#v", view.Items)
		}
		return nil
	})
	sc.Step(`^the next lesson of her copy of "([^"]+)" is "([^"]+)"$`, func(slug, lesson string) error {
		view, err := w.copyView(slug)
		if err != nil {
			return err
		}
		for _, item := range view.Items {
			if item.Position == view.CurrentPosition {
				if item.ContentNodeId != nodeID(lesson) {
					return fmt.Errorf("expected the next lesson to be %q, got %s", lesson, item.ContentNodeId)
				}
				return nil
			}
		}
		return fmt.Errorf("no item at the current position %d", view.CurrentPosition)
	})
	sc.Step(`^the updated copy keeps its id$`, func() error {
		card, err := w.updatedPath()
		if err != nil {
			return err
		}
		if card.StudentPathId.String() != u.copyBeforeUpdate.ID {
			return fmt.Errorf("expected id %s, got %s", u.copyBeforeUpdate.ID, card.StudentPathId)
		}
		return nil
	})
	sc.Step(`^it is still "([^"]+)"'s current path$`, func(name string) error {
		state, err := w.learningState.GetByStudentID(context.Background(), w.userMotifID[name].String())
		if err != nil {
			return err
		}
		if state.CurrentStandalonePathID == nil || *state.CurrentStandalonePathID != u.copyBeforeUpdate.ID {
			return fmt.Errorf("expected the updated copy to stay current, got %+v", state)
		}
		return nil
	})
	sc.Step(`^her copy of "([^"]+)" is returned unchanged$`, func(string) error {
		if _, err := w.updatedPath(); err != nil {
			return err
		}
		after, err := w.studentPaths.GetByID(context.Background(), u.copyBeforeUpdate.ID)
		if err != nil {
			return err
		}
		if !slices.Equal(after.Items, u.copyBeforeUpdate.Items) || after.Title != u.copyBeforeUpdate.Title {
			return fmt.Errorf("expected the copy unchanged, got %+v", after)
		}
		return nil
	})
	sc.Step(`^her copy still records "([^"]+)" as the assigner$`, func(assigner string) error {
		card, err := w.updatedPath()
		if err != nil {
			return err
		}
		if card.AssignedBy.UserId != w.userMotifID[assigner] {
			return fmt.Errorf("expected %q as the assigner, got %+v", assigner, card.AssignedBy)
		}
		return nil
	})
	sc.Step(`^the update is refused because no active standalone path was found$`, func() error {
		if _, ok := w.lastResp.(generated.UpdateStandalonePathToLatest404JSONResponse); !ok {
			return fmt.Errorf("expected not found, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return nil
	})
	sc.Step(`^the update is refused because the path is finished$`, func() error {
		return w.updateRefusedAsConflict()
	})
	sc.Step(`^the update is refused because the path is no longer published$`, func() error {
		return w.updateRefusedAsConflict()
	})

	// ── Outcomes: course enrollments ────────────────────────────────────────
	sc.Step(`^her enrollment in "([^"]+)" shows that an update is available$`, func(courseSlug string) error {
		list, ok := w.lastResp.(generated.ListMyCourseEnrollments200JSONResponse)
		if !ok {
			return fmt.Errorf("expected her course enrollments, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		for _, e := range list {
			if e.CourseId == w.courseIDBySlug[courseSlug] {
				if !e.UpdateAvailable {
					return fmt.Errorf("expected an update available for %q", courseSlug)
				}
				return nil
			}
		}
		return fmt.Errorf("no enrollment in %q listed", courseSlug)
	})
	sc.Step(`^her enrollment is pinned to the course's latest published version$`, func() error {
		e, err := w.updatedEnrollment()
		if err != nil {
			return err
		}
		latest, err := w.courseVersions.GetLatestByCourseID(context.Background(), e.CourseId.String())
		if err != nil {
			return err
		}
		if e.CourseVersionNumber != latest.VersionNumber {
			return fmt.Errorf("expected version %d, got %d", latest.VersionNumber, e.CourseVersionNumber)
		}
		return nil
	})
	sc.Step(`^her enrollment shows the new title and thumbnail$`, func() error {
		e, err := w.updatedEnrollment()
		if err != nil {
			return err
		}
		latest, err := w.courseVersions.GetLatestByCourseID(context.Background(), e.CourseId.String())
		if err != nil {
			return err
		}
		if e.CourseTitle != latest.TitleSnapshot || e.CourseThumbnailUrl == nil || *e.CourseThumbnailUrl != *latest.ThumbnailURLSnapshot {
			return fmt.Errorf("expected the new title and thumbnail, got %+v", e)
		}
		return nil
	})
	sc.Step(`^her enrollment shows no update available$`, func() error {
		e, err := w.updatedEnrollment()
		if err != nil {
			return err
		}
		if e.UpdateAvailable {
			return fmt.Errorf("expected no update available, got %+v", e)
		}
		return nil
	})
	sc.Step(`^her active checkpoint is "([^"]+)" at position (\d+)$`, func(slug string, position int) error {
		return w.activePartIs(slug, position)
	})
	sc.Step(`^a student path for "([^"]+)" is created and becomes her active checkpoint at position (\d+)$`, func(slug string, position int) error {
		return w.activePartIs(slug, position)
	})
	sc.Step(`^her student path for "([^"]+)" is the same copy as before$`, func(string) error {
		e, err := w.updatedEnrollment()
		if err != nil {
			return err
		}
		if e.ActiveCheckpointStudentPathId == nil || e.ActiveCheckpointStudentPathId.String() != u.activePartBeforeID {
			return fmt.Errorf("expected the same copy %s, got %v", u.activePartBeforeID, e.ActiveCheckpointStudentPathId)
		}
		return nil
	})
	sc.Step(`^her student path for checkpoint (\d+) contains "([^"]+)"$`, func(position int, lesson string) error {
		part, err := w.activePart()
		if err != nil {
			return err
		}
		if part.CourseCheckpointPosition == nil || *part.CourseCheckpointPosition != position {
			return fmt.Errorf("expected the active part at checkpoint %d, got %v", position, part.CourseCheckpointPosition)
		}
		return itemsInclude(part.Items, lesson)
	})
	sc.Step(`^her student path for "([^"]+)" is archived$`, func(slug string) error {
		before, err := w.studentPaths.GetByID(context.Background(), u.activePartBeforeID)
		if err != nil {
			return err
		}
		if before.SourceTemplateID != pathID(slug).String() || before.ArchivedAt == nil {
			return fmt.Errorf("expected her copy of %q archived, got %+v", slug, before)
		}
		return nil
	})
	sc.Step(`^her enrollment in "([^"]+)" is completed$`, func(string) error {
		e, err := w.updatedEnrollment()
		if err != nil {
			return err
		}
		if e.Status != generated.CourseEnrollmentStatusCompleted {
			return fmt.Errorf("expected the enrollment completed, got %s", e.Status)
		}
		return nil
	})
	sc.Step(`^"([^"]+)"'s current path is still "([^"]+)"$`, func(name, slug string) error {
		state, err := w.learningState.GetByStudentID(context.Background(), w.userMotifID[name].String())
		if err != nil {
			return err
		}
		if state.CurrentStandalonePathID == nil {
			return fmt.Errorf("expected %q to be current, got %+v", slug, state)
		}
		sp, err := w.studentPaths.GetByID(context.Background(), *state.CurrentStandalonePathID)
		if err != nil {
			return err
		}
		if sp.SourceTemplateID != pathID(slug).String() {
			return fmt.Errorf("expected %q to be current, got a copy of %s", slug, sp.SourceTemplateID)
		}
		return nil
	})
	sc.Step(`^the update is refused because no active enrollment was found$`, func() error {
		if _, ok := w.lastResp.(generated.UpdateCourseEnrollmentToLatest404JSONResponse); !ok {
			return fmt.Errorf("expected not found, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return nil
	})

	// ── Outcomes: advancing on the pinned version ───────────────────────────
	sc.Step(`^a student path for "([^"]+)" is created as checkpoint (\d+)$`, func(slug string, position int) error {
		return w.lastEnrollmentActivePartIs(slug, position)
	})
	sc.Step(`^the enrollment is still pinned to the version she enrolled in$`, func() error {
		e, err := w.courseEnrollments.GetByID(context.Background(), w.courseEnrollmentIDByKey[w.lastCourseEnrollmentKey])
		if err != nil {
			return err
		}
		if e.CourseVersionNumber != 1 {
			return fmt.Errorf("expected the enrollment pinned to version 1, got %d", e.CourseVersionNumber)
		}
		return nil
	})
}

// setPathLessons gives slug exactly the published lessons named, in order.
func (w *world) setPathLessons(slug string, lessons ...string) error {
	items := make([]domain.LearningPathItem, len(lessons))
	for i, lesson := range lessons {
		if err := w.putContentNode(lesson, domain.ContentTypeVideo); err != nil {
			return err
		}
		w.autoPublishNode(lesson)
		items[i] = domain.LearningPathItem{Position: i + 1, ContentNodeID: nodeID(lesson).String(), Title: lesson, ContentType: domain.ContentTypeVideo}
	}
	return w.editSeededPath(slug, func(p *domain.LearningPath) { p.Items = items })
}

func (w *world) completeItems(name string, items []domain.StudentPathItemRecord) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent).String()
	for _, item := range items {
		w.completion.set(studentID, item.ContentNodeID, domain.CompletionStatusCompleted)
	}
	return nil
}

// publishCourseVersion publishes courseSlug's next version straight into
// the store: the latest version, edited, over pathSlugs when given.
func (w *world) publishCourseVersion(courseSlug string, edit func(*domain.CourseVersion), pathSlugs []string) error {
	courseID, ok := w.courseIDBySlug[courseSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", courseSlug)
	}
	latest, err := w.courseVersions.GetLatestByCourseID(context.Background(), courseID.String())
	if err != nil {
		return err
	}
	next := latest
	next.ID = uuid.NewString()
	next.VersionNumber = latest.VersionNumber + 1
	if pathSlugs != nil {
		if err := w.ensurePaths(pathSlugs); err != nil {
			return err
		}
		next.Checkpoints = make([]domain.CourseVersionCheckpoint, len(pathSlugs))
		for i, slug := range pathSlugs {
			next.Checkpoints[i] = domain.CourseVersionCheckpoint{Position: i + 1, LearningPathID: pathID(slug).String(), EffectiveTitle: slug}
		}
	}
	if edit != nil {
		edit(&next)
	}
	return w.courseVersions.Create(context.Background(), next)
}

// copyUpdateAvailable checks slug's copy in the learner's standalone paths:
// the last listing, or a fresh one after an update.
func (w *world) copyUpdateAvailable(slug string, want bool) error {
	list, ok := w.lastResp.(generated.ListMyStandalonePaths200JSONResponse)
	if !ok {
		resp, err := w.handler.ListMyStandalonePaths(w.ctx(), generated.ListMyStandalonePathsRequestObject{})
		if err != nil {
			return err
		}
		if list, ok = resp.(generated.ListMyStandalonePaths200JSONResponse); !ok {
			return fmt.Errorf("expected her standalone paths, got %#v", resp)
		}
	}
	for _, card := range list {
		if card.SourceTemplateId == pathID(slug) && card.ArchivedAt == nil {
			if card.UpdateAvailable != want {
				return fmt.Errorf("expected update_available=%v for %q, got %v", want, slug, card.UpdateAvailable)
			}
			return nil
		}
	}
	return fmt.Errorf("no active copy of %q listed", slug)
}

func (w *world) updatedPath() (generated.StudentPath, error) {
	resp, ok := w.lastResp.(generated.UpdateStandalonePathToLatest200JSONResponse)
	if !ok {
		return generated.StudentPath{}, fmt.Errorf("expected the updated path, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return generated.StudentPath(resp), nil
}

func (w *world) updatedEnrollment() (generated.CourseEnrollment, error) {
	resp, ok := w.lastResp.(generated.UpdateCourseEnrollmentToLatest200JSONResponse)
	if !ok {
		return generated.CourseEnrollment{}, fmt.Errorf("expected the updated enrollment, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return generated.CourseEnrollment(resp), nil
}

func (w *world) copyContains(slug, lesson string) error {
	card, err := w.updatedPath()
	if err != nil {
		return err
	}
	if card.SourceTemplateId != pathID(slug) {
		return fmt.Errorf("expected a copy of %q, got one of %s", slug, card.SourceTemplateId)
	}
	sp, err := w.studentPaths.GetByID(context.Background(), card.StudentPathId.String())
	if err != nil {
		return err
	}
	return itemsInclude(sp.Items, lesson)
}

func itemsInclude(items []domain.StudentPathItemRecord, lesson string) error {
	for _, item := range items {
		if item.ContentNodeID == nodeID(lesson).String() {
			return nil
		}
	}
	return fmt.Errorf("expected the copy to include %q", lesson)
}

// copyView switches the learner onto her copy of slug and returns its view.
func (w *world) copyView(slug string) (generated.SetCurrentPath200JSONResponse, error) {
	sp, err := w.heldCopy(w.persona, slug)
	if err != nil {
		return generated.SetCurrentPath200JSONResponse{}, err
	}
	if err := w.switchCurrentTo(w.persona, uuid.MustParse(sp.ID)); err != nil {
		return generated.SetCurrentPath200JSONResponse{}, err
	}
	view, ok := w.lastResp.(generated.SetCurrentPath200JSONResponse)
	if !ok {
		return generated.SetCurrentPath200JSONResponse{}, fmt.Errorf("expected the copy's path view, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return view, nil
}

func (w *world) activePart() (domain.StudentPath, error) {
	e, err := w.updatedEnrollment()
	if err != nil {
		return domain.StudentPath{}, err
	}
	if e.ActiveCheckpointStudentPathId == nil {
		return domain.StudentPath{}, fmt.Errorf("expected an active checkpoint, got %+v", e)
	}
	return w.studentPaths.GetByID(context.Background(), e.ActiveCheckpointStudentPathId.String())
}

func (w *world) activePartIs(slug string, position int) error {
	e, err := w.updatedEnrollment()
	if err != nil {
		return err
	}
	part, err := w.activePart()
	if err != nil {
		return err
	}
	if part.SourceTemplateID != pathID(slug).String() || e.ActiveCheckpointPosition == nil || *e.ActiveCheckpointPosition != position {
		return fmt.Errorf("expected %q active at position %d, got a copy of %s at %v", slug, position, part.SourceTemplateID, e.ActiveCheckpointPosition)
	}
	return nil
}

func (w *world) lastEnrollmentActivePartIs(slug string, position int) error {
	if err := w.triggerCheckpointDiscoveryForLastEnrollment(); err != nil {
		return err
	}
	e, err := w.courseEnrollments.GetByID(context.Background(), w.courseEnrollmentIDByKey[w.lastCourseEnrollmentKey])
	if err != nil {
		return err
	}
	if e.ActiveCheckpointStudentPathID == nil || e.ActiveCheckpointPosition == nil || *e.ActiveCheckpointPosition != position {
		return fmt.Errorf("expected an active checkpoint at %d, got %+v", position, e)
	}
	part, err := w.studentPaths.GetByID(context.Background(), *e.ActiveCheckpointStudentPathID)
	if err != nil {
		return err
	}
	if part.SourceTemplateID != pathID(slug).String() {
		return fmt.Errorf("expected a copy of %q, got one of %s", slug, part.SourceTemplateID)
	}
	return nil
}

func (w *world) updateRefusedAsConflict() error {
	if _, ok := w.lastResp.(generated.UpdateStandalonePathToLatest409JSONResponse); !ok {
		return fmt.Errorf("expected a conflict, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}
