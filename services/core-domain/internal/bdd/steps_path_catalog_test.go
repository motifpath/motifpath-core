//go:build integration

package bdd

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerPathCatalogSteps(sc *godog.ScenarioContext, w *world) {
	// ── Seeding published and draft paths ───────────────────────────────────
	sc.Step(`^a learning path "([^"]+)" exists as a draft, created by "([^"]+)"$`, func(slug, creator string) error {
		return w.putPathFor(slug, creator, domain.LearningPathStatusDraft)
	})
	sc.Step(`^a learning path "([^"]+)" exists, published, created by "([^"]+)"$`, func(slug, creator string) error {
		return w.putPathFor(slug, creator, domain.LearningPathStatusPublished)
	})
	sc.Step(`^a learning path "([^"]+)" exists, published, created by "([^"]+)", with (\d+) lessons, a summary, language "([^"]+)" and level "([^"]+)"$`, func(slug, creator string, lessons int, language, level string) error {
		if err := w.putPublishedPathWithLessons(slug, creator, lessons); err != nil {
			return err
		}
		return w.editSeededPath(slug, func(p *domain.LearningPath) {
			l := domain.DifficultyLevel(level)
			p.Language, p.Level = &language, &l
		})
	})
	sc.Step(`^a learning path "([^"]+)" exists, published, created by "([^"]+)", with (\d+) lessons$`, w.putPublishedPathWithLessons)
	sc.Step(`^(\d+) learning paths exist, published$`, func(count int) error {
		return w.bulkPublishedPaths("bulk-path-", count, domain.DifficultyLevelBeginner)
	})
	sc.Step(`^(\d+) learning paths exist, published, at level "([^"]+)" and (\d+) at level "([^"]+)"$`, func(countA int, levelA string, countB int, levelB string) error {
		if err := w.bulkPublishedPaths(levelA+"-path-", countA, domain.DifficultyLevel(levelA)); err != nil {
			return err
		}
		return w.bulkPublishedPaths(levelB+"-path-", countB, domain.DifficultyLevel(levelB))
	})
	sc.Step(`^a learning path "([^"]+)" exists, published, titled "([^"]+)"$`, func(slug, title string) error {
		return w.putPublishedPathEdited(slug, func(p *domain.LearningPath) { p.Title = title })
	})
	sc.Step(`^learning paths exist, published, at levels "([^"]+)", "([^"]+)" and "([^"]+)"$`, func(a, b, c string) error {
		for _, level := range []string{a, b, c} {
			l := domain.DifficultyLevel(level)
			if err := w.putPublishedPathEdited(level+"-path", func(p *domain.LearningPath) { p.Level = &l }); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^a learning path "([^"]+)" exists, published in language "([^"]+)"$`, func(slug, language string) error {
		return w.putPublishedPathEdited(slug, func(p *domain.LearningPath) { p.Language = &language })
	})
	sc.Step(`^a learning path "([^"]+)" exists, published, with a content node classified with skill "([^"]+)"$`, func(slug, skill string) error {
		if err := w.putPathFor(slug, "bob", domain.LearningPathStatusPublished); err != nil {
			return err
		}
		return w.classifyFirstNodeOfPath(slug, "skill", skill)
	})
	sc.Step(`^a learning path "([^"]+)" exists, published, with a content node classified with skill "([^"]+)" and concept "([^"]+)"$`, func(slug, skill, concept string) error {
		if err := w.putPathFor(slug, "bob", domain.LearningPathStatusPublished); err != nil {
			return err
		}
		if err := w.classifyFirstNodeOfPath(slug, "skill", skill); err != nil {
			return err
		}
		return w.classifyFirstNodeOfPath(slug, "concept", concept)
	})
	sc.Step(`^a learning path "([^"]+)" exists, published, with one content node classified with skill "([^"]+)" and another with concept "([^"]+)"$`, w.putPathSplitClassification)
	sc.Step(`^a learning path "([^"]+)" exists, published, for instrument "([^"]+)"$`, func(slug, instrument string) error {
		return w.putPublishedPathEdited(slug, func(p *domain.LearningPath) { p.InstrumentIDs = []string{instrumentID(instrument).String()} })
	})
	sc.Step(`^a learning path "([^"]+)" exists, published, for every instrument$`, func(slug string) error {
		return w.putPublishedPathEdited(slug, func(p *domain.LearningPath) { p.InstrumentIDs = nil })
	})
	sc.Step(`^a learning path "([^"]+)" exists, published, with lessons "([^"]+)" and "([^"]+)" in section "([^"]+)" and "([^"]+)" in section "([^"]+)"$`, w.putPublishedPathWithSections)
	sc.Step(`^learning paths exist, published, created by "([^"]+)", "([^"]+)" and "([^"]+)"$`, func(a, b, c string) error {
		return w.putPublishedPathsByNamedCreators(a, b, c)
	})
	sc.Step(`^learning paths exist, published, created by "([^"]+)" and "([^"]+)"$`, func(a, b string) error {
		return w.putPublishedPathsByNamedCreators(a, b)
	})

	// ── Browsing ────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" lists the path catalog$`, func(name string) error { return w.listsPathCatalogWith(name, "") })
	sc.Step(`^"([^"]+)" lists the path catalog(.+)$`, w.listsPathCatalogWith)
	sc.Step(`^an unauthenticated request attempts to list the path catalog$`, func() error {
		resp, err := w.handler.ListCatalogPaths(context.Background(), generated.ListCatalogPathsRequestObject{})
		w.lastResp, w.lastErr = resp, err
		return nil
	})
	sc.Step(`^the path catalog no longer includes "([^"]+)"$`, func(slug string) error {
		if err := w.listsPathCatalogWith("admin", ""); err != nil {
			return err
		}
		return w.responseDoesNotInclude(slug)
	})
	sc.Step(`^the response includes the "([^"]+)" and "([^"]+)" paths$`, func(a, b string) error {
		return w.responseIncludesTwo(a+"-path", b+"-path")
	})
	sc.Step(`^the response does not include the "([^"]+)" path$`, func(level string) error {
		return w.responseDoesNotInclude(level + "-path")
	})
	sc.Step(`^the response is an empty page with a total of 0$`, func() error {
		if err := w.responseContainsItems(0, ""); err != nil {
			return err
		}
		return w.responseReportsTotal(0)
	})
	sc.Step(`^the entry for "([^"]+)" reports its title, summary, language "([^"]+)", level "([^"]+)" and (\d+) lessons$`, w.catalogEntryReports)
	sc.Step(`^the entry for "([^"]+)" does not include any item's content_node_id$`, func(slug string) error {
		_, err := w.catalogEntry(slug)
		return err
	})

	// ── Detail ──────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" retrieves catalog path "([^"]+)"$`, func(name, slug string) error {
		resp, err := w.handler.GetCatalogPath(w.actorCtx(name), generated.GetCatalogPathRequestObject{LearningPathId: pathID(slug)})
		w.lastResp, w.lastErr = resp, err
		return nil
	})
	sc.Step(`^"([^"]+)" retrieves a catalog path ID that does not exist$`, func(name string) error {
		resp, err := w.handler.GetCatalogPath(w.actorCtx(name), generated.GetCatalogPathRequestObject{LearningPathId: uuid.New()})
		w.lastResp, w.lastErr = resp, err
		return nil
	})
	sc.Step(`^the response lists the lesson titles "([^"]+)", "([^"]+)" and "([^"]+)" in order, each with its section$`, w.detailListsLessonsWithSections)
	sc.Step(`^the response does not include any item's lesson content or content_node_id$`, func() error {
		_, err := w.pathDetail()
		return err
	})
	sc.Step(`^the response identifies "([^"]+)" as the path creator$`, func(creator string) error {
		detail, err := w.pathDetail()
		if err != nil {
			return err
		}
		if want := w.ensureRegistered(creator, domain.RoleTeacher); detail.CreatedBy.UserId != want {
			return fmt.Errorf("expected creator %s, got %s", want, detail.CreatedBy.UserId)
		}
		return nil
	})
	sc.Step(`^the response reports (\d+) lessons$`, func(lessons int) error {
		detail, err := w.pathDetail()
		if err != nil {
			return err
		}
		if detail.LessonCount != lessons || len(detail.Items) != lessons {
			return fmt.Errorf("expected %d lessons, got lesson_count %d and %d items", lessons, detail.LessonCount, len(detail.Items))
		}
		return nil
	})

	// ── Creators ────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" lists the path creators$`, func(string) error { return w.listPathCreators(nil) })
	sc.Step(`^"([^"]+)" lists the path creators matching "([^"]+)"$`, func(_, q string) error { return w.listPathCreators(&q) })
	sc.Step(`^an unauthenticated request attempts to list the path creators$`, func() error {
		resp, err := w.handler.ListCatalogPathCreators(context.Background(), generated.ListCatalogPathCreatorsRequestObject{})
		w.lastResp, w.lastErr = resp, err
		return nil
	})
	sc.Step(`^the creators returned are ((?:"[^"]+"(?:, | and )?)+), in that order$`, w.creatorsReturnedAre)

	// ── Library creators (authoring) ────────────────────────────────────────
	sc.Step(`^"([^"]+)" lists the creators of the learning path library$`, func(string) error { return w.listLibraryPathCreators(nil) })
	sc.Step(`^"([^"]+)" lists the creators of the learning path library matching "([^"]+)"$`, func(_, q string) error { return w.listLibraryPathCreators(&q) })
	sc.Step(`^an unauthenticated request attempts to list the creators of the learning path library$`, func() error {
		resp, err := w.handler.ListLearningPathCreators(context.Background(), generated.ListLearningPathCreatorsRequestObject{})
		w.lastResp, w.lastErr = resp, err
		return nil
	})
}

// putPublishedPathEdited seeds a complete published path, then applies edit.
func (w *world) putPublishedPathEdited(slug string, edit func(*domain.LearningPath)) error {
	if err := w.putPathFor(slug, "bob", domain.LearningPathStatusPublished); err != nil {
		return err
	}
	return w.editSeededPath(slug, edit)
}

// putPublishedPathWithLessons seeds a published path holding lessons
// published lessons.
func (w *world) putPublishedPathWithLessons(slug, creator string, lessons int) error {
	if err := w.putPathFor(slug, creator, domain.LearningPathStatusPublished); err != nil {
		return err
	}
	items := make([]domain.LearningPathItem, lessons)
	for i := range items {
		nodeSlug := fmt.Sprintf("%s-lesson-%d", slug, i+1)
		if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
			return err
		}
		w.autoPublishNode(nodeSlug)
		items[i] = domain.LearningPathItem{Position: i + 1, ContentNodeID: nodeID(nodeSlug).String(), Title: nodeSlug, ContentType: domain.ContentTypeVideo}
	}
	return w.editSeededPath(slug, func(p *domain.LearningPath) { p.Items = items })
}

func (w *world) bulkPublishedPaths(prefix string, count int, level domain.DifficultyLevel) error {
	for i := 1; i <= count; i++ {
		slug := padded(prefix, i, 3)
		if err := w.putPublishedPathEdited(slug, func(p *domain.LearningPath) { p.Level = &level }); err != nil {
			return err
		}
	}
	return nil
}

// putPathSplitClassification seeds a published path whose first node carries
// the skill and whose second carries the concept — so no single node has both.
func (w *world) putPathSplitClassification(slug, skill, concept string) error {
	if err := w.putPublishedPathWithLessons(slug, "bob", 2); err != nil {
		return err
	}
	if err := w.classifyFirstNodeOfPath(slug, "skill", skill); err != nil {
		return err
	}
	path, err := w.seededPath(slug)
	if err != nil {
		return err
	}
	node, err := w.nodes.GetByID(context.Background(), path.Items[1].ContentNodeID)
	if err != nil {
		return err
	}
	node.Classification.Concepts = append(node.Classification.Concepts, domain.KnowledgeNode{ID: w.conceptIDFor(concept).String()})
	w.nodes.put(node)
	return nil
}

func (w *world) putPublishedPathWithSections(slug, first, second, sectionA, third, sectionB string) error {
	if err := w.putPublishedPathWithLessons(slug, "bob", 3); err != nil {
		return err
	}
	titles := []string{first, second, third}
	sections := []string{sectionA, sectionA, sectionB}
	return w.editSeededPath(slug, func(p *domain.LearningPath) {
		for i := range p.Items {
			p.Items[i].Title = titles[i]
			section := sections[i]
			p.Items[i].SectionLabel = &section
		}
	})
}

// putPublishedPathsByNamedCreators seeds one published path per creator,
// each creator's display name being exactly the name given.
func (w *world) putPublishedPathsByNamedCreators(names ...string) error {
	for i, name := range names {
		w.nameClaims[name] = name
		if err := w.putPathFor("path-by-"+strconv.Itoa(i), name, domain.LearningPathStatusPublished); err != nil {
			return err
		}
	}
	return nil
}

// catalogListTail matches a catalog step's trailing clauses that aren't a
// "filtered by" clause.
var catalogListTail = regexp.MustCompile(`^( with limit \d+( and offset \d+)?| matching text "[^"]*"| filtered by .*)?$`)

func (w *world) listsPathCatalogWith(name, tail string) error {
	if !catalogListTail.MatchString(tail) {
		return fmt.Errorf("unsupported path catalog clauses %q", tail)
	}
	query, err := w.parseCourseListQuery(tail)
	if err != nil {
		return err
	}
	params := generated.ListCatalogPathsParams{
		Limit: query.limit, Offset: query.offset, Q: query.q,
		SkillIds: query.skillIDs, ConceptIds: query.conceptIDs, CreatedBy: query.createdBy, Language: query.language, InstrumentId: query.instrument,
	}
	if query.levels != nil {
		levels := make([]generated.ListCatalogPathsParamsLevels, len(query.levels))
		for i, v := range query.levels {
			levels[i] = generated.ListCatalogPathsParamsLevels(v)
		}
		params.Levels = &levels
	}
	resp, err := w.handler.ListCatalogPaths(w.actorCtx(name), generated.ListCatalogPathsRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, err
	return nil
}

func (w *world) catalogEntry(slug string) (generated.PathCatalogEntry, error) {
	page, ok := w.lastResp.(generated.ListCatalogPaths200JSONResponse)
	if !ok {
		return generated.PathCatalogEntry{}, fmt.Errorf("expected a path catalog page, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, entry := range page.Items {
		if entry.LearningPathId == pathID(slug) {
			return entry, nil
		}
	}
	return generated.PathCatalogEntry{}, fmt.Errorf("expected the catalog to include %q", slug)
}

func (w *world) catalogEntryReports(slug, language, level string, lessons int) error {
	entry, err := w.catalogEntry(slug)
	if err != nil {
		return err
	}
	if entry.Title != slug || entry.Summary == "" || entry.Language != language || string(entry.Level) != level || entry.LessonCount != lessons {
		return fmt.Errorf("unexpected catalog entry for %q: %#v", slug, entry)
	}
	return nil
}

func (w *world) pathDetail() (generated.GetCatalogPath200JSONResponse, error) {
	detail, ok := w.lastResp.(generated.GetCatalogPath200JSONResponse)
	if !ok {
		return detail, fmt.Errorf("expected a catalog path detail, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return detail, nil
}

func (w *world) detailListsLessonsWithSections(first, second, third string) error {
	detail, err := w.pathDetail()
	if err != nil {
		return err
	}
	want := []string{first, second, third}
	if len(detail.Items) != len(want) {
		return fmt.Errorf("expected %d lessons, got %d", len(want), len(detail.Items))
	}
	for i, item := range detail.Items {
		if item.Title != want[i] || item.SectionLabel == nil {
			return fmt.Errorf("expected lesson %d to be %q with a section, got %#v", i+1, want[i], item)
		}
	}
	return nil
}

// listLibraryPathCreators lists the authoring library's creators, storing a
// successful response as the course creators' type like listPathCreators.
func (w *world) listLibraryPathCreators(q *string) error {
	resp, err := w.handler.ListLearningPathCreators(w.ctx(), generated.ListLearningPathCreatorsRequestObject{Params: generated.ListLearningPathCreatorsParams{Q: q}})
	if creators, ok := resp.(generated.ListLearningPathCreators200JSONResponse); ok {
		w.lastResp, w.lastErr = generated.ListCourseCreators200JSONResponse(creators), err
		return nil
	}
	w.lastResp, w.lastErr = resp, err
	return nil
}

// listPathCreators stores a successful response as the course creators'
// type — both are a plain UserRef list — so the creator assertions read it.
func (w *world) listPathCreators(q *string) error {
	resp, err := w.handler.ListCatalogPathCreators(w.ctx(), generated.ListCatalogPathCreatorsRequestObject{Params: generated.ListCatalogPathCreatorsParams{Q: q}})
	if creators, ok := resp.(generated.ListCatalogPathCreators200JSONResponse); ok {
		w.lastResp, w.lastErr = generated.ListCourseCreators200JSONResponse(creators), err
		return nil
	}
	w.lastResp, w.lastErr = resp, err
	return nil
}
