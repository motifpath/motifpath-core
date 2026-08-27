//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerLearningPathSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", "([^"]+)", "([^"]+)"$`, w.putLearningPathThreeItems)
	sc.Step(`^a learning path "([^"]+)" exists in the system$`, w.putLearningPathDefault)
	sc.Step(`^a second learning path "([^"]+)" exists in the system$`, w.putLearningPathDefault)

	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)"\s+with items in order: "([^"]+)", "([^"]+)", "([^"]+)"$`, w.createsLearningPathThreeItems)
	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)"\s+with items in order: "([^"]+)", "([^"]+)"$`, w.createsLearningPathTwoItems)
	sc.Step(`^"([^"]+)" retrieves the learning path "([^"]+)"$`, w.retrievesLearningPath)
	sc.Step(`^"([^"]+)" submits a create learning path request with the title field omitted$`, w.submitsLearningPathMissingTitle)
	sc.Step(`^"([^"]+)" submits a create learning path request with an empty items array$`, w.submitsLearningPathEmptyItems)
	sc.Step(`^"([^"]+)" creates a learning path with an item referencing a content node ID that does not exist$`, w.createsLearningPathMissingNode)
	sc.Step(`^"([^"]+)" attempts to create a learning path$`, w.attemptsCreateLearningPath)
	sc.Step(`^"([^"]+)" attempts to retrieve the learning path "([^"]+)"$`, w.attemptsRetrieveLearningPath)
	sc.Step(`^an unauthenticated request attempts to create a learning path$`, w.unauthCreatesLearningPath)
	sc.Step(`^"([^"]+)" retrieves a learning path with an ID that does not exist$`, w.retrievesMissingLearningPath)

	sc.Step(`^the learning path is created and assigned a stable identifier$`, w.learningPathCreated)
	sc.Step(`^the items are returned with positions 1, 2, and 3 respectively$`, w.itemsHavePositions123)
	sc.Step(`^the path records "([^"]+)" as the owner$`, w.learningPathRecordsOwner)
	sc.Step(`^the response returns the path title, owner, and ordered items$`, w.learningPathResponseComplete)
}

func (w *world) putLearningPathThreeItems(slug, n1, n2, n3 string) error {
	for _, n := range []string{n1, n2, n3} {
		if err := w.putContentNode(n, domain.ContentTypeVideo); err != nil {
			return err
		}
	}
	w.paths.put(domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: deterministicUUID("motif-user", "seed-teacher").String(),
		Title:     slug,
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: nodeID(n1).String(), Title: n1, ContentType: domain.ContentTypeVideo},
			{Position: 2, ContentNodeID: nodeID(n2).String(), Title: n2, ContentType: domain.ContentTypeVideo},
			{Position: 3, ContentNodeID: nodeID(n3).String(), Title: n3, ContentType: domain.ContentTypeVideo},
		},
		CreatedAt: fixedNow,
	})
	return nil
}

func (w *world) putLearningPathDefault(slug string) error {
	if err := w.putContentNode("default-node-for-"+slug, domain.ContentTypeVideo); err != nil {
		return err
	}
	w.paths.put(domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: deterministicUUID("motif-user", "seed-teacher").String(),
		Title:     slug,
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: nodeID("default-node-for-" + slug).String(), Title: "default", ContentType: domain.ContentTypeVideo},
		},
		CreatedAt: fixedNow,
	})
	return nil
}

func (w *world) createsLearningPathThreeItems(name, title, n1, n2, n3 string) error {
	return w.createsLearningPath(title, []string{n1, n2, n3})
}

func (w *world) createsLearningPathTwoItems(name, title, n1, n2 string) error {
	return w.createsLearningPath(title, []string{n1, n2})
}

func (w *world) createsLearningPath(title string, nodeSlugs []string) error {
	items := make([]struct {
		ContentNodeId uuid.UUID `json:"content_node_id"`
	}, len(nodeSlugs))
	for i, slug := range nodeSlugs {
		items[i].ContentNodeId = nodeID(slug)
	}
	resp, err := w.handler.CreateLearningPath(w.ctx(), generated.CreateLearningPathRequestObject{
		Body: &generated.CreateLearningPathRequest{Title: title, Items: items},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) retrievesLearningPath(name, slug string) error {
	resp, err := w.handler.GetLearningPath(w.ctx(), generated.GetLearningPathRequestObject{LearningPathId: pathID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsLearningPathMissingTitle(string) error {
	return w.createsLearningPath("", []string{"node-01"})
}

func (w *world) submitsLearningPathEmptyItems(string) error {
	return w.createsLearningPath("Title", nil)
}

func (w *world) createsLearningPathMissingNode(string) error {
	return w.createsLearningPath("Title", []string{"does-not-exist"})
}

func (w *world) attemptsCreateLearningPath(string) error {
	return w.createsLearningPath("Title", []string{"node-01"})
}

func (w *world) attemptsRetrieveLearningPath(name, slug string) error {
	return w.retrievesLearningPath(name, slug)
}

func (w *world) unauthCreatesLearningPath() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateLearningPath("")
}

func (w *world) retrievesMissingLearningPath(string) error {
	resp, err := w.handler.GetLearningPath(w.ctx(), generated.GetLearningPathRequestObject{LearningPathId: deterministicUUID("path", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) learningPathCreated() error {
	if _, ok := w.lastResp.(generated.CreateLearningPath201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) itemsHavePositions123() error {
	resp, ok := w.lastResp.(generated.CreateLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if len(resp.Items) != 3 {
		return fmt.Errorf("expected 3 items, got %d", len(resp.Items))
	}
	for i, item := range resp.Items {
		if item.Position != i+1 {
			return fmt.Errorf("expected item %d to have position %d, got %d", i, i+1, item.Position)
		}
	}
	return nil
}

func (w *world) learningPathRecordsOwner(name string) error {
	resp, ok := w.lastResp.(generated.CreateLearningPath201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.TeacherId != w.userMotifID[name] {
		return fmt.Errorf("expected teacher_id %s for %q, got %s", w.userMotifID[name], name, resp.TeacherId)
	}
	return nil
}

func (w *world) learningPathResponseComplete() error {
	resp, ok := w.lastResp.(generated.GetLearningPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.Title == "" || resp.TeacherId.String() == "" || len(resp.Items) == 0 {
		return fmt.Errorf("expected a fully populated learning path, got %+v", resp)
	}
	return nil
}
