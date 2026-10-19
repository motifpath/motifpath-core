//go:build integration

package bdd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerPathPublishingSteps(sc *godog.ScenarioContext, w *world) {
	// ── Seeding paths in a given state ──────────────────────────────────────
	sc.Step(`^a learning path "([^"]+)" exists as a draft, created by "([^"]+)", with a summary, a language, a level and published lessons$`, w.putCompleteDraftPath)
	sc.Step(`^a learning path "([^"]+)" exists as a draft$`, func(slug string) error {
		return w.putStatusPath(slug, "bob", domain.LearningPathStatusDraft)
	})
	sc.Step(`^a learning path "([^"]+)" exists, published$`, func(slug string) error {
		return w.putStatusPath(slug, "bob", domain.LearningPathStatusPublished)
	})
	sc.Step(`^learning path "([^"]+)" is a draft$`, func(slug string) error {
		return w.setPathStatus(slug, domain.LearningPathStatusDraft)
	})
	sc.Step(`^learning path "([^"]+)" is published$`, func(slug string) error {
		return w.setPathStatus(slug, domain.LearningPathStatusPublished)
	})
	sc.Step(`^learning path "([^"]+)" has no summary$`, func(slug string) error {
		return w.editSeededPath(slug, func(p *domain.LearningPath) { p.Summary = nil })
	})
	sc.Step(`^learning path "([^"]+)" has no language$`, func(slug string) error {
		return w.editSeededPath(slug, func(p *domain.LearningPath) { p.Language = nil })
	})
	sc.Step(`^learning path "([^"]+)" has no summary and no language$`, func(slug string) error {
		return w.editSeededPath(slug, func(p *domain.LearningPath) { p.Summary, p.Language = nil, nil })
	})
	sc.Step(`^learning path "([^"]+)" was created before levels were recorded$`, func(slug string) error {
		return w.editSeededPath(slug, func(p *domain.LearningPath) { p.Level = nil })
	})
	sc.Step(`^learning path "([^"]+)" includes a content node "([^"]+)" that has never been published$`, w.pathIncludesUnpublishedNode)
	sc.Step(`^a content node "([^"]+)" exists and has never been published$`, func(nodeSlug string) error {
		return w.putContentNode(nodeSlug, domain.ContentTypeVideo)
	})

	// ── Publishing and unpublishing ─────────────────────────────────────────
	sc.Step(`^"([^"]+)" publishes learning path "([^"]+)"$`, w.publishesPath)
	sc.Step(`^"([^"]+)" attempts to publish learning path "([^"]+)"$`, w.publishesPath)
	sc.Step(`^"([^"]+)" unpublishes learning path "([^"]+)"$`, w.unpublishesPath)
	sc.Step(`^"([^"]+)" attempts to unpublish learning path "([^"]+)"$`, w.unpublishesPath)
	sc.Step(`^"([^"]+)" publishes a learning path ID that does not exist$`, func(name string) error {
		resp, err := w.handler.PublishLearningPath(w.actorCtx(name), generated.PublishLearningPathRequestObject{LearningPathId: uuid.New()})
		w.lastResp, w.lastErr = resp, err
		return nil
	})
	sc.Step(`^an unauthenticated request attempts to publish learning path "([^"]+)"$`, func(slug string) error {
		resp, err := w.handler.PublishLearningPath(context.Background(), generated.PublishLearningPathRequestObject{LearningPathId: pathID(slug)})
		w.lastResp, w.lastErr = resp, err
		return nil
	})

	sc.Step(`^the learning path's status is "([^"]+)"$`, w.pathStatusIs)
	sc.Step(`^the learning path's status is still "([^"]+)"$`, w.pathStatusIs)
	sc.Step(`^the refusal lists "([^"]+)" as missing$`, func(requirement string) error {
		return w.refusalListsMissing([]string{requirement}, "")
	})
	sc.Step(`^the refusal lists "([^"]+)" and "([^"]+)" as missing$`, func(first, second string) error {
		return w.refusalListsMissing([]string{first, second}, "")
	})
	sc.Step(`^the refusal lists "([^"]+)" as missing, naming "([^"]+)"$`, func(requirement, nodeSlug string) error {
		return w.refusalListsMissing([]string{requirement}, nodeSlug)
	})

	// ── Editing and deleting published paths ────────────────────────────────
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" leaving out its summary$`, func(name, slug string) error {
		return w.replaceSeededPath(name, slug, func(body *generated.ReplaceLearningPathRequest) { body.Summary = nil })
	})
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" without a summary$`, func(name, slug string) error {
		return w.replaceSeededPath(name, slug, func(body *generated.ReplaceLearningPathRequest) { body.Summary = nil })
	})
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" adding "([^"]+)"$`, func(name, slug, nodeSlug string) error {
		return w.replaceSeededPath(name, slug, func(body *generated.ReplaceLearningPathRequest) {
			body.Items = append(body.Items, replaceItem(nodeSlug))
		})
	})
	sc.Step(`^the learning path is saved without a summary$`, w.pathSavedWithoutSummary)
	sc.Step(`^the learning path has no summary$`, w.pathSavedWithoutSummary)
	sc.Step(`^"([^"]+)" deletes learning path "([^"]+)"$`, w.deletesLearningPath)
	sc.Step(`^course "([^"]+)" has been retired$`, w.courseHasBeenRetired)

	// ── Summary and language ────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)" with a summary, language "([^"]+)" and level "([^"]+)"$`, func(name, title, language, level string) error {
		summary := "A summary of " + title
		return w.createsPathWith(name, title, &summary, &language, level)
	})
	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)" with summary "([^"]+)" in language "([^"]+)" with items in order: "([^"]+)"$`, func(name, title, summary, language, nodeSlug string) error {
		return w.createsPathWith(name, title, &summary, &language, "beginner", nodeSlug)
	})
	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)" in language "([^"]+)" with items in order: "([^"]+)"$`, func(name, title, language, nodeSlug string) error {
		return w.createsPathWith(name, title, nil, &language, "beginner", nodeSlug)
	})
	sc.Step(`^the learning path's summary is "([^"]+)"$`, func(want string) error {
		return w.createdPathField("summary", func(p generated.LearningPath) *string { return p.Summary }, want)
	})
	sc.Step(`^the learning path's language is "([^"]+)"$`, func(want string) error {
		return w.createdPathField("language", func(p generated.LearningPath) *string { return p.Language }, want)
	})
	sc.Step(`^the learning path has no summary and no language$`, w.createdPathHasNoSummaryOrLanguage)
	sc.Step(`^a learning path "([^"]+)" exists as a draft with items "([^"]+)" and summary "([^"]+)"$`, func(slug, nodeSlug, summary string) error {
		if err := w.putLearningPathOneItem(slug, nodeSlug); err != nil {
			return err
		}
		return w.editSeededPath(slug, func(p *domain.LearningPath) {
			p.Summary, p.Status = &summary, domain.LearningPathStatusDraft
		})
	})
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", in language "([^"]+)"$`, func(slug, nodeSlug, language string) error {
		if err := w.putLearningPathOneItem(slug, nodeSlug); err != nil {
			return err
		}
		return w.editSeededPath(slug, func(p *domain.LearningPath) { p.Language = &language })
	})
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)" and no language recorded$`, w.putLearningPathOneItem)

	// ── Library filters ─────────────────────────────────────────────────────
	sc.Step(`^the entry for "([^"]+)" reports status "([^"]+)"$`, w.libraryEntryReportsStatus)

	// ── Courses and assignment need published paths ─────────────────────────
	sc.Step(`^the refusal names "([^"]+)" as a draft learning path$`, w.refusalNamesDraftPath)
	sc.Step(`^course "([^"]+)" is still a draft$`, w.courseIsStillDraft)
	sc.Step(`^the draft of course "([^"]+)" adds checkpoint "([^"]+)"$`, w.courseDraftAddsCheckpoint)
	sc.Step(`^learners still see the course's earlier published version$`, w.learnersSeeEarlierVersion)
	sc.Step(`^a new course version is created$`, func() error {
		if _, ok := w.lastResp.(generated.PublishCourse201JSONResponse); !ok {
			return fmt.Errorf("expected a new course version, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return nil
	})
	sc.Step(`^"([^"]+)" has no copy of "([^"]+)"$`, w.hasNoCopyOf)
}

// actorCtx is the context of a request made by name — a setup step like
// "admin" publishes … acts as that user whatever the scenario authenticated
// as, the way alreadyHasAssigned does.
func (w *world) actorCtx(name string) context.Context {
	if name == "admin" {
		w.ensureRegistered(name, domain.RoleAdmin)
	}
	return w.identityCtx(name)
}

func (w *world) seededPath(slug string) (domain.LearningPath, error) {
	return w.paths.GetByID(context.Background(), pathID(slug).String())
}

func (w *world) editSeededPath(slug string, edit func(*domain.LearningPath)) error {
	path, err := w.seededPath(slug)
	if err != nil {
		return fmt.Errorf("setup: learning path %q: %w", slug, err)
	}
	edit(&path)
	w.paths.put(path)
	return nil
}

func (w *world) setPathStatus(slug string, status domain.LearningPathStatus) error {
	return w.editSeededPath(slug, func(p *domain.LearningPath) { p.Status = status })
}

// putCompleteDraftPath seeds a draft path that meets every publishing
// requirement: a summary, a language, a level and one published lesson.
func (w *world) putCompleteDraftPath(slug, creator string) error {
	return w.putPathFor(slug, creator, domain.LearningPathStatusDraft)
}

// putStatusPath seeds a complete path in the given status.
func (w *world) putStatusPath(slug, creator string, status domain.LearningPathStatus) error {
	return w.putPathFor(slug, creator, status)
}

func (w *world) putPathFor(slug, creator string, status domain.LearningPathStatus) error {
	nodeSlug := slug + "-lesson-1"
	if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
		return err
	}
	w.autoPublishNode(nodeSlug)
	summary, language, level := "A summary of "+slug, "en", domain.DifficultyLevelBeginner
	w.paths.put(domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: w.ensureRegistered(creator, domain.RoleTeacher).String(),
		Title:     slug,
		Summary:   &summary,
		Language:  &language,
		Level:     &level,
		Status:    status,
		Items:     []domain.LearningPathItem{{Position: 1, ContentNodeID: nodeID(nodeSlug).String(), Title: nodeSlug, ContentType: domain.ContentTypeVideo}},
		CreatedAt: fixedNow,
		UpdatedAt: fixedNow,
	})
	return nil
}

func (w *world) pathIncludesUnpublishedNode(slug, nodeSlug string) error {
	if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
		return err
	}
	return w.editSeededPath(slug, func(p *domain.LearningPath) {
		p.Items = append(p.Items, domain.LearningPathItem{
			Position: len(p.Items) + 1, ContentNodeID: nodeID(nodeSlug).String(), Title: nodeSlug, ContentType: domain.ContentTypeVideo,
		})
	})
}

func (w *world) publishesPath(name, slug string) error {
	w.lastPathSlug = slug
	resp, err := w.handler.PublishLearningPath(w.actorCtx(name), generated.PublishLearningPathRequestObject{LearningPathId: pathID(slug)})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) unpublishesPath(name, slug string) error {
	w.lastPathSlug = slug
	resp, err := w.handler.UnpublishLearningPath(w.actorCtx(name), generated.UnpublishLearningPathRequestObject{LearningPathId: pathID(slug)})
	w.lastResp, w.lastErr = resp, err
	return nil
}

// pathStatusIs checks the stored status of the path the scenario last acted
// on, or — for a path the scenario just created — the create response's.
func (w *world) pathStatusIs(want string) error {
	if w.lastPathSlug == "" {
		created, ok := w.lastResp.(generated.CreateLearningPath201JSONResponse)
		if !ok {
			return fmt.Errorf("expected a created learning path, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		if string(created.Status) != want {
			return fmt.Errorf("expected status %q, got %q", want, created.Status)
		}
		return nil
	}
	path, err := w.seededPath(w.lastPathSlug)
	if err != nil {
		return err
	}
	if string(path.Status) != want {
		return fmt.Errorf("expected %q to be %q, got %q (last response %#v)", w.lastPathSlug, want, path.Status, w.lastResp)
	}
	return nil
}

func (w *world) refusalListsMissing(want []string, unpublishedNode string) error {
	var refusal generated.LearningPathNotPublishableError
	switch resp := w.lastResp.(type) {
	case generated.PublishLearningPath409JSONResponse:
		refusal = generated.LearningPathNotPublishableError(resp)
	case generated.ReplaceLearningPath409JSONResponse:
		refusal = generated.LearningPathNotPublishableError(resp)
	default:
		return fmt.Errorf("expected a not-publishable refusal, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	got := make([]string, len(refusal.Missing))
	for i, m := range refusal.Missing {
		got[i] = string(m)
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("expected missing %v, got %v", want, got)
	}
	if unpublishedNode == "" {
		return nil
	}
	if refusal.UnpublishedContentNodeIds == nil || !slices.Contains(*refusal.UnpublishedContentNodeIds, nodeID(unpublishedNode)) {
		return fmt.Errorf("expected the refusal to name %q, got %v", unpublishedNode, refusal.UnpublishedContentNodeIds)
	}
	return nil
}

func replaceItem(nodeSlug string) struct {
	ContentNodeId uuid.UUID `json:"content_node_id"`
	SectionLabel  *string   `json:"section_label,omitempty"`
} {
	return struct {
		ContentNodeId uuid.UUID `json:"content_node_id"`
		SectionLabel  *string   `json:"section_label,omitempty"`
	}{ContentNodeId: nodeID(nodeSlug)}
}

// replaceSeededPath resends a path's whole current state, the way an editor
// saves it, after edit changes one part of it.
func (w *world) replaceSeededPath(name, slug string, edit func(*generated.ReplaceLearningPathRequest)) error {
	w.lastPathSlug = slug
	path, err := w.seededPath(slug)
	if err != nil {
		return err
	}
	level := generated.ReplaceLearningPathRequestLevelBeginner
	if path.Level != nil {
		level = generated.ReplaceLearningPathRequestLevel(*path.Level)
	}
	body := &generated.ReplaceLearningPathRequest{
		Title: path.Title, Summary: path.Summary, Language: path.Language, Level: level,
		InstrumentIds: toUUIDList(path.InstrumentIDs), ThumbnailUrl: path.ThumbnailURL,
	}
	for _, item := range path.Items {
		entry := replaceItem("")
		entry.ContentNodeId = uuid.MustParse(item.ContentNodeID)
		entry.SectionLabel = item.SectionLabel
		body.Items = append(body.Items, entry)
	}
	edit(body)
	resp, err := w.handler.ReplaceLearningPath(w.actorCtx(name), generated.ReplaceLearningPathRequestObject{LearningPathId: pathID(slug), Body: body})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) pathSavedWithoutSummary() error {
	replaced, ok := w.lastResp.(generated.ReplaceLearningPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a replaced learning path, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if replaced.Summary != nil {
		return fmt.Errorf("expected no summary, got %q", *replaced.Summary)
	}
	return nil
}

func (w *world) createsPathWith(name, title string, summary, language *string, level string, nodeSlugs ...string) error {
	if len(nodeSlugs) == 0 {
		nodeSlug := strings.ToLower(strings.ReplaceAll(title, " ", "-")) + "-lesson"
		if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
			return err
		}
		nodeSlugs = []string{nodeSlug}
	}
	body := &generated.CreateLearningPathRequest{
		Title: title, Summary: summary, Language: language, Level: generated.CreateLearningPathRequestLevel(level), InstrumentIds: []uuid.UUID{},
	}
	for _, slug := range nodeSlugs {
		body.Items = append(body.Items, replaceItem(slug))
	}
	resp, err := w.handler.CreateLearningPath(w.actorCtx(name), generated.CreateLearningPathRequestObject{Body: body})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) createdPathField(field string, get func(generated.LearningPath) *string, want string) error {
	created, ok := w.lastResp.(generated.CreateLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a created learning path, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	got := get(generated.LearningPath(created))
	if got == nil || *got != want {
		return fmt.Errorf("expected %s %q, got %v", field, want, got)
	}
	return nil
}

func (w *world) createdPathHasNoSummaryOrLanguage() error {
	created, ok := w.lastResp.(generated.CreateLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a created learning path, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if created.Summary != nil || created.Language != nil {
		return fmt.Errorf("expected no summary and no language, got %v and %v", created.Summary, created.Language)
	}
	return nil
}

func (w *world) libraryEntryReportsStatus(slug, want string) error {
	page, ok := w.lastResp.(generated.ListLearningPaths200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a learning path list, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, p := range page.Items {
		if p.LearningPathId == pathID(slug) {
			if string(p.Status) != want {
				return fmt.Errorf("expected %q to report %q, got %q", slug, want, p.Status)
			}
			return nil
		}
	}
	return fmt.Errorf("expected the list to include %q", slug)
}

func (w *world) refusalNamesDraftPath(slug string) error {
	refusal, ok := w.lastResp.(generated.PublishCourse409JSONResponse)
	if !ok {
		return fmt.Errorf("expected a course publish refusal, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if !slices.Contains(refusal.DraftLearningPathIds, pathID(slug)) {
		return fmt.Errorf("expected the refusal to name %q, got %v", slug, refusal.DraftLearningPathIds)
	}
	return nil
}

func (w *world) courseIsStillDraft(courseSlug string) error {
	course, err := w.courses.GetByID(context.Background(), w.courseIDBySlug[courseSlug].String())
	if err != nil {
		return err
	}
	if course.Status != domain.CourseStatusDraft {
		return fmt.Errorf("expected %q to be a draft, got %q", courseSlug, course.Status)
	}
	return nil
}

// courseDraftAddsCheckpoint appends a checkpoint for pathSlug to the course's
// live draft, leaving its published versions as they were.
func (w *world) courseDraftAddsCheckpoint(courseSlug, pathSlug string) error {
	course, err := w.courses.GetByID(context.Background(), w.courseIDBySlug[courseSlug].String())
	if err != nil {
		return err
	}
	course.Checkpoints = append(course.Checkpoints, domain.CourseCheckpoint{
		Position: len(course.Checkpoints) + 1, LearningPathID: pathID(pathSlug).String(), EffectiveTitle: pathSlug,
	})
	return w.courses.Replace(context.Background(), course)
}

func (w *world) learnersSeeEarlierVersion() error {
	version, err := w.courseVersions.GetLatestByCourseID(context.Background(), w.courseIDBySlug[w.lastCourseSlug].String())
	if err != nil {
		return err
	}
	if version.VersionNumber != 1 {
		return fmt.Errorf("expected learners to still see version 1, got version %d", version.VersionNumber)
	}
	return nil
}

func (w *world) hasNoCopyOf(name, slug string) error {
	held, err := w.studentPaths.ListStandaloneByStudentID(context.Background(), w.ensureRegistered(name, domain.RoleStudent).String())
	if err != nil {
		return err
	}
	for _, sp := range held {
		if sp.SourceTemplateID == pathID(slug).String() {
			return fmt.Errorf("expected %q to hold no copy of %q, found %s", name, slug, sp.ID)
		}
	}
	return nil
}
