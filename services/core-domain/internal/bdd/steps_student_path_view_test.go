//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerStudentPathViewSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" has "([^"]+)" assigned with no progress recorded$`, w.alreadyHasAssigned)
	sc.Step(`^"([^"]+)" has "([^"]+)" assigned$`, w.alreadyHasAssigned)
	sc.Step(`^"([^"]+)" has completed "([^"]+)"$`, w.hasCompletedNode)
	sc.Step(`^"([^"]+)" has completed "([^"]+)", "([^"]+)", and "([^"]+)"$`, w.hasCompletedThreeNodes)
	sc.Step(`^"([^"]+)" has started but not completed "([^"]+)"$`, w.hasStartedNode)
	sc.Step(`^"([^"]+)" has no active path assignment$`, func(string) error { return nil })

	sc.Step(`^"([^"]+)" retrieves her current path$`, w.retrievesCurrentPath)
	sc.Step(`^"([^"]+)" requests GET /students/me/path$`, w.retrievesCurrentPath)
	sc.Step(`^an unauthenticated request attempts to retrieve the student path view$`, w.unauthGetMyPath)

	sc.Step(`^the response contains all three items in order$`, w.responseContainsThreeItemsInOrder)
	sc.Step(`^"([^"]+)" has status "([^"]+)"$`, w.nodeHasStatus)
	sc.Step(`^all three items have status "([^"]+)"$`, w.allItemsHaveStatus)
	sc.Step(`^the current_position is (\d+)$`, w.currentPositionIs)
	sc.Step(`^each item in the response includes a title and content_type$`, w.eachItemHasTitleAndContentType)
	sc.Step(`^none of the items have a section_label$`, w.noItemsHaveSectionLabel)
	sc.Step(`^"([^"]+)" and "([^"]+)" have section_label "([^"]+)"$`, w.itemsHaveSectionLabel)
	sc.Step(`^"([^"]+)" has section_label "([^"]+)"$`, w.itemHasSectionLabel)
}

func (w *world) hasCompletedNode(name, nodeSlug string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent)
	w.completion.set(studentID.String(), nodeID(nodeSlug).String(), domain.CompletionStatusCompleted)
	return nil
}

func (w *world) hasCompletedThreeNodes(name, n1, n2, n3 string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent)
	for _, n := range []string{n1, n2, n3} {
		w.completion.set(studentID.String(), nodeID(n).String(), domain.CompletionStatusCompleted)
	}
	return nil
}

func (w *world) hasStartedNode(name, nodeSlug string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent)
	w.completion.set(studentID.String(), nodeID(nodeSlug).String(), domain.CompletionStatusInProgress)
	return nil
}

func (w *world) retrievesCurrentPath(name string) error {
	resp, err := w.handler.GetMyPath(w.ctx(), generated.GetMyPathRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthGetMyPath() error {
	w.noAuthToken() //nolint:errcheck // never errors
	resp, err := w.handler.GetMyPath(w.ctx(), generated.GetMyPathRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) studentPathView() (generated.GetMyPath200JSONResponse, error) {
	resp, ok := w.lastResp.(generated.GetMyPath200JSONResponse)
	if !ok {
		return generated.GetMyPath200JSONResponse{}, fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return resp, nil
}

func (w *world) responseContainsThreeItemsInOrder() error {
	resp, err := w.studentPathView()
	if err != nil {
		return err
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

func (w *world) nodeHasStatus(nodeSlug, status string) error {
	resp, err := w.studentPathView()
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		if item.ContentNodeId == nodeID(nodeSlug) {
			if string(item.Status) != status {
				return fmt.Errorf("expected %q to have status %q, got %q", nodeSlug, status, item.Status)
			}
			return nil
		}
	}
	return fmt.Errorf("no item found for content node %q", nodeSlug)
}

func (w *world) allItemsHaveStatus(status string) error {
	resp, err := w.studentPathView()
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		if string(item.Status) != status {
			return fmt.Errorf("expected all items to have status %q, but item at position %d had %q", status, item.Position, item.Status)
		}
	}
	return nil
}

func (w *world) currentPositionIs(position int) error {
	resp, err := w.studentPathView()
	if err != nil {
		return err
	}
	if resp.CurrentPosition != position {
		return fmt.Errorf("expected current_position %d, got %d", position, resp.CurrentPosition)
	}
	return nil
}

func (w *world) sectionLabelFor(nodeSlug string) (string, bool, error) {
	resp, err := w.studentPathView()
	if err != nil {
		return "", false, err
	}
	for _, item := range resp.Items {
		if item.ContentNodeId == nodeID(nodeSlug) {
			if item.SectionLabel == nil {
				return "", false, nil
			}
			return *item.SectionLabel, true, nil
		}
	}
	return "", false, fmt.Errorf("no item found for content node %q", nodeSlug)
}

func (w *world) noItemsHaveSectionLabel() error {
	resp, err := w.studentPathView()
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		if item.SectionLabel != nil {
			return fmt.Errorf("expected no section_label on item at position %d, got %q", item.Position, *item.SectionLabel)
		}
	}
	return nil
}

func (w *world) itemsHaveSectionLabel(slugA, slugB, label string) error {
	for _, slug := range []string{slugA, slugB} {
		if err := w.itemHasSectionLabel(slug, label); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) itemHasSectionLabel(nodeSlug, label string) error {
	got, present, err := w.sectionLabelFor(nodeSlug)
	if err != nil {
		return err
	}
	if !present || got != label {
		return fmt.Errorf("expected %q to have section_label %q, got %q (present=%t)", nodeSlug, label, got, present)
	}
	return nil
}

func (w *world) eachItemHasTitleAndContentType() error {
	resp, err := w.studentPathView()
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		if item.Title == "" || item.ContentType == "" {
			return fmt.Errorf("expected item at position %d to have title and content_type, got %+v", item.Position, item)
		}
	}
	return nil
}
