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

// courseCheckpointBody is the anonymous struct type generated for both
// CreateCourseRequest.Checkpoints and ReplaceCourseRequest.Checkpoints —
// structurally identical (same field names, types, and tags), so a single
// alias here builds either request body's checkpoint list.
type courseCheckpointBody = struct {
	LearningPathId uuid.UUID `json:"learning_path_id"`
	Title          *string   `json:"title,omitempty"`
}

// courseCheckpointSpec is one checkpoint a "creates/replaces a course"
// step wants in its request: the learning path slug and an optional title
// override (empty means none).
type courseCheckpointSpec struct {
	slug  string
	title string
}

func toCourseCheckpointBody(specs []courseCheckpointSpec) []courseCheckpointBody {
	result := make([]courseCheckpointBody, len(specs))
	for i, s := range specs {
		result[i].LearningPathId = pathID(s.slug)
		if s.title != "" {
			title := s.title
			result[i].Title = &title
		}
	}
	return result
}

func registerCourseSteps(sc *godog.ScenarioContext, w *world) {
	// ── Seeding ──────────────────────────────────────────────────────────
	sc.Step(`^a course "([^"]+)" exists as a draft with checkpoints "([^"]+)", "([^"]+)"$`, w.courseExistsDraftTwoCheckpoints)
	sc.Step(`^a course "([^"]+)" exists, published, with checkpoints "([^"]+)", "([^"]+)"$`, w.courseExistsPublishedTwoCheckpoints)
	sc.Step(`^a course "([^"]+)" exists as a draft with checkpoints "([^"]+)", created by "([^"]+)"$`, w.courseExistsDraftWithCreator)
	sc.Step(`^a second course "([^"]+)" exists, published, with checkpoints "([^"]+)"$`, w.courseExistsPublishedWithCheckpoint)
	sc.Step(`^student "([^"]+)" is enrolled in "([^"]+)"$`, w.studentEnrolledInCourseSetup)

	// ── Creating a course ────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" creates a course titled "([^"]+)" with checkpoints in order: "([^"]+)", "([^"]+)"$`, w.createsCourseTwoCheckpoints)
	sc.Step(`^"([^"]+)" creates a course titled "([^"]+)" with checkpoints in order: "([^"]+)"$`, w.createsCourseOneCheckpoint)
	sc.Step(`^"([^"]+)" creates a course titled "([^"]+)" with checkpoints in order: "([^"]+)" titled "([^"]+)", "([^"]+)"$`, w.createsCourseWithTitledCheckpoint)
	sc.Step(`^"([^"]+)" submits a create course request with the title field omitted$`, w.submitsCreateCourseMissingTitle)
	sc.Step(`^"([^"]+)" submits a create course request with an empty checkpoints array$`, w.submitsCreateCourseEmptyCheckpoints)
	sc.Step(`^"([^"]+)" creates a course with a checkpoint referencing a learning path ID that does not exist$`, w.createsCourseMissingLearningPath)
	sc.Step(`^"([^"]+)" attempts to create a course$`, w.attemptsCreateCourse)
	sc.Step(`^an unauthenticated request attempts to create a course$`, w.unauthCreatesCourse)

	sc.Step(`^the course is created and assigned a stable identifier$`, w.courseCreatedStableID)
	sc.Step(`^the checkpoints are returned with positions 1 and 2 respectively$`, w.checkpointsHavePositions1And2)
	sc.Step(`^the course has status "([^"]+)"$`, w.courseHasStatus)
	sc.Step(`^the course records "([^"]+)" as the creator$`, w.courseRecordsCreator)
	sc.Step(`^the first checkpoint's effective title is "([^"]+)"$`, w.firstCheckpointEffectiveTitleIs)
	sc.Step(`^the second checkpoint's effective title is the title of "([^"]+)"$`, w.secondCheckpointEffectiveTitleIsTitleOf)

	// ── Retrieving live state ────────────────────────────────────────────
	sc.Step(`^"([^"]+)" retrieves course "([^"]+)"$`, w.retrievesCourse)
	sc.Step(`^"([^"]+)" attempts to retrieve course "([^"]+)"$`, w.attemptsRetrieveCourse)
	sc.Step(`^"([^"]+)" retrieves a course with an ID that does not exist$`, w.retrievesMissingCourse)
	sc.Step(`^the response includes each checkpoint's learning_path_id$`, w.responseIncludesCheckpointLearningPathIDs)

	// ── Replacing / reordering ───────────────────────────────────────────
	sc.Step(`^"([^"]+)" replaces course "([^"]+)" with the retrieved checkpoints reordered to: "([^"]+)", "([^"]+)"$`, w.replacesCourseWithRetrievedCheckpointsReordered)
	sc.Step(`^"([^"]+)" replaces course "([^"]+)" with checkpoints in order: "([^"]+)", "([^"]+)"$`, w.replacesCourseTwoCheckpoints)
	sc.Step(`^"([^"]+)" replaces course "([^"]+)" with checkpoints in order: "([^"]+)"$`, w.replacesCourseOneCheckpoint)
	sc.Step(`^"([^"]+)" attempts to replace course "([^"]+)" with checkpoints in order: "([^"]+)"$`, w.attemptsReplaceCourseOneCheckpoint)
	sc.Step(`^the checkpoints are returned with positions (\d+) and (\d+) in the order "([^"]+)", "([^"]+)"$`, w.checkpointsReturnedInOrder)
	sc.Step(`^"([^"]+)"'s enrollment is still pinned to the version she enrolled under$`, w.enrollmentStillPinnedToVersionEnrolledUnder)
	sc.Step(`^"([^"]+)"'s current path is unaffected$`, w.currentPathUnaffected)

	// ── Publishing ───────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" publishes course "([^"]+)"$`, w.publishesCourse)
	sc.Step(`^"([^"]+)" attempts to publish course "([^"]+)"$`, w.attemptsPublishCourse)
	sc.Step(`^a new course version (\d+) is created, snapshotting the title, summary, level, and checkpoints$`, w.newCourseVersionCreated)
	sc.Step(`^a new course version (\d+) is created$`, w.newCourseVersionCreated)
	sc.Step(`^the course's status becomes "([^"]+)"$`, w.courseStatusBecomes)
	sc.Step(`^"([^"]+)"'s enrollment is still pinned to course version (\d+)$`, w.enrollmentPinnedToCourseVersion)

	// ── Retiring ─────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" retires course "([^"]+)"$`, w.retiresCourse)
	sc.Step(`^"([^"]+)" attempts to retire course "([^"]+)"$`, w.attemptsRetireCourse)
	sc.Step(`^"([^"]+)"'s enrollment and current path are unaffected$`, w.enrollmentAndCurrentPathUnaffected)

	// ── Catalog ──────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" lists the course catalog$`, w.listsCourseCatalog)

	// ── Published outline ────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" retrieves the published version of course "([^"]+)"$`, w.retrievesPublishedCourse)
	sc.Step(`^"([^"]+)" attempts to retrieve the published version of course "([^"]+)"$`, w.attemptsRetrievePublishedCourse)
	sc.Step(`^"([^"]+)" attempts to retrieve the published version of a course with an ID that does not exist$`, w.attemptsRetrievePublishedMissingCourse)
	sc.Step(`^the response includes each checkpoint's title and its ordered item titles$`, w.responseIncludesCheckpointTitlesAndItemTitles)
	sc.Step(`^the response does not include any item's lesson content$`, w.responseHasNoLessonContent)
	sc.Step(`^the response does not include any checkpoint's learning_path_id$`, w.responseHasNoCheckpointLearningPathID)
	sc.Step(`^the response still shows checkpoints "([^"]+)", "([^"]+)" from the last published version$`, w.responseStillShowsCheckpoints)
}

// seedCourse creates a course draft with one checkpoint per slug in
// pathSlugs (in order, no title overrides), owned by creatorName, and
// optionally publishes it — the general-purpose seed helper behind every
// "a course ... exists ..." Given step that needs more than one checkpoint
// or an explicit creator. Mirrors seedCourseWithCheckpoint's isolated-admin
// publish identity (steps_learning_paths_test.go), but authors the draft as
// creatorName rather than an isolated seed identity, since several
// courses.feature scenarios go on to replace the course as a named teacher
// and require ownership to match. Any pathSlug with no learning path seeded
// yet is created via putLearningPathDefault first — feature files outside
// courses.feature (course-enrollment.feature, current-path-lifecycle.feature)
// reference checkpoint slugs directly, with no separate "a learning path ...
// exists" Given step of their own, so "a course ... exists ..." must be
// self-sufficient rather than depend on Background ordering.
func (w *world) seedCourse(courseSlug string, pathSlugs []string, creatorName string, publish bool) (uuid.UUID, error) {
	return w.seedCourseIn("en", courseSlug, pathSlugs, creatorName, publish)
}

// seedCourseIn is seedCourse for a course written in language.
func (w *world) seedCourseIn(language, courseSlug string, pathSlugs []string, creatorName string, publish bool) (uuid.UUID, error) {
	return w.seedCourseWith(courseSlug, pathSlugs, creatorName, publish, func(body *generated.CreateCourseRequest) {
		body.Language = language
	})
}

// seedCourseWith is seedCourse with the create request adjusted before it is
// sent, for a scenario that needs a particular language or instruments.
func (w *world) seedCourseWith(courseSlug string, pathSlugs []string, creatorName string, publish bool, adjust func(*generated.CreateCourseRequest)) (uuid.UUID, error) {
	for _, slug := range pathSlugs {
		if _, err := w.paths.GetByID(context.Background(), pathID(slug).String()); err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				return uuid.UUID{}, err
			}
			if err := w.putLearningPathDefault(slug); err != nil {
				return uuid.UUID{}, err
			}
		}
	}

	w.ensureRegistered(creatorName, domain.RoleTeacher)
	teacherCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub(creatorName))

	specs := make([]courseCheckpointSpec, len(pathSlugs))
	for i, slug := range pathSlugs {
		specs[i] = courseCheckpointSpec{slug: slug}
	}

	body := &generated.CreateCourseRequest{
		Language:    "en",
		Title:       courseSlug,
		Summary:     "Seeded for testing",
		Level:       generated.CreateCourseRequestLevelBeginner,
		Checkpoints: toCourseCheckpointBody(specs),
	}
	adjust(body)
	resp, err := w.handler.CreateCourse(teacherCtx, generated.CreateCourseRequestObject{Body: body})
	if err != nil {
		return uuid.UUID{}, err
	}
	created, ok := resp.(generated.CreateCourse201JSONResponse)
	if !ok {
		return uuid.UUID{}, fmt.Errorf("setup: expected course creation to succeed, got %#v", resp)
	}
	w.courseIDBySlug[courseSlug] = created.CourseId

	if !publish {
		return created.CourseId, nil
	}

	adminName := "seed-admin-for-" + courseSlug
	w.ensureRegistered(adminName, domain.RoleAdmin)
	adminCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub(adminName))
	publishResp, err := w.handler.PublishCourse(adminCtx, generated.PublishCourseRequestObject{CourseId: created.CourseId})
	if err != nil {
		return uuid.UUID{}, err
	}
	if _, ok := publishResp.(generated.PublishCourse201JSONResponse); !ok {
		return uuid.UUID{}, fmt.Errorf("setup: expected course publish to succeed, got %#v", publishResp)
	}
	return created.CourseId, nil
}

func (w *world) courseExistsDraftTwoCheckpoints(courseSlug, p1, p2 string) error {
	_, err := w.seedCourse(courseSlug, []string{p1, p2}, "bob", false)
	return err
}

func (w *world) courseExistsPublishedTwoCheckpoints(courseSlug, p1, p2 string) error {
	_, err := w.seedCourse(courseSlug, []string{p1, p2}, "bob", true)
	return err
}

func (w *world) courseExistsDraftWithCreator(courseSlug, p1, creatorName string) error {
	_, err := w.seedCourse(courseSlug, []string{p1}, creatorName, false)
	return err
}

func (w *world) createsCourse(title string, specs []courseCheckpointSpec) error {
	resp, err := w.handler.CreateCourse(w.ctx(), generated.CreateCourseRequestObject{
		Body: &generated.CreateCourseRequest{
			Language:    "en",
			Title:       title,
			Summary:     "A summary",
			Level:       generated.CreateCourseRequestLevelBeginner,
			Checkpoints: toCourseCheckpointBody(specs),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsCourseTwoCheckpoints(name, title, p1, p2 string) error {
	return w.createsCourse(title, []courseCheckpointSpec{{slug: p1}, {slug: p2}})
}

func (w *world) createsCourseOneCheckpoint(name, title, p1 string) error {
	return w.createsCourse(title, []courseCheckpointSpec{{slug: p1}})
}

func (w *world) createsCourseWithTitledCheckpoint(name, title, p1, p1Title, p2 string) error {
	return w.createsCourse(title, []courseCheckpointSpec{{slug: p1, title: p1Title}, {slug: p2}})
}

func (w *world) submitsCreateCourseMissingTitle(string) error {
	return w.createsCourse("", []courseCheckpointSpec{{slug: "open-chords-path"}})
}

func (w *world) submitsCreateCourseEmptyCheckpoints(string) error {
	return w.createsCourse("Title", nil)
}

func (w *world) createsCourseMissingLearningPath(string) error {
	return w.createsCourse("Title", []courseCheckpointSpec{{slug: "does-not-exist"}})
}

func (w *world) attemptsCreateCourse(string) error {
	return w.createsCourse("Title", []courseCheckpointSpec{{slug: "open-chords-path"}})
}

func (w *world) unauthCreatesCourse() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateCourse("")
}

func (w *world) courseCreatedStableID() error {
	if _, ok := w.lastResp.(generated.CreateCourse201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) checkpointsHavePositions1And2() error {
	resp, ok := w.lastResp.(generated.CreateCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(resp.Checkpoints) != 2 {
		return fmt.Errorf("expected 2 checkpoints, got %d", len(resp.Checkpoints))
	}
	for i, cp := range resp.Checkpoints {
		if cp.Position != i+1 {
			return fmt.Errorf("expected checkpoint %d to have position %d, got %d", i, i+1, cp.Position)
		}
	}
	return nil
}

func (w *world) courseHasStatus(status string) error {
	resp, ok := w.lastResp.(generated.CreateCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if string(resp.Status) != status {
		return fmt.Errorf("expected course status %q, got %q", status, resp.Status)
	}
	return nil
}

func (w *world) courseRecordsCreator(name string) error {
	resp, ok := w.lastResp.(generated.CreateCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.CreatedBy.UserId != w.userMotifID[name] {
		return fmt.Errorf("expected created_by %s for %q, got %s", w.userMotifID[name], name, resp.CreatedBy.UserId)
	}
	return nil
}

func (w *world) firstCheckpointEffectiveTitleIs(title string) error {
	resp, ok := w.lastResp.(generated.CreateCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(resp.Checkpoints) == 0 {
		return fmt.Errorf("expected at least one checkpoint")
	}
	if resp.Checkpoints[0].EffectiveTitle != title {
		return fmt.Errorf("expected first checkpoint effective_title %q, got %q", title, resp.Checkpoints[0].EffectiveTitle)
	}
	return nil
}

func (w *world) secondCheckpointEffectiveTitleIsTitleOf(pathSlug string) error {
	resp, ok := w.lastResp.(generated.CreateCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(resp.Checkpoints) < 2 {
		return fmt.Errorf("expected at least two checkpoints")
	}
	// putLearningPath* helpers set a template's Title to its own slug, so
	// the path's own title is simply the slug text itself.
	if resp.Checkpoints[1].EffectiveTitle != pathSlug {
		return fmt.Errorf("expected second checkpoint effective_title %q (the path's own title), got %q", pathSlug, resp.Checkpoints[1].EffectiveTitle)
	}
	return nil
}

func (w *world) retrievesCourse(name, courseSlug string) error {
	resp, err := w.handler.GetCourse(w.ctx(), generated.GetCourseRequestObject{CourseId: w.courseIDBySlug[courseSlug]})
	w.lastResp, w.lastErr = resp, err
	w.lastCourseSlug = courseSlug
	if r, ok := resp.(generated.GetCourse200JSONResponse); ok {
		w.lastRetrievedCourseCheckpoints = r.Checkpoints
	}
	return err
}

func (w *world) attemptsRetrieveCourse(name, courseSlug string) error {
	return w.retrievesCourse(name, courseSlug)
}

func (w *world) retrievesMissingCourse(string) error {
	resp, err := w.handler.GetCourse(w.ctx(), generated.GetCourseRequestObject{CourseId: deterministicUUID("course", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) responseIncludesCheckpointLearningPathIDs() error {
	resp, ok := w.lastResp.(generated.GetCourse200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(resp.Checkpoints) == 0 {
		return fmt.Errorf("expected at least one checkpoint")
	}
	for i, cp := range resp.Checkpoints {
		if cp.LearningPathId == (uuid.UUID{}) {
			return fmt.Errorf("expected checkpoint %d to include learning_path_id, got the zero value", i)
		}
	}
	return nil
}

func (w *world) replacesCourseWithCheckpoints(name, courseSlug string, checkpoints []courseCheckpointBody) error {
	resp, err := w.handler.ReplaceCourse(w.ctx(), generated.ReplaceCourseRequestObject{
		CourseId: w.courseIDBySlug[courseSlug],
		Body: &generated.ReplaceCourseRequest{
			Language:    "en",
			Title:       courseSlug,
			Summary:     "A summary",
			Level:       generated.ReplaceCourseRequestLevelBeginner,
			Checkpoints: checkpoints,
		},
	})
	w.lastResp, w.lastErr = resp, err
	w.lastCourseSlug = courseSlug
	return err
}

func (w *world) replacesCourseWithRetrievedCheckpointsReordered(name, courseSlug, p1, p2 string) error {
	order := []string{p1, p2}
	checkpoints := make([]courseCheckpointBody, len(order))
	for i, slug := range order {
		want := pathID(slug)
		found := false
		for _, cp := range w.lastRetrievedCourseCheckpoints {
			if cp.LearningPathId == want {
				checkpoints[i] = courseCheckpointBody{LearningPathId: cp.LearningPathId, Title: cp.Title}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no previously retrieved checkpoint references learning path %q", slug)
		}
	}
	return w.replacesCourseWithCheckpoints(name, courseSlug, checkpoints)
}

func (w *world) replacesCourseTwoCheckpoints(name, courseSlug, p1, p2 string) error {
	return w.replacesCourseWithCheckpoints(name, courseSlug, toCourseCheckpointBody([]courseCheckpointSpec{{slug: p1}, {slug: p2}}))
}

func (w *world) replacesCourseOneCheckpoint(name, courseSlug, p1 string) error {
	return w.replacesCourseWithCheckpoints(name, courseSlug, toCourseCheckpointBody([]courseCheckpointSpec{{slug: p1}}))
}

func (w *world) attemptsReplaceCourseOneCheckpoint(name, courseSlug, p1 string) error {
	return w.replacesCourseOneCheckpoint(name, courseSlug, p1)
}

func (w *world) checkpointsReturnedInOrder(p1Str, p2Str, n1, n2 string) error {
	var checkpoints []generated.CourseCheckpoint
	switch resp := w.lastResp.(type) {
	case generated.ReplaceCourse200JSONResponse:
		checkpoints = resp.Checkpoints
	case generated.CreateCourse201JSONResponse:
		checkpoints = resp.Checkpoints
	default:
		return fmt.Errorf("expected a course response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	wantPositions := []string{p1Str, p2Str}
	wantSlugs := []string{n1, n2}
	if len(checkpoints) != 2 {
		return fmt.Errorf("expected 2 checkpoints, got %d", len(checkpoints))
	}
	for i, cp := range checkpoints {
		wantPos, err := parseInt(wantPositions[i])
		if err != nil {
			return err
		}
		if cp.Position != wantPos {
			return fmt.Errorf("expected checkpoint %d to have position %d, got %d", i, wantPos, cp.Position)
		}
		if cp.LearningPathId != pathID(wantSlugs[i]) {
			return fmt.Errorf("expected checkpoint %d to reference %s, got %s", i, pathID(wantSlugs[i]), cp.LearningPathId)
		}
	}
	return nil
}

func (w *world) enrollmentStillPinnedToVersionEnrolledUnder(name string) error {
	id, ok := w.courseEnrollmentIDByStudent[name]
	if !ok {
		return fmt.Errorf("no enrollment tracked for %q", name)
	}
	e, err := w.courseEnrollments.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	if e.CourseVersionNumber != 1 {
		return fmt.Errorf("expected %q's enrollment to remain pinned to version 1, got version %d", name, e.CourseVersionNumber)
	}
	return nil
}

// currentPathUnaffected asserts studentName's current course enrollment
// still has an active checkpoint copied from "open-chords-path" — every
// scenario this backs seeds "fingerstyle-journey" with checkpoint 1
// pointing at that template, so this directly confirms the student's
// in-progress copy was never disturbed by a later draft edit or retire.
func (w *world) currentPathUnaffected(name string) error {
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
	if e.ActiveCheckpointStudentPathID == nil {
		return fmt.Errorf("expected %q's enrollment to still have an active checkpoint", name)
	}
	sp, err := w.studentPaths.GetByID(context.Background(), *e.ActiveCheckpointStudentPathID)
	if err != nil {
		return err
	}
	if sp.SourceTemplateID != pathID("open-chords-path").String() {
		return fmt.Errorf("expected %q's active checkpoint to still be copied from open-chords-path, got %s", name, sp.SourceTemplateID)
	}
	return nil
}

// publishesCourse authenticates as name explicitly, rather than relying on
// w.ctx()'s single "currently authenticated" identity — scenarios that name
// more than one actor (e.g. a teacher replaces a draft, then a separately
// named admin publishes it) authenticate the second actor after the first,
// which would otherwise leave the wrong identity as "current" by the time
// this step runs.
func (w *world) publishesCourse(name, courseSlug string) error {
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(name))
	resp, err := w.handler.PublishCourse(ctx, generated.PublishCourseRequestObject{CourseId: w.courseIDBySlug[courseSlug]})
	w.lastResp, w.lastErr = resp, err
	w.lastCourseSlug = courseSlug
	return err
}

func (w *world) attemptsPublishCourse(name, courseSlug string) error {
	return w.publishesCourse(name, courseSlug)
}

func (w *world) newCourseVersionCreated(versionStr string) error {
	resp, ok := w.lastResp.(generated.PublishCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	want, err := parseInt(versionStr)
	if err != nil {
		return err
	}
	if resp.VersionNumber != want {
		return fmt.Errorf("expected version_number %d, got %d", want, resp.VersionNumber)
	}
	return nil
}

func (w *world) courseStatusBecomes(status string) error {
	id, ok := w.courseIDBySlug[w.lastCourseSlug]
	if !ok {
		return fmt.Errorf("no course tracked to check status for")
	}
	c, err := w.courses.GetByID(context.Background(), id.String())
	if err != nil {
		return err
	}
	if string(c.Status) != status {
		return fmt.Errorf("expected course %q status %q, got %q", w.lastCourseSlug, status, c.Status)
	}
	return nil
}

func (w *world) enrollmentPinnedToCourseVersion(name, versionStr string) error {
	want, err := parseInt(versionStr)
	if err != nil {
		return err
	}
	id, ok := w.courseEnrollmentIDByStudent[name]
	if !ok {
		return fmt.Errorf("no enrollment tracked for %q", name)
	}
	e, err := w.courseEnrollments.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	if e.CourseVersionNumber != want {
		return fmt.Errorf("expected %q's enrollment pinned to version %d, got %d", name, want, e.CourseVersionNumber)
	}
	return nil
}

// retiresCourse sets courseSlug's status directly on the fake
// CourseRepository rather than through the HTTP handler, mirroring
// courseHasBeenRetired's identical shortcut in steps_learning_paths_test.go.
// Every scenario using this step wording only needs the course to end up
// retired — as a precondition for something else under test, or as the
// fact asserted by courseStatusBecomes — never RetireCourse's own
// request/response behavior, which attemptsRetireCourse below exercises
// for real.
func (w *world) retiresCourse(name, courseSlug string) error {
	id, ok := w.courseIDBySlug[courseSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", courseSlug)
	}
	w.lastCourseSlug = courseSlug
	return w.courses.UpdateStatus(context.Background(), id.String(), domain.CourseStatusRetired)
}

// attemptsRetireCourse calls the real RetireCourse handler — unlike
// retiresCourse above, this exercises the endpoint's own
// request/response/authorization behavior for real.
func (w *world) attemptsRetireCourse(name, courseSlug string) error {
	resp, err := w.handler.RetireCourse(w.ctx(), generated.RetireCourseRequestObject{CourseId: w.courseIDBySlug[courseSlug]})
	w.lastResp, w.lastErr = resp, err
	w.lastCourseSlug = courseSlug
	return err
}

func (w *world) enrollmentAndCurrentPathUnaffected(name string) error {
	id, ok := w.courseEnrollmentIDByStudent[name]
	if !ok {
		return fmt.Errorf("no enrollment tracked for %q", name)
	}
	e, err := w.courseEnrollments.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	if !e.IsActive() {
		return fmt.Errorf("expected %q's enrollment to remain active, got status %q", name, e.Status)
	}
	return w.currentPathUnaffected(name)
}

func (w *world) listsCourseCatalog(string) error {
	resp, err := w.handler.ListCatalogCourses(w.ctx(), generated.ListCatalogCoursesRequestObject{})
	w.storeCatalogResponse(resp, err)
	return err
}

func (w *world) retrievesPublishedCourse(name, courseSlug string) error {
	resp, err := w.handler.GetPublishedCourse(w.ctx(), generated.GetPublishedCourseRequestObject{CourseId: w.courseIDBySlug[courseSlug]})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsRetrievePublishedCourse(name, courseSlug string) error {
	return w.retrievesPublishedCourse(name, courseSlug)
}

func (w *world) attemptsRetrievePublishedMissingCourse(string) error {
	resp, err := w.handler.GetPublishedCourse(w.ctx(), generated.GetPublishedCourseRequestObject{CourseId: deterministicUUID("course", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) responseIncludesCheckpointTitlesAndItemTitles() error {
	resp, ok := w.lastResp.(generated.GetPublishedCourse200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(resp.Checkpoints) == 0 {
		return fmt.Errorf("expected at least one checkpoint")
	}
	for _, cp := range resp.Checkpoints {
		if cp.Title == "" {
			return fmt.Errorf("expected every checkpoint to have a title")
		}
		if len(cp.Items) == 0 {
			return fmt.Errorf("expected checkpoint %d to have items", cp.Position)
		}
		for _, item := range cp.Items {
			if item.Title == "" {
				return fmt.Errorf("expected every item to have a title")
			}
		}
	}
	return nil
}

// responseHasNoLessonContent and responseHasNoCheckpointLearningPathID both
// hold unconditionally once the response is confirmed to be a
// GetPublishedCourse200JSONResponse: generated.CourseOutlineItem and
// CourseOutlineCheckpoint (see toCourseDetail) structurally carry no
// lesson-content or learning_path_id field at all, unlike the live draft's
// richer Course/CourseCheckpoint representation — so a caller of this
// endpoint cannot receive either even by mistake.
func (w *world) responseHasNoLessonContent() error {
	if _, ok := w.lastResp.(generated.GetPublishedCourse200JSONResponse); !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) responseHasNoCheckpointLearningPathID() error {
	return w.responseHasNoLessonContent()
}

func (w *world) responseStillShowsCheckpoints(n1, n2 string) error {
	resp, ok := w.lastResp.(generated.GetPublishedCourse200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	want := []string{n1, n2}
	if len(resp.Checkpoints) != len(want) {
		return fmt.Errorf("expected %d checkpoints, got %d", len(want), len(resp.Checkpoints))
	}
	for i, cp := range resp.Checkpoints {
		if cp.Title != want[i] {
			return fmt.Errorf("expected checkpoint %d title %q, got %q", i, want[i], cp.Title)
		}
	}
	return nil
}

func registerCourseReactivationSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" reactivates course "([^"]+)"$`, w.reactivatesCourse)
	sc.Step(`^"([^"]+)" attempts to reactivate course "([^"]+)"$`, w.reactivatesCourse)
	sc.Step(`^"([^"]+)" reactivates a course with an ID that does not exist$`, w.reactivatesMissingCourse)
	sc.Step(`^course "([^"]+)" still has only course version (\d+)$`, w.courseStillHasOnlyVersion)
	sc.Step(`^the course has unpublished changes$`, w.courseHasUnpublishedChanges)
	sc.Step(`^the published version of "([^"]+)" still has checkpoints "([^"]+)"$`, w.publishedVersionHasCheckpoints)
}

// reactivatesCourse goes through the HTTP handler, unlike retiresCourse:
// reactivation's own refusals (a course that isn't retired, a caller who
// isn't an admin) are what several scenarios assert.
func (w *world) reactivatesCourse(_, courseSlug string) error {
	resp, err := w.handler.ReactivateCourse(w.ctx(), generated.ReactivateCourseRequestObject{CourseId: w.courseIDBySlug[courseSlug]})
	w.lastResp, w.lastErr = resp, err
	w.lastCourseSlug = courseSlug
	return err
}

func (w *world) reactivatesMissingCourse(string) error {
	resp, err := w.handler.ReactivateCourse(w.ctx(), generated.ReactivateCourseRequestObject{CourseId: deterministicUUID("course", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) courseStillHasOnlyVersion(courseSlug, versionStr string) error {
	want, err := parseInt(versionStr)
	if err != nil {
		return err
	}
	latest, ok := w.courseVersions.latest(w.courseIDBySlug[courseSlug].String())
	if !ok {
		return fmt.Errorf("course %q has no published version", courseSlug)
	}
	if latest.VersionNumber != want {
		return fmt.Errorf("expected course %q's latest version to be %d, got %d", courseSlug, want, latest.VersionNumber)
	}
	return nil
}

func (w *world) courseHasUnpublishedChanges() error {
	resp, err := w.handler.GetCourse(w.ctx(), generated.GetCourseRequestObject{CourseId: w.courseIDBySlug[w.lastCourseSlug]})
	if err != nil {
		return err
	}
	course, ok := resp.(generated.GetCourse200JSONResponse)
	if !ok {
		return fmt.Errorf("expected the course, got %#v", resp)
	}
	if !course.HasUnpublishedChanges {
		return fmt.Errorf("expected course %q to have unpublished changes", w.lastCourseSlug)
	}
	return nil
}

// publishedVersionHasCheckpoints compares the published outline's checkpoint
// titles with the listed learning path slugs, which the seeding helpers use
// as each template's own title.
func (w *world) publishedVersionHasCheckpoints(courseSlug, slugList string) error {
	resp, err := w.handler.GetPublishedCourse(w.ctx(), generated.GetPublishedCourseRequestObject{CourseId: w.courseIDBySlug[courseSlug]})
	if err != nil {
		return err
	}
	detail, ok := resp.(generated.GetPublishedCourse200JSONResponse)
	if !ok {
		return fmt.Errorf("expected the published course, got %#v", resp)
	}
	var got []string
	for _, checkpoint := range detail.Checkpoints {
		got = append(got, checkpoint.Title)
	}
	want := quotedValues(`"` + slugList + `"`)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		return fmt.Errorf("expected the published version of %q to have checkpoints %v, got %v", courseSlug, want, got)
	}
	return nil
}

func registerCourseLanguageSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" creates a course titled "([^"]+)" in language "([^"]*)" with checkpoints in order: "([^"]+)"$`, w.createsCourseInLanguage)
	sc.Step(`^"([^"]+)" submits a create course request with the language field omitted$`, w.submitsCreateCourseWithoutLanguage)
	sc.Step(`^the course's language is "([^"]+)"$`, w.courseLanguageIs)
	sc.Step(`^a course "([^"]+)" exists, published in language "([^"]+)", with checkpoints "([^"]+)"$`, func(courseSlug, language, pathSlug string) error {
		_, err := w.seedCourseIn(language, courseSlug, []string{pathSlug}, "bob", true)
		return err
	})
	sc.Step(`^a course "([^"]+)" exists, published in language "([^"]+)", created by "([^"]+)", with checkpoints "([^"]+)"$`, func(courseSlug, language, creator, pathSlug string) error {
		_, err := w.seedCourseIn(language, courseSlug, []string{pathSlug}, creator, true)
		return err
	})
	sc.Step(`^a course "([^"]+)" exists as a draft in language "([^"]+)" with checkpoints "([^"]+)"$`, func(courseSlug, language, pathSlug string) error {
		_, err := w.seedCourseIn(language, courseSlug, []string{pathSlug}, "bob", false)
		return err
	})
	sc.Step(`^"([^"]+)" replaces course "([^"]+)" setting its language to "([^"]+)"$`, w.replacesCourseLanguage)
	sc.Step(`^course version (\d+) records the language "([^"]+)"$`, w.courseVersionRecordsLanguage)
}

func (w *world) createCourseWithLanguage(title, language string, specs []courseCheckpointSpec) error {
	resp, err := w.handler.CreateCourse(w.ctx(), generated.CreateCourseRequestObject{
		Body: &generated.CreateCourseRequest{
			Title:       title,
			Summary:     "A summary",
			Level:       generated.CreateCourseRequestLevelBeginner,
			Language:    language,
			Checkpoints: toCourseCheckpointBody(specs),
		},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsCourseInLanguage(_, title, language, pathSlug string) error {
	return w.createCourseWithLanguage(title, language, []courseCheckpointSpec{{slug: pathSlug}})
}

// submitsCreateCourseWithoutLanguage sends the request with language left
// at its zero value, which is how an omitted JSON field decodes.
func (w *world) submitsCreateCourseWithoutLanguage(string) error {
	return w.createCourseWithLanguage("Title", "", []courseCheckpointSpec{{slug: "open-chords-path"}})
}

func (w *world) courseLanguageIs(want string) error {
	resp, ok := w.lastResp.(generated.CreateCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.Language != want {
		return fmt.Errorf("expected the course's language to be %q, got %q", want, resp.Language)
	}
	return nil
}

// replacesCourseLanguage replaces the course's live draft with the same
// content in another language, leaving it otherwise as it was.
func (w *world) replacesCourseLanguage(_, courseSlug, language string) error {
	current, err := w.courses.GetByID(context.Background(), w.courseIDBySlug[courseSlug].String())
	if err != nil {
		return err
	}
	checkpoints := make([]courseCheckpointBody, len(current.Checkpoints))
	for i, cp := range current.Checkpoints {
		checkpoints[i].LearningPathId = uuid.MustParse(cp.LearningPathID)
		checkpoints[i].Title = cp.Title
	}
	resp, err := w.handler.ReplaceCourse(w.ctx(), generated.ReplaceCourseRequestObject{
		CourseId: w.courseIDBySlug[courseSlug],
		Body: &generated.ReplaceCourseRequest{
			Title:       current.Title,
			Summary:     current.Summary,
			Level:       generated.ReplaceCourseRequestLevel(current.Level),
			Language:    language,
			Checkpoints: checkpoints,
		},
	})
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.ReplaceCourse200JSONResponse); !ok {
		return fmt.Errorf("setup: expected the course replace to succeed, got %#v", resp)
	}
	w.lastCourseSlug = courseSlug
	return nil
}

func (w *world) courseVersionRecordsLanguage(versionStr, want string) error {
	resp, ok := w.lastResp.(generated.PublishCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	number, err := parseInt(versionStr)
	if err != nil {
		return err
	}
	if resp.VersionNumber != number || resp.LanguageSnapshot != want {
		return fmt.Errorf("expected course version %d to record language %q, got version %d in %q", number, want, resp.VersionNumber, resp.LanguageSnapshot)
	}
	return nil
}
