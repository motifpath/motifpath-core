//go:build integration

package bdd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerLearningPathLibrarySteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)" and no level recorded$`, func(slug, node string) error {
		return w.putLibraryPath(slug, node, func(*domain.LearningPath) {})
	})
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", last updated on "([^"]+)"$`, w.putPathUpdatedOn)
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", created by "([^"]+)"$`, func(slug, node, creator string) error {
		teacher := w.ensureRegistered(creator, domain.RoleTeacher).String()
		return w.putLibraryPath(slug, node, func(p *domain.LearningPath) { p.TeacherID = teacher })
	})
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", at level "([^"]+)"$`, func(slug, node, level string) error {
		l := domain.DifficultyLevel(level)
		return w.putLibraryPath(slug, node, func(p *domain.LearningPath) { p.Level = &l })
	})
	sc.Step(`^content node "([^"]+)" is classified with skill "([^"]+)"$`, func(node, skill string) error {
		return w.classifyNode(node, skill, "")
	})
	sc.Step(`^content node "([^"]+)" is classified with skill "([^"]+)" and concept "([^"]+)"$`, w.classifyNode)

	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)" at level "([^"]+)" with items in order: "([^"]+)", "([^"]+)"$`, w.createsPathAtLevel)
	sc.Step(`^"([^"]+)" submits a create learning path request with the level field omitted$`, w.submitsPathWithoutLevel)
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" at level "([^"]+)" with items in order: "([^"]+)"$`, w.replacesPathAtLevel)
	sc.Step(`^"([^"]+)" lists learning paths filtered by (.+)$`, w.listsPathsFilteredBy)
	sc.Step(`^"([^"]+)" lists learning paths sorted by most recently updated$`, func(string) error {
		return w.listPaths(generated.ListLearningPathsParams{Sort: sortParam("updated")})
	})
	sc.Step(`^"([^"]+)" lists learning paths sorted by "([^"]+)"$`, func(_, sort string) error {
		return w.listPaths(generated.ListLearningPathsParams{Sort: sortParam(sort)})
	})

	sc.Step(`^the learning path's level is "([^"]+)"$`, w.learningPathLevelIs)
	sc.Step(`^the learning path has no level$`, w.learningPathHasNoLevel)
	sc.Step(`^the learning path's last update is later than "([^"]+)"$`, w.learningPathUpdatedAfter)
}

// putLibraryPath seeds a one-item path titled after its slug, created by
// "bob" at fixedNow with no level recorded, then lets adjust change it. The
// item's content node is created only if it doesn't exist yet, so a
// classification an earlier step gave it survives.
func (w *world) putLibraryPath(slug, nodeSlug string, adjust func(*domain.LearningPath)) error {
	if _, err := w.nodes.GetByID(context.Background(), nodeID(nodeSlug).String()); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
			return err
		}
	}
	path := domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: w.ensureRegistered("bob", domain.RoleTeacher).String(),
		Title:     slug,
		Items:     []domain.LearningPathItem{{Position: 1, ContentNodeID: nodeID(nodeSlug).String(), Title: nodeSlug, ContentType: domain.ContentTypeVideo}},
		CreatedAt: fixedNow,
		UpdatedAt: fixedNow,
	}
	adjust(&path)
	w.paths.put(path)
	return nil
}

func (w *world) putPathUpdatedOn(slug, nodeSlug, date string) error {
	updated, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return err
	}
	return w.putLibraryPath(slug, nodeSlug, func(p *domain.LearningPath) {
		p.CreatedAt, p.UpdatedAt = updated, updated
	})
}

// classifyNode replaces a content node's skills (and concepts, when concept
// is given) with the named ones.
func (w *world) classifyNode(nodeSlug, skill, concept string) error {
	node, err := w.nodes.GetByID(context.Background(), nodeID(nodeSlug).String())
	if err != nil {
		return err
	}
	node.Classification.Skills = []domain.Skill{{ID: w.skillIDFor(skill).String(), Name: skill}}
	if concept != "" {
		node.Classification.Concepts = []domain.Concept{{ID: w.conceptIDFor(concept).String(), Name: concept}}
	}
	w.nodes.put(node)
	return nil
}

func (w *world) createsPathAtLevel(_, title, level, n1, n2 string) error {
	return w.createsLearningPathAt(generated.CreateLearningPathRequestLevel(level), title, []pathItemSpec{{slug: n1}, {slug: n2}})
}

// submitsPathWithoutLevel sends the level at its zero value, which is how an
// omitted JSON field decodes.
func (w *world) submitsPathWithoutLevel(string) error {
	return w.createsLearningPathAt("", "Beginner Guitar", []pathItemSpec{{slug: "node-01"}})
}

func (w *world) replacesPathAtLevel(_, slug, level, n1 string) error {
	return w.replacesLearningPathAt(generated.ReplaceLearningPathRequestLevel(level), slug, []pathItemSpec{{slug: n1}})
}

func sortParam(sort string) *generated.ListLearningPathsParamsSort {
	s := generated.ListLearningPathsParamsSort(sort)
	return &s
}

func (w *world) listPaths(params generated.ListLearningPathsParams) error {
	resp, err := w.handler.ListLearningPaths(w.ctx(), generated.ListLearningPathsRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, err
	return err
}

// listsPathsFilteredBy reads creator, level(s), skill(s) and concept(s)
// clauses, each holding quoted values, like the course-list steps do.
func (w *world) listsPathsFilteredBy(_, clauses string) error {
	var params generated.ListLearningPathsParams
	for _, m := range filterClauses.FindAllStringSubmatch(clauses, -1) {
		values := quotedValues(m[2])
		switch m[1] {
		case "creator":
			id := w.ensureRegistered(values[0], domain.RoleTeacher)
			params.CreatedBy = &id
		case "level", "levels":
			levels := make([]generated.ListLearningPathsParamsLevels, len(values))
			for i, v := range values {
				levels[i] = generated.ListLearningPathsParamsLevels(v)
			}
			params.Levels = &levels
		case "skill", "skills":
			ids := w.skillIDsForNames(values)
			params.SkillIds = &ids
		case "concept", "concepts":
			ids := w.conceptIDsForNames(values)
			params.ConceptIds = &ids
		case "instrument":
			id := instrumentID(values[0])
			params.InstrumentId = &id
		default:
			return fmt.Errorf("unsupported learning path filter %q", m[1])
		}
	}
	return w.listPaths(params)
}

func (w *world) learningPathLevelIs(want string) error {
	resp, ok := w.lastResp.(generated.CreateLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.Level == nil || string(*resp.Level) != want {
		return fmt.Errorf("expected the learning path's level to be %q, got %v", want, resp.Level)
	}
	return nil
}

func (w *world) learningPathHasNoLevel() error {
	resp, ok := w.lastResp.(generated.GetLearningPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a learning path, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.Level != nil {
		return fmt.Errorf("expected no level, got %q", *resp.Level)
	}
	return nil
}

func (w *world) learningPathUpdatedAfter(date string) error {
	resp, ok := w.lastResp.(generated.ReplaceLearningPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a replaced learning path, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	earliest, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return err
	}
	if !resp.UpdatedAt.After(earliest) {
		return fmt.Errorf("expected the last update to be after %s, got %s", date, resp.UpdatedAt)
	}
	return nil
}
