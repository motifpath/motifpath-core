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

func registerLearningPathSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", "([^"]+)", "([^"]+)"$`, w.putLearningPathThreeItems)
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", "([^"]+)"$`, w.putLearningPathTwoItems)
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)"$`, w.putLearningPathOneItem)
	sc.Step(`^a second learning path "([^"]+)" exists with items "([^"]+)"$`, w.putLearningPathOneItem)
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

	sc.Step(`^"([^"]+)" lists all learning paths$`, w.listsAllLearningPaths)
	sc.Step(`^"([^"]+)" attempts to list all learning paths$`, w.listsAllLearningPaths)
	sc.Step(`^an unauthenticated request attempts to list all learning paths$`, w.unauthListsAllLearningPaths)

	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" with items in order: "([^"]+)", "([^"]+)"$`, w.replacesLearningPathTwoItems)
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" with items in order: "([^"]+)", "([^"]+)", "([^"]+)"$`, w.replacesLearningPathThreeItems)
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" with items in order: "([^"]+)" in section "([^"]+)", "([^"]+)" in section "([^"]+)", "([^"]+)"$`, w.replacesLearningPathThreeItemsWithSections)
	sc.Step(`^"([^"]+)" attempts to replace learning path "([^"]+)" with items in order: "([^"]+)"$`, w.attemptsReplaceLearningPath)
	sc.Step(`^an unauthenticated request attempts to replace learning path "([^"]+)" with items in order: "([^"]+)"$`, w.unauthReplacesLearningPath)
	sc.Step(`^"([^"]+)" submits a replace learning path request for "([^"]+)" with an empty items array$`, w.submitsReplaceLearningPathEmptyItems)
	sc.Step(`^"([^"]+)" replaces learning path "([^"]+)" with an item referencing a content node ID that does not exist$`, w.replacesLearningPathMissingNode)
	sc.Step(`^"([^"]+)" attempts to replace a learning path with an ID that does not exist$`, w.attemptsReplaceMissingLearningPath)

	sc.Step(`^the items are returned with positions (\d+), (\d+), and (\d+) in the order "([^"]+)", "([^"]+)", "([^"]+)"$`, w.replaceItemsReturnedInOrder)
	sc.Step(`^the learning path has (\d+) items$`, w.learningPathHasNItems)

	// ── Deleting a learning path ────────────────────────────────────────────
	sc.Step(`^a learning path "([^"]+)" exists with items "([^"]+)", "([^"]+)", "([^"]+)", owned by a different teacher$`, w.putLearningPathThreeItemsOtherTeacher)
	sc.Step(`^student "([^"]+)" has "([^"]+)" assigned as a standalone path$`, w.alreadyHasAssigned)
	sc.Step(`^a course "([^"]+)" exists, published, with checkpoints "([^"]+)"$`, w.courseExistsPublishedWithCheckpoint)
	sc.Step(`^a course "([^"]+)" exists as a draft with checkpoints "([^"]+)"$`, w.courseExistsDraftWithCheckpoint)
	sc.Step(`^"([^"]+)" has since been retired$`, w.courseHasBeenRetired)

	sc.Step(`^"([^"]+)" deletes the learning path "([^"]+)"$`, w.deletesLearningPath)
	sc.Step(`^"([^"]+)" attempts to delete the learning path "([^"]+)"$`, w.deletesLearningPath)
	sc.Step(`^"([^"]+)" attempts to delete a learning path with an ID that does not exist$`, w.attemptsDeleteMissingLearningPath)
	sc.Step(`^an unauthenticated request attempts to delete learning path "([^"]+)"$`, w.unauthDeletesLearningPath)

	sc.Step(`^the learning path is deleted$`, w.learningPathIsDeleted)
	sc.Step(`^"([^"]+)"'s copy of the path is unaffected$`, w.studentsCopyOfPathUnaffected)
}

func (w *world) listsAllLearningPaths(name string) error {
	resp, err := w.handler.ListLearningPaths(w.ctx(), generated.ListLearningPathsRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthListsAllLearningPaths() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsAllLearningPaths("")
}

func (w *world) replacesLearningPathWithSpecs(slug string, specs []pathItemSpec) error {
	body := &generated.ReplaceLearningPathRequest{Title: slug}
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
	resp, err := w.handler.ReplaceLearningPath(w.ctx(), generated.ReplaceLearningPathRequestObject{
		LearningPathId: pathID(slug),
		Body:           body,
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) replacesLearningPath(slug string, nodeSlugs []string) error {
	specs := make([]pathItemSpec, len(nodeSlugs))
	for i, s := range nodeSlugs {
		specs[i] = pathItemSpec{slug: s}
	}
	return w.replacesLearningPathWithSpecs(slug, specs)
}

func (w *world) replacesLearningPathTwoItems(name, slug, n1, n2 string) error {
	return w.replacesLearningPath(slug, []string{n1, n2})
}

func (w *world) replacesLearningPathThreeItems(name, slug, n1, n2, n3 string) error {
	return w.replacesLearningPath(slug, []string{n1, n2, n3})
}

func (w *world) replacesLearningPathThreeItemsWithSections(name, slug, n1, s1, n2, s2, n3 string) error {
	return w.replacesLearningPathWithSpecs(slug, []pathItemSpec{
		{slug: n1, sectionLabel: s1},
		{slug: n2, sectionLabel: s2},
		{slug: n3},
	})
}

func (w *world) attemptsReplaceLearningPath(name, slug, n1 string) error {
	return w.replacesLearningPath(slug, []string{n1})
}

func (w *world) unauthReplacesLearningPath(slug, n1 string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsReplaceLearningPath("", slug, n1)
}

func (w *world) submitsReplaceLearningPathEmptyItems(name, slug string) error {
	return w.replacesLearningPathWithSpecs(slug, nil)
}

func (w *world) replacesLearningPathMissingNode(name, slug string) error {
	return w.replacesLearningPath(slug, []string{"does-not-exist"})
}

func (w *world) attemptsReplaceMissingLearningPath(string) error {
	body := &generated.ReplaceLearningPathRequest{Title: "Title"}
	body.Items = append(body.Items, struct {
		ContentNodeId uuid.UUID `json:"content_node_id"`
		SectionLabel  *string   `json:"section_label,omitempty"`
	}{ContentNodeId: nodeID("node-01")})
	resp, err := w.handler.ReplaceLearningPath(w.ctx(), generated.ReplaceLearningPathRequestObject{
		LearningPathId: deterministicUUID("path", "does-not-exist"),
		Body:           body,
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) replaceItemsReturnedInOrder(p1, p2, p3, n1, n2, n3 string) error {
	resp, ok := w.lastResp.(generated.ReplaceLearningPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	wantPositions := []string{p1, p2, p3}
	wantNodes := []string{n1, n2, n3}
	if len(resp.Items) != 3 {
		return fmt.Errorf("expected 3 items, got %d", len(resp.Items))
	}
	for i, item := range resp.Items {
		wantPos, err := parseInt(wantPositions[i])
		if err != nil {
			return err
		}
		if item.Position != wantPos {
			return fmt.Errorf("expected item %d to have position %d, got %d", i, wantPos, item.Position)
		}
		if item.ContentNodeId != nodeID(wantNodes[i]) {
			return fmt.Errorf("expected item %d to reference %s, got %s", i, nodeID(wantNodes[i]), item.ContentNodeId)
		}
	}
	return nil
}

func (w *world) learningPathHasNItems(countStr string) error {
	count, err := parseInt(countStr)
	if err != nil {
		return err
	}
	resp, ok := w.lastResp.(generated.ReplaceLearningPath200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if len(resp.Items) != count {
		return fmt.Errorf("expected %d items, got %d", count, len(resp.Items))
	}
	return nil
}

type pathItemSpec struct {
	slug         string
	sectionLabel string
}

// autoPublishNode seeds a version 1 ContentNodeVersion for the content node
// slug, unless one already exists. A learning path "exists in the system"
// ready to be assigned implies its items' content nodes are already
// published — assigning refuses any path whose content node has never
// been published (content-node-versioning.feature), so every
// putLearningPath* helper here publishes each item's node by default.
// Scenarios that specifically need an unpublished node instead seed it
// directly via putContentNode (never through these helpers).
func (w *world) autoPublishNode(slug string) {
	ctx := context.Background()
	node, err := w.nodes.GetByID(ctx, nodeID(slug).String())
	if err != nil {
		return
	}
	if _, err := w.versions.GetLatestByContentNodeID(ctx, node.ID); err == nil {
		return
	}
	_ = w.versions.Create(ctx, domain.ContentNodeVersion{
		ID:            deterministicUUID("content-node-version", slug, "1").String(),
		ContentNodeID: node.ID,
		VersionNumber: 1,
		Title:         node.Title,
		ContentType:   node.ContentType,
		MediaURL:      node.MediaURL,
		RichContent:   node.RichContent,
		PublishedBy:   node.TeacherID,
		PublishedAt:   fixedNow,
	})
}

func (w *world) putLearningPathThreeItems(slug, n1, n2, n3 string) error {
	for _, n := range []string{n1, n2, n3} {
		if err := w.putContentNode(n, domain.ContentTypeVideo); err != nil {
			return err
		}
		w.autoPublishNode(n)
	}
	w.paths.put(domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: w.ensureRegistered("bob", domain.RoleTeacher).String(),
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

// putLearningPathOneItem deliberately does NOT auto-publish its item's
// content node (unlike the multi-item variants below) — every scenario
// that uses the single-item form is in content-node-versioning.feature,
// which manages publishing explicitly itself as the thing under test.
func (w *world) putLearningPathOneItem(slug, n1 string) error {
	if err := w.putContentNode(n1, domain.ContentTypeVideo); err != nil {
		return err
	}
	w.paths.put(domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: w.ensureRegistered("bob", domain.RoleTeacher).String(),
		Title:     slug,
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: nodeID(n1).String(), Title: n1, ContentType: domain.ContentTypeVideo},
		},
		CreatedAt: fixedNow,
	})
	return nil
}

func (w *world) putLearningPathTwoItems(slug, n1, n2 string) error {
	for _, n := range []string{n1, n2} {
		if err := w.putContentNode(n, domain.ContentTypeVideo); err != nil {
			return err
		}
		w.autoPublishNode(n)
	}
	w.paths.put(domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: w.ensureRegistered("bob", domain.RoleTeacher).String(),
		Title:     slug,
		Items: []domain.LearningPathItem{
			{Position: 1, ContentNodeID: nodeID(n1).String(), Title: n1, ContentType: domain.ContentTypeVideo},
			{Position: 2, ContentNodeID: nodeID(n2).String(), Title: n2, ContentType: domain.ContentTypeVideo},
		},
		CreatedAt: fixedNow,
	})
	return nil
}

func (w *world) putLearningPathThreeItemsWithSections(slug, n1, n2, sectionA, n3, sectionB string) error {
	labels := map[string]*string{n1: &sectionA, n2: &sectionA, n3: &sectionB}
	orderedSlugs := []string{n1, n2, n3}
	items := make([]domain.LearningPathItem, len(orderedSlugs))
	for i, n := range orderedSlugs {
		if err := w.putContentNode(n, domain.ContentTypeVideo); err != nil {
			return err
		}
		w.autoPublishNode(n)
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
		TeacherID: w.ensureRegistered("bob", domain.RoleTeacher).String(),
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
	w.autoPublishNode("default-node-for-" + slug)
	w.paths.put(domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: w.ensureRegistered("bob", domain.RoleTeacher).String(),
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
	var items []generated.LearningPathItem
	switch resp := w.lastResp.(type) {
	case generated.CreateLearningPath201JSONResponse:
		items = resp.Items
	case generated.ReplaceLearningPath200JSONResponse:
		items = resp.Items
	default:
		return "", fmt.Errorf("expected a 201 or 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	want := nodeID(slug)
	for _, item := range items {
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

// putLearningPathThreeItemsOtherTeacher is putLearningPathThreeItems's
// counterpart for the "owned by a different teacher" scenarios — every
// other putLearningPath* helper here hardcodes "bob" as the owner, which is
// exactly the teacher every ownership-rejection scenario authenticates as,
// so this needs a distinct owner instead.
func (w *world) putLearningPathThreeItemsOtherTeacher(slug, n1, n2, n3 string) error {
	for _, n := range []string{n1, n2, n3} {
		if err := w.putContentNode(n, domain.ContentTypeVideo); err != nil {
			return err
		}
		w.autoPublishNode(n)
	}
	w.paths.put(domain.LearningPath{
		ID:        pathID(slug).String(),
		TeacherID: w.ensureRegistered("a-different-teacher-for-"+slug, domain.RoleTeacher).String(),
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

// courseExistsPublishedWithCheckpoint seeds a Course with a single
// checkpoint referencing pathSlug's learning path and publishes it — the
// "referenced by a published CourseVersion" precondition the deletion
// guard scenarios need. Both the create and the publish run under an
// isolated seed identity (mirroring alreadyHasAssigned), so this setup
// never disturbs whichever identity the scenario itself is authenticated
// as. Publishing requires an admin caller, distinct from the teacher who
// creates the draft.
func (w *world) courseExistsPublishedWithCheckpoint(courseSlug, pathSlug string) error {
	return w.seedCourseWithCheckpoint(courseSlug, pathSlug, true)
}

// courseExistsDraftWithCheckpoint is courseExistsPublishedWithCheckpoint's
// counterpart for "referenced only by an unpublished course draft" — the
// checkpoint is recorded on the live Course draft but never snapshotted
// into a CourseVersion, so the deletion guard (which only ever consults
// CourseVersionCheckpoint) finds nothing.
func (w *world) courseExistsDraftWithCheckpoint(courseSlug, pathSlug string) error {
	return w.seedCourseWithCheckpoint(courseSlug, pathSlug, false)
}

func (w *world) seedCourseWithCheckpoint(courseSlug, pathSlug string, publish bool) error {
	if _, err := w.paths.GetByID(context.Background(), pathID(pathSlug).String()); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		if err := w.putLearningPathDefault(pathSlug); err != nil {
			return err
		}
	}

	teacherName := "seed-teacher-for-" + courseSlug
	w.ensureRegistered(teacherName, domain.RoleTeacher)
	teacherCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub(teacherName))

	resp, err := w.handler.CreateCourse(teacherCtx, generated.CreateCourseRequestObject{
		Body: &generated.CreateCourseRequest{
			Title:   courseSlug,
			Summary: "Seeded for testing",
			Level:   generated.CreateCourseRequestLevelBeginner,
			Checkpoints: []struct {
				LearningPathId uuid.UUID `json:"learning_path_id"`
				Title          *string   `json:"title,omitempty"`
			}{
				{LearningPathId: pathID(pathSlug)},
			},
		},
	})
	if err != nil {
		return err
	}
	created, ok := resp.(generated.CreateCourse201JSONResponse)
	if !ok {
		return fmt.Errorf("setup: expected course creation to succeed, got %#v", resp)
	}
	w.courseIDBySlug[courseSlug] = created.CourseId

	if !publish {
		return nil
	}

	adminName := "seed-admin-for-" + courseSlug
	w.ensureRegistered(adminName, domain.RoleAdmin)
	adminCtx := appHTTP.WithClerkUserID(context.Background(), clerkSub(adminName))

	publishResp, err := w.handler.PublishCourse(adminCtx, generated.PublishCourseRequestObject{CourseId: created.CourseId})
	if err != nil {
		return err
	}
	if _, ok := publishResp.(generated.PublishCourse201JSONResponse); !ok {
		return fmt.Errorf("setup: expected course publish to succeed, got %#v", publishResp)
	}
	return nil
}

// courseHasBeenRetired sets courseSlug's status directly on the fake
// CourseRepository rather than through the HTTP handler — this step only
// needs the Course's status to read "retired" for the deletion guard
// scenario to exercise its "even a since-retired course" clause; it never
// asserts on RetireCourse's own request/response behavior, which
// steps_courses_test.go's attemptsRetireCourse exercises for real.
func (w *world) courseHasBeenRetired(courseSlug string) error {
	courseID, ok := w.courseIDBySlug[courseSlug]
	if !ok {
		return fmt.Errorf("no course was seeded for slug %q", courseSlug)
	}
	return w.courses.UpdateStatus(context.Background(), courseID.String(), domain.CourseStatusRetired)
}

func (w *world) deletesLearningPath(name, slug string) error {
	w.lastDeletedPathSlug = slug
	resp, err := w.handler.DeleteLearningPath(w.ctx(), generated.DeleteLearningPathRequestObject{LearningPathId: pathID(slug)})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsDeleteMissingLearningPath(string) error {
	resp, err := w.handler.DeleteLearningPath(w.ctx(), generated.DeleteLearningPathRequestObject{LearningPathId: deterministicUUID("path", "does-not-exist")})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthDeletesLearningPath(slug string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.deletesLearningPath("", slug)
}

func (w *world) learningPathIsDeleted() error {
	if _, ok := w.lastResp.(generated.DeleteLearningPath204Response); !ok {
		return fmt.Errorf("expected a 204 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if w.lastDeletedPathSlug == "" {
		return fmt.Errorf("no learning path slug was recorded to check")
	}
	if _, err := w.paths.GetByID(context.Background(), pathID(w.lastDeletedPathSlug).String()); !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("expected learning path %q to no longer exist, got err=%v", w.lastDeletedPathSlug, err)
	}
	return nil
}

// studentsCopyOfPathUnaffected asserts studentName's own StudentPath copy —
// resolved via their current-path pointer, the same way
// studentPathBecomesCurrentPath does — still exists after the template it
// was copied from is deleted. Each StudentPath is an independent snapshot
// (see domain.StudentPath), so the template's deletion must never cascade
// to it.
func (w *world) studentsCopyOfPathUnaffected(studentName string) error {
	studentMotifID, ok := w.userMotifID[studentName]
	if !ok {
		return fmt.Errorf("no user %q has been registered", studentName)
	}
	state, err := w.learningState.GetByStudentID(context.Background(), studentMotifID.String())
	if err != nil {
		return err
	}
	if state.CurrentStandalonePathID == nil {
		return fmt.Errorf("%q has no current standalone path", studentName)
	}
	sp, err := w.studentPaths.GetByID(context.Background(), *state.CurrentStandalonePathID)
	if err != nil {
		return fmt.Errorf("expected %q's copy of the path to still exist: %w", studentName, err)
	}
	if sp.ArchivedAt != nil {
		return fmt.Errorf("expected %q's copy of the path to remain unarchived", studentName)
	}
	return nil
}
