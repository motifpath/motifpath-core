//go:build integration

package bdd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// registerListingSteps registers the steps shared by the paginated, filterable
// list endpoints (courses, learning paths, content nodes, exercises), the
// content node version history, and a student's standalone path list. It
// must be registered after the feature-specific steps: a few of its patterns
// are supersets of theirs, and godog runs the first definition that matches.
func registerListingSteps(sc *godog.ScenarioContext, w *world) {
	// ── Pagination assertions ────────────────────────────────────────────
	sc.Step(`^the response contains (\d+) items?( ordered by (?:title|name))?$`, w.responseContainsItems)
	sc.Step(`^the response reports a total of (\d+), a limit of (\d+), and an offset of (\d+)$`, w.responseReportsPage)
	sc.Step(`^the response reports a total of (\d+)$`, w.responseReportsTotal)
	sc.Step(`^the response does not include "([^"]+)" or "([^"]+)"$`, w.responseDoesNotIncludeEither)
	sc.Step(`^the request is refused with a validation error$`, w.requestRejectedInvalid)

	// ── Content nodes ────────────────────────────────────────────────────
	sc.Step(`^(\d+) content nodes exist in the library$`, w.bulkContentNodes)
	sc.Step(`^content nodes titled (.+) exist$`, w.contentNodesTitled)
	sc.Step(`^(\d+) article content nodes and (\d+) video content nodes titled "Chords N" exist$`, w.bulkChordsNodes)
	sc.Step(`^"([^"]+)" lists content nodes( with .*| of type .*| matching text .*)$`, w.listsContentNodesWith)

	// ── Exercises ────────────────────────────────────────────────────────
	sc.Step(`^(\d+) exercises exist in the pool$`, w.bulkExercises)
	sc.Step(`^(\d+) "([^"]+)" exercises and (\d+) "([^"]+)" exercises exist$`, w.bulkExercisesOfTwoTypes)
	sc.Step(`^"([^"]+)" lists exercises( with .*| of type .*)$`, w.listsExercisesWith)

	// ── Learning paths ───────────────────────────────────────────────────
	sc.Step(`^(\d+) learning paths exist in the library$`, w.bulkLearningPaths)
	sc.Step(`^learning paths titled (.+) exist$`, w.learningPathsTitled)
	sc.Step(`^"([^"]+)" lists learning paths( with .*| matching text .*)$`, w.listsLearningPathsWith)

	// ── Courses ──────────────────────────────────────────────────────────
	sc.Step(`^a course "([^"]+)" exists(?: (as a draft)|, (published))(, (?:created by|at level|titled|classified with skill) .*)$`, w.courseExistsWith)
	sc.Step(`^(\d+) courses exist, published$`, w.bulkCourses)
	sc.Step(`^courses exist, published, at levels (.+)$`, w.coursesAtLevels)
	sc.Step(`^(\d+) courses exist, published, at level "([^"]+)" and (\d+) at level "([^"]+)"$`, w.bulkCoursesAtTwoLevels)
	sc.Step(`^a content node in "([^"]+)" is classified with (skill|concept) "([^"]+)"(?: and (skill|concept) "([^"]+)")?$`, w.nodeInPathClassified)
	sc.Step(`^"([^"]+)" replaces the draft of "([^"]+)" with checkpoints "([^"]+)", "([^"]+)"$`, w.replacesDraftAs)
	sc.Step(`^"([^"]+)" lists the course catalog(.+)$`, w.listsCourseCatalogWith)
	sc.Step(`^the response includes the "([^"]+)" and "([^"]+)" courses$`, w.responseIncludesLevelCourses)
	sc.Step(`^the response does not include the "([^"]+)" course$`, w.responseExcludesLevelCourse)
	sc.Step(`^the entry for "([^"]+)" records "([^"]+)" as the creator$`, w.entryRecordsCreator)
	sc.Step(`^"([^"]+)" lists the course creators$`, w.listsCourseCreators)
	sc.Step(`^"([^"]+)" lists the course creators whose name matches "([^"]+)"$`, w.listsCourseCreatorsMatching)
	sc.Step(`^no creators are returned$`, w.noCreatorsReturned)
	sc.Step(`^an unauthenticated request attempts to list the course creators$`, w.unauthListsCourseCreators)
	sc.Step(`^the creators returned are "([^"]+)"(?: and "([^"]+)")?, each with their display name$`, w.creatorsReturnedAre)
	sc.Step(`^the creators returned do not include "([^"]+)"$`, w.creatorsReturnedExclude)

	// ── Content node version history ─────────────────────────────────────
	sc.Step(`^"([^"]+)" lists the versions of content node "([^"]+)"$`, w.listsContentNodeVersions)
	sc.Step(`^"([^"]+)" attempts to list the versions of content node "([^"]+)"$`, w.listsContentNodeVersions)
	sc.Step(`^"([^"]+)" attempts to list the versions of a content node with an ID that does not exist$`, w.listsVersionsOfMissingNode)
	sc.Step(`^an unauthenticated request attempts to list a content node's versions$`, w.unauthListsContentNodeVersions)
	sc.Step(`^the response contains versions (\d+) and (\d+), in that order$`, w.responseContainsVersionsInOrder)
	sc.Step(`^the response contains version (\d+)$`, w.responseContainsOnlyVersion)
	sc.Step(`^each version includes its title, classification, and published timestamp$`, w.eachVersionHasSnapshotFields)

	// ── A student's standalone paths ─────────────────────────────────────
	sc.Step(`^"([^"]+)" holds a standalone path "([^"]+)" that is (active|archived)$`, w.holdsStandalonePath)
	sc.Step(`^"([^"]+)" lists their standalone paths$`, w.listsStandalonePaths)
	sc.Step(`^"([^"]+)" attempts to list their standalone paths$`, w.listsStandalonePaths)
	sc.Step(`^an unauthenticated request attempts to list standalone paths$`, w.unauthListsStandalonePaths)
	sc.Step(`^the response does not include checkpoint 1 of "([^"]+)"$`, w.responseHasNoCourseCheckpoints)
}

// ── Parsing helpers ─────────────────────────────────────────────────────

var (
	quotedValue    = regexp.MustCompile(`"([^"]*)"`)
	limitClause    = regexp.MustCompile(`limit (-?\d+)`)
	offsetClause   = regexp.MustCompile(`offset (-?\d+)`)
	textClause     = regexp.MustCompile(`(?:matching )?text "([^"]*)"`)
	typeClause     = regexp.MustCompile(`of type "([^"]+)"`)
	filterClauses  = regexp.MustCompile(`(levels|level|skills|skill|concepts|concept|creator|text) ((?:"[^"]*"(?:, )?)+)`)
	quotedListItem = regexp.MustCompile(`"([^"]*)"`)
)

// quotedValues returns every double-quoted value in s, in order.
func quotedValues(s string) []string {
	matches := quotedValue.FindAllStringSubmatch(s, -1)
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = m[1]
	}
	return out
}

// pageParams reads "limit N" / "offset N" out of a step's trailing text.
func pageParams(tail string) (limit, offset *int, err error) {
	if m := limitClause.FindStringSubmatch(tail); m != nil {
		n, convErr := strconv.Atoi(m[1])
		if convErr != nil {
			return nil, nil, convErr
		}
		limit = &n
	}
	if m := offsetClause.FindStringSubmatch(tail); m != nil {
		n, convErr := strconv.Atoi(m[1])
		if convErr != nil {
			return nil, nil, convErr
		}
		offset = &n
	}
	return limit, offset, nil
}

func searchParam(tail string) *string {
	if m := textClause.FindStringSubmatch(tail); m != nil {
		return &m[1]
	}
	return nil
}

func padded(prefix string, n, width int) string {
	return fmt.Sprintf("%s%0*d", prefix, width, n)
}

// ── Pagination assertions ───────────────────────────────────────────────

// pageView is the type-independent shape of a paginated list response.
type pageView struct {
	titles               []string
	total, limit, offset int
	itemCount            int
	hasPageMeta          bool
}

func (w *world) currentPage() (pageView, error) {
	switch resp := w.lastResp.(type) {
	case generated.ListContentNodes200JSONResponse:
		titles := make([]string, len(resp.Items))
		for i, n := range resp.Items {
			titles[i] = n.Title
		}
		return pageView{titles, resp.Total, resp.Limit, resp.Offset, len(resp.Items), true}, nil
	case generated.ListLearningPaths200JSONResponse:
		titles := make([]string, len(resp.Items))
		for i, p := range resp.Items {
			titles[i] = p.Title
		}
		return pageView{titles, resp.Total, resp.Limit, resp.Offset, len(resp.Items), true}, nil
	case generated.ListCourses200JSONResponse:
		titles := make([]string, len(resp.Items))
		for i, c := range resp.Items {
			titles[i] = c.Title
		}
		return pageView{titles, resp.Total, resp.Limit, resp.Offset, len(resp.Items), true}, nil
	case generated.ListExercises200JSONResponse:
		return pageView{nil, resp.Total, resp.Limit, resp.Offset, len(resp.Items), true}, nil
	case generated.ListDiagrams200JSONResponse:
		names := make([]string, len(resp.Items))
		for i, d := range resp.Items {
			names[i] = domain.LocalizedText(d.Names).Resolve("en")
		}
		return pageView{names, resp.Total, resp.Limit, resp.Offset, len(resp.Items), true}, nil
	default:
		return pageView{}, fmt.Errorf("expected a paginated list response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) responseContainsItems(count int, ordered string) error {
	page, err := w.currentPage()
	if err != nil {
		return err
	}
	if page.itemCount != count {
		return fmt.Errorf("expected %d items, got %d", count, page.itemCount)
	}
	if ordered != "" && !sort.StringsAreSorted(page.titles) {
		return fmt.Errorf("expected items%s, got %v", ordered, page.titles)
	}
	return nil
}

func (w *world) responseReportsPage(total, limit, offset int) error {
	page, err := w.currentPage()
	if err != nil {
		return err
	}
	if page.total != total || page.limit != limit || page.offset != offset {
		return fmt.Errorf("expected total %d, limit %d, offset %d; got total %d, limit %d, offset %d",
			total, limit, offset, page.total, page.limit, page.offset)
	}
	return nil
}

func (w *world) responseReportsTotal(total int) error {
	page, err := w.currentPage()
	if err != nil {
		return err
	}
	if page.total != total {
		return fmt.Errorf("expected a total of %d, got %d", total, page.total)
	}
	return nil
}

func (w *world) responseDoesNotIncludeEither(a, b string) error {
	if err := w.responseDoesNotInclude(a); err != nil {
		return err
	}
	return w.responseDoesNotInclude(b)
}

// ── Content nodes ───────────────────────────────────────────────────────

func (w *world) bulkContentNodes(count int) error {
	for i := 1; i <= count; i++ {
		if err := w.putContentNode(padded("bulk-node-", i, 3), domain.ContentTypeVideo); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) contentNodesTitled(list string) error {
	for _, title := range quotedValues(list) {
		if err := w.putContentNode(title, domain.ContentTypeVideo); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) bulkChordsNodes(articles, videos int) error {
	for i := 1; i <= articles; i++ {
		if err := w.putContentNode(padded("Chords article ", i, 2), domain.ContentTypeArticle); err != nil {
			return err
		}
	}
	for i := 1; i <= videos; i++ {
		if err := w.putContentNode(padded("Chords video ", i, 2), domain.ContentTypeVideo); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) listsContentNodesWith(_ string, tail string) error {
	limit, offset, err := pageParams(tail)
	if err != nil {
		return err
	}
	params := generated.ListContentNodesParams{Limit: limit, Offset: offset, Q: searchParam(tail)}
	if m := typeClause.FindStringSubmatch(tail); m != nil {
		ct := generated.ListContentNodesParamsContentType(m[1])
		params.ContentType = &ct
	}
	resp, callErr := w.handler.ListContentNodes(w.ctx(), generated.ListContentNodesRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, callErr
	return callErr
}

// ── Exercises ───────────────────────────────────────────────────────────

func (w *world) putExerciseOfType(slug string, exerciseType domain.ExerciseType) {
	label := "option-" + slug
	w.exercises.put(domain.Exercise{
		ID:             exerciseID(slug).String(),
		Title:          "title-" + slug,
		Prompt:         domain.NewPlainTextPrompt("prompt-" + slug),
		ExerciseType:   exerciseType,
		Options:        []domain.Option{{ID: uuid.NewString(), IsCorrect: true, Label: &label}},
		ChallengeIDs:   []string{},
		ContentNodeIDs: []string{},
		CreatedAt:      fixedNow,
	})
}

func (w *world) bulkExercises(count int) error {
	for i := 1; i <= count; i++ {
		w.putExerciseOfType(padded("bulk-exercise-", i, 3), domain.ExerciseTypeTextResponse)
	}
	return nil
}

func (w *world) bulkExercisesOfTwoTypes(nA int, typeA string, nB int, typeB string) error {
	for i := 1; i <= nA; i++ {
		w.putExerciseOfType(padded(typeA+"-exercise-", i, 3), domain.ExerciseType(typeA))
	}
	for i := 1; i <= nB; i++ {
		w.putExerciseOfType(padded(typeB+"-exercise-", i, 3), domain.ExerciseType(typeB))
	}
	return nil
}

func (w *world) listsExercisesWith(_ string, tail string) error {
	limit, offset, err := pageParams(tail)
	if err != nil {
		return err
	}
	params := generated.ListExercisesParams{Limit: limit, Offset: offset}
	if m := typeClause.FindStringSubmatch(tail); m != nil {
		et := generated.ListExercisesParamsExerciseType(m[1])
		params.ExerciseType = &et
	}
	resp, callErr := w.handler.ListExercises(w.ctx(), generated.ListExercisesRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, callErr
	return callErr
}

// ── Learning paths ──────────────────────────────────────────────────────

func (w *world) bulkLearningPaths(count int) error {
	for i := 1; i <= count; i++ {
		if err := w.putLearningPathDefault(padded("bulk-path-", i, 3)); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) learningPathsTitled(list string) error {
	for _, title := range quotedValues(list) {
		if err := w.putLearningPathDefault(title); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) listsLearningPathsWith(_ string, tail string) error {
	limit, offset, err := pageParams(tail)
	if err != nil {
		return err
	}
	resp, callErr := w.handler.ListLearningPaths(w.ctx(), generated.ListLearningPathsRequestObject{
		Params: generated.ListLearningPathsParams{Limit: limit, Offset: offset, Q: searchParam(tail)},
	})
	w.lastResp, w.lastErr = resp, callErr
	return callErr
}

// ── Courses ─────────────────────────────────────────────────────────────

// courseSeed describes a course to create and, optionally, publish.
type courseSeed struct {
	slug, title, creator string
	level                generated.CreateCourseRequestLevel
	pathSlugs            []string
	publish              bool
}

// seedCourseFrom creates the course through the real handler — as its
// creator — publishing it through an admin when asked, and records its id
// under its slug. Unknown learning paths are created with a default node.
func (w *world) seedCourseFrom(seed courseSeed) (uuid.UUID, error) {
	for _, slug := range seed.pathSlugs {
		if _, err := w.paths.GetByID(context.Background(), pathID(slug).String()); err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				return uuid.UUID{}, err
			}
			if err := w.putLearningPathDefault(slug); err != nil {
				return uuid.UUID{}, err
			}
		}
	}

	w.ensureRegistered(seed.creator, domain.RoleTeacher)
	creatorCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub(seed.creator))
	specs := make([]courseCheckpointSpec, len(seed.pathSlugs))
	for i, slug := range seed.pathSlugs {
		specs[i] = courseCheckpointSpec{slug: slug}
	}
	resp, err := w.handler.CreateCourse(creatorCtx, generated.CreateCourseRequestObject{
		Body: &generated.CreateCourseRequest{
			Title: seed.title, Summary: "Seeded for testing", Level: seed.level, Checkpoints: toCourseCheckpointBody(specs),
		},
	})
	if err != nil {
		return uuid.UUID{}, err
	}
	created, ok := resp.(generated.CreateCourse201JSONResponse)
	if !ok {
		return uuid.UUID{}, fmt.Errorf("setup: expected course creation to succeed, got %#v", resp)
	}
	w.courseIDBySlug[seed.slug] = created.CourseId

	if seed.publish {
		adminName := "seed-admin-for-" + seed.slug
		w.ensureRegistered(adminName, domain.RoleAdmin)
		adminCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub(adminName))
		published, err := w.handler.PublishCourse(adminCtx, generated.PublishCourseRequestObject{CourseId: created.CourseId})
		if err != nil {
			return uuid.UUID{}, err
		}
		if _, ok := published.(generated.PublishCourse201JSONResponse); !ok {
			return uuid.UUID{}, fmt.Errorf("setup: expected course publish to succeed, got %#v", published)
		}
	}
	return created.CourseId, nil
}

var (
	createdByClause  = regexp.MustCompile(`, created by "([^"]+)"`)
	atLevelClause    = regexp.MustCompile(`, at level "([^"]+)"`)
	titledClause     = regexp.MustCompile(`, titled "([^"]+)"`)
	classifiedClause = regexp.MustCompile(`, classified with skill "([^"]+)"`)
	checkpointClause = regexp.MustCompile(`, with checkpoints ((?:"[^"]+"(?:, )?)+)`)
)

// firstMatch returns the first capture group of re in s, or "" if absent.
func firstMatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// courseExistsWith handles the "a course ... exists" variants that add a
// creator, level, title, classification skill and/or checkpoints after the
// status. An omitted creator is "bob", an omitted title is the slug, an
// omitted level is beginner and omitted checkpoints are one default path
// named for the course.
func (w *world) courseExistsWith(slug, draft, _ string, clauses string) error {
	creator := firstMatch(createdByClause, clauses)
	if creator == "" {
		creator = "bob"
	}
	title := firstMatch(titledClause, clauses)
	if title == "" {
		title = slug
	}
	level := firstMatch(atLevelClause, clauses)
	if level == "" {
		level = string(generated.CreateCourseRequestLevelBeginner)
	}
	pathSlugs := quotedValues(firstMatch(checkpointClause, clauses))
	if len(pathSlugs) == 0 {
		pathSlugs = []string{slug + "-path"}
	}
	if err := w.ensurePaths(pathSlugs); err != nil {
		return err
	}
	if skill := firstMatch(classifiedClause, clauses); skill != "" {
		if err := w.classifyFirstNodeOfPath(pathSlugs[0], "skill", skill); err != nil {
			return err
		}
	}
	_, err := w.seedCourseFrom(courseSeed{
		slug: slug, title: title, creator: creator, level: generated.CreateCourseRequestLevel(level),
		pathSlugs: pathSlugs, publish: draft == "",
	})
	return err
}

func (w *world) ensurePaths(slugs []string) error {
	for _, slug := range slugs {
		if _, err := w.paths.GetByID(context.Background(), pathID(slug).String()); err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				return err
			}
			if err := w.putLearningPathDefault(slug); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *world) bulkCourses(count int) error {
	for i := 1; i <= count; i++ {
		slug := padded("bulk-course-", i, 3)
		if _, err := w.seedCourseFrom(courseSeed{
			slug: slug, title: slug, creator: "bob", level: generated.CreateCourseRequestLevelBeginner,
			pathSlugs: []string{"bulk-course-path"}, publish: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func levelCourseSlug(level string) string { return "level-" + level + "-course" }

func (w *world) coursesAtLevels(list string) error {
	for _, level := range quotedValues(list) {
		slug := levelCourseSlug(level)
		if _, err := w.seedCourseFrom(courseSeed{
			slug: slug, title: slug, creator: "bob", level: generated.CreateCourseRequestLevel(level),
			pathSlugs: []string{"level-course-path"}, publish: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) bulkCoursesAtTwoLevels(nA int, levelA string, nB int, levelB string) error {
	for _, group := range []struct {
		count int
		level string
	}{{nA, levelA}, {nB, levelB}} {
		for i := 1; i <= group.count; i++ {
			slug := padded(group.level+"-course-", i, 3)
			if _, err := w.seedCourseFrom(courseSeed{
				slug: slug, title: slug, creator: "bob", level: generated.CreateCourseRequestLevel(group.level),
				pathSlugs: []string{"level-course-path"}, publish: true,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// classifyFirstNodeOfPath links the first content node of the learning path
// identified by pathSlug to the named skill or concept, in addition to what
// it is already linked to.
func (w *world) classifyFirstNodeOfPath(pathSlug, kind, name string) error {
	path, err := w.paths.GetByID(context.Background(), pathID(pathSlug).String())
	if err != nil {
		return fmt.Errorf("setup: learning path %q: %w", pathSlug, err)
	}
	if len(path.Items) == 0 {
		return fmt.Errorf("setup: learning path %q has no items", pathSlug)
	}
	node, err := w.nodes.GetByID(context.Background(), path.Items[0].ContentNodeID)
	if err != nil {
		return err
	}
	switch kind {
	case "skill":
		node.Classification.Skills = append(node.Classification.Skills, domain.Skill{ID: w.skillIDFor(name).String(), Name: name})
	case "concept":
		node.Classification.Concepts = append(node.Classification.Concepts, domain.Concept{ID: w.conceptIDFor(name).String(), Name: name})
	}
	w.nodes.put(node)
	return nil
}

func (w *world) nodeInPathClassified(pathSlug, kindA, nameA, kindB, nameB string) error {
	if err := w.ensurePaths([]string{pathSlug}); err != nil {
		return err
	}
	if err := w.classifyFirstNodeOfPath(pathSlug, kindA, nameA); err != nil {
		return err
	}
	if kindB == "" {
		return nil
	}
	return w.classifyFirstNodeOfPath(pathSlug, kindB, nameB)
}

// listsCourseCatalogWith parses a "lists the course catalog ..." step's
// trailing text into query parameters: paging ("with limit N and offset M"),
// text ("matching text ..."), and any "filtered by" clauses — level(s),
// skill(s), concept(s), creator and text — each holding quoted values.
func (w *world) listsCourseCatalogWith(_ string, tail string) error {
	limit, offset, err := pageParams(tail)
	if err != nil {
		return err
	}
	params := generated.ListCoursesParams{Limit: limit, Offset: offset, Q: searchParam(tail)}

	for _, m := range filterClauses.FindAllStringSubmatch(tail, -1) {
		values := quotedValues(m[2])
		switch m[1] {
		case "level", "levels":
			levels := make([]generated.ListCoursesParamsLevels, len(values))
			for i, v := range values {
				levels[i] = generated.ListCoursesParamsLevels(v)
			}
			params.Levels = &levels
		case "skill", "skills":
			ids := w.skillIDsForNames(values)
			params.SkillIds = &ids
		case "concept", "concepts":
			ids := w.conceptIDsForNames(values)
			params.ConceptIds = &ids
		case "creator":
			id := w.ensureRegistered(values[0], domain.RoleTeacher)
			params.CreatedBy = &id
		case "text":
			text := values[0]
			params.Q = &text
		}
	}

	resp, callErr := w.handler.ListCourses(w.ctx(), generated.ListCoursesRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, callErr
	return callErr
}

func (w *world) skillIDsForNames(names []string) []uuid.UUID {
	ids := make([]uuid.UUID, len(names))
	for i, n := range names {
		ids[i] = w.skillIDFor(n)
	}
	return ids
}

func (w *world) conceptIDsForNames(names []string) []uuid.UUID {
	ids := make([]uuid.UUID, len(names))
	for i, n := range names {
		ids[i] = w.conceptIDFor(n)
	}
	return ids
}

// courseMatchesSlug reports whether entry is the course seeded under slug —
// by id when the slug was seeded through a course step, else by exact title.
func (w *world) courseMatchesSlug(entry generated.CourseCatalogEntry, slug string) bool {
	if id, ok := w.courseIDBySlug[slug]; ok {
		return entry.CourseId == id
	}
	return entry.Title == slug
}

func (w *world) responseIncludesLevelCourses(levelA, levelB string) error {
	if err := w.responseIncludes(levelCourseSlug(levelA)); err != nil {
		return err
	}
	return w.responseIncludes(levelCourseSlug(levelB))
}

func (w *world) responseExcludesLevelCourse(level string) error {
	return w.responseDoesNotInclude(levelCourseSlug(level))
}

func (w *world) entryRecordsCreator(slug, creator string) error {
	resp, ok := w.lastResp.(generated.ListCourses200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a course list response, got %#v", w.lastResp)
	}
	want := w.ensureRegistered(creator, domain.RoleTeacher)
	for _, entry := range resp.Items {
		if w.courseMatchesSlug(entry, slug) {
			if entry.CreatedBy.UserId != want {
				return fmt.Errorf("expected %q's creator to be %s, got %s", slug, want, entry.CreatedBy.UserId)
			}
			return nil
		}
	}
	return fmt.Errorf("expected the response to include %q", slug)
}

func (w *world) listsCourseCreators(string) error {
	resp, err := w.handler.ListCourseCreators(w.ctx(), generated.ListCourseCreatorsRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsCourseCreatorsMatching(_, query string) error {
	resp, err := w.handler.ListCourseCreators(w.ctx(), generated.ListCourseCreatorsRequestObject{
		Params: generated.ListCourseCreatorsParams{Q: &query},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) noCreatorsReturned() error {
	resp, err := w.courseCreatorsResponse()
	if err != nil {
		return err
	}
	if len(resp) != 0 {
		return fmt.Errorf("expected no creators, got %#v", resp)
	}
	return nil
}

func (w *world) unauthListsCourseCreators() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsCourseCreators("")
}

func (w *world) courseCreatorsResponse() (generated.ListCourseCreators200JSONResponse, error) {
	resp, ok := w.lastResp.(generated.ListCourseCreators200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("expected a course creators response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return resp, nil
}

// creatorsReturnedAre asserts the response holds exactly the named creators,
// in the order given, each carrying their current display name.
func (w *world) creatorsReturnedAre(first, second string) error {
	resp, err := w.courseCreatorsResponse()
	if err != nil {
		return err
	}
	names := []string{first}
	if second != "" {
		names = append(names, second)
	}
	if len(resp) != len(names) {
		return fmt.Errorf("expected %d creators, got %d: %#v", len(names), len(resp), resp)
	}
	for i, name := range names {
		want := generated.UserRef{UserId: w.ensureRegistered(name, domain.RoleTeacher), DisplayName: w.nameClaim(name)}
		if resp[i] != want {
			return fmt.Errorf("expected creator %d to be %#v, got %#v", i+1, want, resp[i])
		}
	}
	return nil
}

func (w *world) creatorsReturnedExclude(name string) error {
	resp, err := w.courseCreatorsResponse()
	if err != nil {
		return err
	}
	excluded := w.ensureRegistered(name, domain.RoleTeacher)
	for _, ref := range resp {
		if ref.UserId == excluded {
			return fmt.Errorf("expected the creators not to include %q, got %#v", name, resp)
		}
	}
	return nil
}

// ── Content node version history ────────────────────────────────────────

func (w *world) listsContentNodeVersions(_, nodeSlug string) error {
	resp, err := w.handler.ListContentNodeVersions(w.ctx(), generated.ListContentNodeVersionsRequestObject{ContentNodeId: nodeID(nodeSlug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listsVersionsOfMissingNode(_ string) error {
	resp, err := w.handler.ListContentNodeVersions(w.ctx(), generated.ListContentNodeVersionsRequestObject{ContentNodeId: uuid.New()})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthListsContentNodeVersions() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsContentNodeVersions("", "node-01")
}

func (w *world) versionsResponse() (generated.ListContentNodeVersions200JSONResponse, error) {
	resp, ok := w.lastResp.(generated.ListContentNodeVersions200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("expected a version list response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return resp, nil
}

func (w *world) responseContainsVersionsInOrder(first, second int) error {
	resp, err := w.versionsResponse()
	if err != nil {
		return err
	}
	if len(resp) != 2 || resp[0].VersionNumber != first || resp[1].VersionNumber != second {
		return fmt.Errorf("expected versions %d then %d, got %+v", first, second, resp)
	}
	return nil
}

func (w *world) responseContainsOnlyVersion(version int) error {
	resp, err := w.versionsResponse()
	if err != nil {
		return err
	}
	if len(resp) != 1 || resp[0].VersionNumber != version {
		return fmt.Errorf("expected only version %d, got %+v", version, resp)
	}
	return nil
}

func (w *world) eachVersionHasSnapshotFields() error {
	resp, err := w.versionsResponse()
	if err != nil {
		return err
	}
	for _, v := range resp {
		if v.TitleSnapshot == "" && strings.TrimSpace(string(v.ClassificationSnapshot.DifficultyLevel)) == "" {
			return fmt.Errorf("version %d has neither a title nor a classification snapshot", v.VersionNumber)
		}
		if v.PublishedAt.IsZero() {
			return fmt.Errorf("version %d has no published timestamp", v.VersionNumber)
		}
	}
	return nil
}

// ── A student's standalone paths ────────────────────────────────────────

func (w *world) holdsStandalonePath(name, pathSlug, state string) error {
	if err := w.ensurePaths([]string{pathSlug}); err != nil {
		return err
	}
	if err := w.hasStandalonePathAssigned(name, pathSlug); err != nil {
		return err
	}
	if state == "archived" {
		return w.studentPaths.Archive(context.Background(), w.standalonePathIDByKey[name+"|"+pathSlug], fixedNow)
	}
	return nil
}

func (w *world) listsStandalonePaths(_ string) error {
	resp, err := w.handler.ListMyStandalonePaths(w.ctx(), generated.ListMyStandalonePathsRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthListsStandalonePaths() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsStandalonePaths("")
}

func (w *world) responseHasNoCourseCheckpoints(_ string) error {
	resp, ok := w.lastResp.(generated.ListMyStandalonePaths200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a standalone path list response, got %#v", w.lastResp)
	}
	for _, sp := range resp {
		if sp.SourceCourseEnrollmentId != nil {
			return fmt.Errorf("expected no course checkpoints in the list, got %+v", sp)
		}
	}
	return nil
}

// replacesDraftAs replaces a course's draft as the named user (an admin when
// the name is "admin", else a teacher) — authenticating as them first, since a Given step naming an actor
// otherwise runs with whatever identity (possibly none) came before it.
func (w *world) replacesDraftAs(name, courseSlug, p1, p2 string) error {
	role := domain.RoleTeacher
	if name == "admin" {
		role = domain.RoleAdmin
	}
	w.authenticateAs(name, role)
	return w.replacesCourseTwoCheckpoints(name, courseSlug, p1, p2)
}
