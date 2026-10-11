//go:build integration

package bdd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

// registerAuthoringListSteps covers sort, any-of and subtree classification
// filters, the lesson filters and creator counts on the authoring lists.
func registerAuthoringListSteps(sc *godog.ScenarioContext, w *world) {
	// ── Seeding ──────────────────────────────────────────────────────────────
	sc.Step(`^a content node "([^"]+)" exists, last changed on "([^"]+)"$`, func(slug, day string) error {
		return w.putAuthoredNode(slug, authoredNode{lastChanged: day})
	})
	sc.Step(`^a content node "([^"]+)" exists, created by "([^"]+)"$`, func(slug, creator string) error {
		return w.putAuthoredNode(slug, authoredNode{creator: creator})
	})
	sc.Step(`^a content node "([^"]+)" exists in languages "([^"]+)"(?: and "([^"]+)")?$`, func(slug, first, second string) error {
		languages := []string{first}
		if second != "" {
			languages = append(languages, second)
		}
		return w.putAuthoredNode(slug, authoredNode{languages: languages})
	})
	sc.Step(`^(\d+) content nodes? exists?, created by "([^"]+)"$`, func(count int, creator string) error {
		for i := 1; i <= count; i++ {
			if err := w.putAuthoredNode(fmt.Sprintf("%s-lesson-%d", creator, i), authoredNode{creator: creator}); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^(\d+) content nodes exist in the system with skills "([^"]+)"$`, func(count int, skills string) error {
		for i := 1; i <= count; i++ {
			if err := w.putContentNodeWithSkills(fmt.Sprintf("%s-lesson-%02d", slug(skills), i), skills); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^an exercise "([^"]+)" exists, last changed on "([^"]+)"$`, func(exSlug, day string) error {
		return w.changeExercise(exSlug, day)
	})
	sc.Step(`^(\d+) exercises exist, created by "([^"]+)"$`, func(count int, creator string) error {
		for i := 1; i <= count; i++ {
			if err := w.putExerciseWith(fmt.Sprintf("%s-exercise-%d", creator, i), exerciseSeed{createdBy: w.ensureRegistered(creator, domain.RoleTeacher).String()}); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^a basic diagram "([^"]+)" exists on instrument "([^"]+)", last changed on "([^"]+)"$`, func(diagramSlug, instrument, day string) error {
		if err := w.aDiagramExistsOn(diagramSlug, instrument); err != nil {
			return err
		}
		return w.changeDiagram(diagramSlug, day)
	})
	sc.Step(`^a custom diagram "([^"]+)" exists on instrument "([^"]+)", created by "([^"]+)", last changed on "([^"]+)"$`, func(diagramSlug, instrument, creator, day string) error {
		if err := w.aCustomDiagramExistsOn(diagramSlug, instrument, creator); err != nil {
			return err
		}
		return w.changeDiagram(diagramSlug, day)
	})
	sc.Step(`^a basic diagram "([^"]+)" exists on instrument "([^"]+)", classified under skills "([^"]+)", concepts "([^"]+)"$`, func(diagramSlug, instrument, skills, concepts string) error {
		if err := w.aDiagramExistsOn(diagramSlug, instrument); err != nil {
			return err
		}
		d, err := w.diagrams.GetByID(w.ctx(), diagramID(diagramSlug).String())
		if err != nil {
			return err
		}
		d.Skills = skillsFromUUIDs(w.skillIDsFor(skills))
		d.Concepts = nil
		for _, id := range w.conceptIDsFor(concepts) {
			d.Concepts = append(d.Concepts, domain.KnowledgeNode{ID: id.String()})
		}
		w.diagrams.put(d)
		return nil
	})
	sc.Step(`^(\d+) (basic|custom) diagrams exist on instrument "([^"]+)", created by "([^"]+)"$`, func(count int, kind, instrument, creator string) error {
		for i := 1; i <= count; i++ {
			diagramSlug := fmt.Sprintf("%s-%s-%d", creator, kind, i)
			var err error
			if kind == "basic" {
				err = w.aBasicDiagramCreatedBy(diagramSlug, instrument, creator)
			} else {
				err = w.aCustomDiagramExistsOn(diagramSlug, instrument, creator)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})

	// ── Listing ──────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" lists content nodes sorted by most recently changed$`, func(string) error {
		return w.listContentNodes(generated.ListContentNodesParams{Sort: ptrTo(generated.ListContentNodesParamsSortUpdated)})
	})
	sc.Step(`^"([^"]+)" lists content nodes filtered by skills "([^"]+)" and "([^"]+)"$`, func(_, first, second string) error {
		return w.listContentNodes(generated.ListContentNodesParams{SkillIds: &[]uuid.UUID{w.skillIDFor(first), w.skillIDFor(second)}})
	})
	sc.Step(`^"([^"]+)" lists content nodes filtered by skill "([^"]+)" with limit (\d+)$`, func(_, skill string, limit int) error {
		return w.listContentNodes(generated.ListContentNodesParams{SkillIds: &[]uuid.UUID{w.skillIDFor(skill)}, Limit: &limit})
	})
	sc.Step(`^"([^"]+)" lists content nodes filtered by a skill that does not exist$`, func(string) error {
		return w.listContentNodes(generated.ListContentNodesParams{SkillIds: &[]uuid.UUID{uuid.New()}})
	})
	sc.Step(`^"([^"]+)" lists content nodes filtered by creator "([^"]+)"$`, func(_, creator string) error {
		id := w.ensureRegistered(creator, domain.RoleTeacher)
		return w.listContentNodes(generated.ListContentNodesParams{CreatedBy: &id})
	})
	sc.Step(`^"([^"]+)" lists content nodes filtered by language "([^"]+)"$`, func(_, language string) error {
		return w.listContentNodes(generated.ListContentNodesParams{Language: &language})
	})
	sc.Step(`^"([^"]+)" lists content nodes filtered by status "([^"]+)"$`, func(_, status string) error {
		return w.listContentNodes(generated.ListContentNodesParams{Status: ptrTo(generated.ListContentNodesParamsStatus(status))})
	})
	sc.Step(`^"([^"]+)" lists exercises$`, func(string) error {
		return w.listExercises(generated.ListExercisesParams{})
	})
	sc.Step(`^"([^"]+)" lists exercises sorted by most recently changed$`, func(string) error {
		return w.listExercises(generated.ListExercisesParams{Sort: ptrTo(generated.ListExercisesParamsSortUpdated)})
	})
	sc.Step(`^"([^"]+)" lists exercises sorted by "([^"]+)"$`, func(_, sort string) error {
		return w.listExercises(generated.ListExercisesParams{Sort: ptrTo(generated.ListExercisesParamsSort(sort))})
	})
	sc.Step(`^"([^"]+)" lists exercises filtered by concepts "([^"]+)" and "([^"]+)"$`, func(_, first, second string) error {
		return w.listExercises(generated.ListExercisesParams{ConceptIds: &[]uuid.UUID{w.conceptIDFor(first), w.conceptIDFor(second)}})
	})
	sc.Step(`^"([^"]+)" lists diagrams sorted by most recently changed$`, func(string) error {
		return w.listDiagramsWith(generated.ListDiagramsParams{Sort: ptrTo(generated.ListDiagramsParamsSortUpdated)})
	})
	sc.Step(`^"([^"]+)" lists diagrams filtered by concept "([^"]+)"$`, func(_, concept string) error {
		return w.listDiagramsWith(generated.ListDiagramsParams{ConceptIds: &[]uuid.UUID{w.conceptIDFor(concept)}})
	})
	sc.Step(`^"([^"]+)" lists diagrams filtered by skill "([^"]+)" and concept "([^"]+)"$`, func(_, skill, concept string) error {
		return w.listDiagramsWith(generated.ListDiagramsParams{SkillIds: &[]uuid.UUID{w.skillIDFor(skill)}, ConceptIds: &[]uuid.UUID{w.conceptIDFor(concept)}})
	})
	sc.Step(`^"([^"]+)" lists the content node creators(?: matching "([^"]+)")?$`, func(_, query string) error {
		params := generated.ListContentNodeCreatorsParams{}
		if query != "" {
			params.Q = &query
		}
		resp, err := w.handler.ListContentNodeCreators(w.ctx(), generated.ListContentNodeCreatorsRequestObject{Params: params})
		if creators, ok := resp.(generated.ListContentNodeCreators200JSONResponse); ok {
			w.lastResp, w.lastErr = generated.ListCourseCreators200JSONResponse(creators), err
			return err
		}
		w.lastResp, w.lastErr = resp, err
		return err
	})

	// ── Editing ──────────────────────────────────────────────────────────────
	sc.Step(`^"([^"]+)" updated content node "([^"]+)" with title "([^"]+)"$`, w.updatesContentNodeTitleOnly)
	sc.Step(`^"([^"]+)" saves content node "([^"]+)" with the values it already has$`, func(name, nodeSlug string) error {
		existing, err := w.nodes.GetByID(w.ctx(), nodeID(nodeSlug).String())
		if err != nil {
			return err
		}
		return w.updatesContentNodeTitleOnly(name, nodeSlug, existing.Title)
	})
	sc.Step(`^"([^"]+)" renames diagram "([^"]+)" to "([^"]+)"$`, func(_, diagramSlug, name string) error {
		return w.updateDiagram(diagramSlug, generated.UpdateDiagramRequest{Names: &generated.LocalizedNames{"en": name}})
	})

	// ── Outcomes ─────────────────────────────────────────────────────────────
	sc.Step(`^the content node's last change is later than "([^"]+)"$`, func(day string) error {
		resp, ok := w.lastResp.(generated.UpdateContentNode200JSONResponse)
		if !ok {
			return fmt.Errorf("expected an updated content node, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return laterThan(resp.UpdatedAt, day)
	})
	sc.Step(`^the diagram's last change is later than "([^"]+)"$`, func(day string) error {
		resp, ok := w.lastResp.(generated.UpdateDiagram200JSONResponse)
		if !ok {
			return fmt.Errorf("expected an updated diagram, got %#v (err=%v)", w.lastResp, w.lastErr)
		}
		return laterThan(resp.UpdatedAt, day)
	})
	sc.Step(`^content node "([^"]+)" has (no )?unpublished changes$`, func(nodeSlug, no string) error {
		resp, err := w.handler.GetContentNode(w.ctx(), generated.GetContentNodeRequestObject{ContentNodeId: nodeID(nodeSlug)})
		if err != nil {
			return err
		}
		node, ok := resp.(generated.GetContentNode200JSONResponse)
		if !ok {
			return fmt.Errorf("expected a content node, got %#v", resp)
		}
		if want := no == ""; node.HasUnpublishedChanges != want {
			return fmt.Errorf("expected has_unpublished_changes %v, got %v", want, node.HasUnpublishedChanges)
		}
		return nil
	})
	sc.Step(`^the creators returned are "([^"]+)" with (\d+) items?(?: and "([^"]+)" with (\d+) items?)?, in that order$`, func(first string, firstCount int, second, secondCount string) error {
		want := []creatorCount{{first, firstCount}}
		if second != "" {
			n, err := strconv.Atoi(secondCount)
			if err != nil {
				return err
			}
			want = append(want, creatorCount{second, n})
		}
		return w.creatorsWithCounts(want)
	})
}

// authoredNode is what a seeded lesson varies by; the zero value is a node
// "bob" created in English, last changed when created.
type authoredNode struct {
	creator, lastChanged string
	languages            []string
}

func (w *world) putAuthoredNode(nodeSlug string, seed authoredNode) error {
	if err := w.putContentNode(nodeSlug, domain.ContentTypeVideo); err != nil {
		return err
	}
	node, err := w.nodes.GetByID(w.ctx(), nodeID(nodeSlug).String())
	if err != nil {
		return err
	}
	if seed.creator != "" {
		node.TeacherID = w.ensureRegistered(seed.creator, domain.RoleTeacher).String()
	}
	if seed.languages != nil {
		node.Languages = nil
		for _, code := range seed.languages {
			node.Languages = append(node.Languages, domain.Language{Code: code})
		}
	}
	node.UpdatedAt = node.CreatedAt
	if seed.lastChanged != "" {
		at, err := parseDay(seed.lastChanged)
		if err != nil {
			return err
		}
		node.UpdatedAt = at
	}
	node.HasUnpublishedChanges = true
	w.nodes.put(node)
	return nil
}

func (w *world) changeExercise(exSlug, day string) error {
	if err := w.putExercise(exSlug); err != nil {
		return err
	}
	ex, err := w.exercises.GetByID(w.ctx(), exerciseID(exSlug).String())
	if err != nil {
		return err
	}
	if ex.UpdatedAt, err = parseDay(day); err != nil {
		return err
	}
	w.exercises.put(ex)
	return nil
}

func (w *world) changeDiagram(diagramSlug, day string) error {
	d, err := w.diagrams.GetByID(w.ctx(), diagramID(diagramSlug).String())
	if err != nil {
		return err
	}
	if d.UpdatedAt, err = parseDay(day); err != nil {
		return err
	}
	w.diagrams.put(d)
	return nil
}

func (w *world) listContentNodes(params generated.ListContentNodesParams) error {
	resp, err := w.handler.ListContentNodes(w.ctx(), generated.ListContentNodesRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) listDiagramsWith(params generated.ListDiagramsParams) error {
	resp, err := w.handler.ListDiagrams(w.ctx(), generated.ListDiagramsRequestObject{Params: params})
	w.lastResp, w.lastErr = resp, err
	return err
}

type creatorCount struct {
	persona string
	count   int
}

func (w *world) creatorsWithCounts(want []creatorCount) error {
	resp, err := w.courseCreatorsResponse()
	if err != nil {
		return err
	}
	if len(resp) != len(want) {
		return fmt.Errorf("expected %d creators, got %d: %#v", len(want), len(resp), resp)
	}
	for i, c := range want {
		id := w.ensureRegistered(c.persona, domain.RoleTeacher)
		if resp[i].UserId != id || resp[i].ItemCount != c.count {
			return fmt.Errorf("expected creator %d to be %q with %d items, got %#v", i+1, c.persona, c.count, resp[i])
		}
	}
	return nil
}

func parseDay(day string) (time.Time, error) {
	return time.Parse(time.DateOnly, day)
}

func laterThan(got time.Time, day string) error {
	at, err := parseDay(day)
	if err != nil {
		return err
	}
	if !got.After(at) {
		return fmt.Errorf("expected the last change after %s, got %s", day, got)
	}
	return nil
}

func ptrTo[T any](v T) *T { return &v }
