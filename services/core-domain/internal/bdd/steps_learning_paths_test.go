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
	sc.Step(`^a learning path "([^"]+)" exists with "([^"]+)" and "([^"]+)" in section "([^"]+)" and "([^"]+)" in section "([^"]+)"$`, w.putLearningPathThreeItemsWithSections)
	sc.Step(`^a learning path "([^"]+)" exists in the system$`, w.putLearningPathDefault)
	sc.Step(`^a second learning path "([^"]+)" exists in the system$`, w.putLearningPathDefault)

	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)"\s+with items in order: "([^"]+)", "([^"]+)", "([^"]+)"$`, w.createsLearningPathThreeItems)
	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)"\s+with items in order: "([^"]+)", "([^"]+)"$`, w.createsLearningPathTwoItems)
	sc.Step(`^"([^"]+)" creates a learning path titled "([^"]+)"\s+with items in order: "([^"]+)" in section "([^"]+)", "([^"]+)" in section "([^"]+)", "([^"]+)" in section "([^"]+)"$`, w.createsLearningPathThreeItemsWithSections)
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
	sc.Step(`^"([^"]+)" and "([^"]+)" are returned with section_label "([^"]+)"$`, w.createdItemsHaveSectionLabel)
	sc.Step(`^"([^"]+)" is returned with section_label "([^"]+)"$`, w.createdItemHasSectionLabel)
}

type pathItemSpec struct {
	slug         string
	sectionLabel string
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

func (w *world) putLearningPathThreeItemsWithSections(slug, n1, n2, sectionA, n3, sectionB string) error {
	labels := map[string]string{n1: sectionA, n2: sectionA, n3: sectionB}
	orderedSlugs := []string{n1, n2, n3}
	items := make([]domain.LearningPathItem, len(orderedSlugs))
	for i, n := range orderedSlugs {
		if err := w.putContentNode(n, domain.ContentTypeVideo); err != nil {
			return err
		}
		items[i] = domain.LearningPathItem{
			Position:      i + 1,
			ContentNodeID: nodeID(n).String(),
			Title:         n,
			ContentType:   domain.ContentTypeVideo,
			SectionLabel:  labels[n],
		}
	}
	w.paths.put(domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: deterministicUUID("motif-user", "seed-teacher").String(),
		Title:     slug,
		Items:     items,
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

func (w *world) createsLearningPathThreeItemsWithSections(name, title, n1, s1, n2, s2, n3, s3 string) error {
	return w.createsLearningPathWithSpecs(title, []pathItemSpec{
		{slug: n1, sectionLabel: s1},
		{slug: n2, sectionLabel: s2},
		{slug: n3, sectionLabel: s3},
	})
}

func (w *world) createsLearningPath(title string, nodeSlugs []string) error {
	specs := make([]pathItemSpec, len(nodeSlugs))
	for i, slug := range nodeSlugs {
		specs[i] = pathItemSpec{slug: slug}
	}
	return w.createsLearningPathWithSpecs(title, specs)
}

func (w *world) createsLearningPathWithSpecs(title string, specs []pathItemSpec) error {
	body := &generated.CreateLearningPathRequest{Title: title}
	for _, spec := range specs {
		item := struct {
			ContentNodeId uuid.UUID `json:"content_node_id"`
			SectionLabel  *string   `json:"section_label,omitempty"`
		}{ContentNodeId: nodeID(spec.slug)}
		if spec.sectionLabel != "" {
			label := spec.sectionLabel
			item.SectionLabel = &label
		}
		body.Items = append(body.Items, item)
	}
	resp, err := w.handler.CreateLearningPath(w.ctx(), generated.CreateLearningPathRequestObject{Body: body})
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

func (w *world) createdSectionLabelFor(slug string) (string, error) {
	resp, ok := w.lastResp.(generated.CreateLearningPath201JSONResponse)
	if !ok {
		return "", fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	want := nodeID(slug)
	for _, item := range resp.Items {
		if item.ContentNodeId == want {
			if item.SectionLabel == nil {
				return "", nil
			}
			return *item.SectionLabel, nil
		}
	}
	return "", fmt.Errorf("no returned item references content node %q", slug)
}

func (w *world) createdItemsHaveSectionLabel(slugA, slugB, label string) error {
	for _, slug := range []string{slugA, slugB} {
		if err := w.createdItemHasSectionLabel(slug, label); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) createdItemHasSectionLabel(slug, label string) error {
	got, err := w.createdSectionLabelFor(slug)
	if err != nil {
		return err
	}
	if got != label {
		return fmt.Errorf("expected %q to have section_label %q, got %q", slug, label, got)
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
