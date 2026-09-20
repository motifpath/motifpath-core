//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

func registerConceptSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a root concept "([^"]+)" exists in the system$`, w.aRootConceptExists)
	sc.Step(`^a concept "([^"]+)" exists under concept "([^"]+)"$`, w.aConceptExistsUnder)

	sc.Step(`^"([^"]+)" creates a concept named "([^"]+)" with no parent$`, w.createsConceptNoParent)
	sc.Step(`^"([^"]+)" creates a concept named "([^"]+)" under concept "([^"]+)"$`, w.createsConceptUnder)
	sc.Step(`^"([^"]+)" attempts to create a concept$`, w.attemptsCreateConcept)
	sc.Step(`^an unauthenticated request attempts to create a concept$`, w.unauthCreatesConcept)
	sc.Step(`^"([^"]+)" submits a create concept request with the name field omitted$`, w.submitsConceptMissingName)
	sc.Step(`^"([^"]+)" submits a create concept request with a parent_id that does not exist$`, w.submitsConceptBadParent)

	sc.Step(`^the concept is created and assigned a stable identifier$`, w.conceptCreated)
	sc.Step(`^the concept has no parent$`, w.conceptHasNoParent)
	sc.Step(`^the concept's parent is "([^"]+)"$`, w.conceptParentIs)
	sc.Step(`^the two concepts named "([^"]+)" are different entities$`, w.twoConceptsAreDifferent)

	sc.Step(`^"([^"]+)" lists all known concepts$`, w.listsAllConcepts)
	sc.Step(`^an unauthenticated request attempts to list all known concepts$`, w.unauthListsAllConcepts)
	sc.Step(`^the response includes concept "([^"]+)" with no parent$`, w.responseIncludesConceptNoParent)
	sc.Step(`^the response includes concept "([^"]+)" with parent "([^"]+)"$`, w.responseIncludesConceptWithParent)
}

func (w *world) aRootConceptExists(name string) error {
	w.putConcept(name, nil)
	return nil
}

func (w *world) aConceptExistsUnder(name, parentName string) error {
	parentID := w.conceptIDFor(parentName)
	w.putConcept(name, &parentID)
	return nil
}

func (w *world) createsConceptNoParent(name, conceptName string) error {
	resp, err := w.handler.CreateConcept(w.ctx(), generated.CreateConceptRequestObject{
		Body: &generated.CreateConceptRequest{Name: conceptName},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsConceptUnder(name, conceptName, parentName string) error {
	parentID := w.conceptIDFor(parentName)
	resp, err := w.handler.CreateConcept(w.ctx(), generated.CreateConceptRequestObject{
		Body: &generated.CreateConceptRequest{Name: conceptName, ParentId: &parentID},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsCreateConcept(string) error {
	resp, err := w.handler.CreateConcept(w.ctx(), generated.CreateConceptRequestObject{
		Body: &generated.CreateConceptRequest{Name: "attempted-concept"},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthCreatesConcept() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateConcept("")
}

func (w *world) submitsConceptMissingName(string) error {
	resp, err := w.handler.CreateConcept(w.ctx(), generated.CreateConceptRequestObject{
		Body: &generated.CreateConceptRequest{},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsConceptBadParent(string) error {
	missing := deterministicUUID("concept", "does-not-exist")
	resp, err := w.handler.CreateConcept(w.ctx(), generated.CreateConceptRequestObject{
		Body: &generated.CreateConceptRequest{Name: "orphan", ParentId: &missing},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) conceptCreated() error {
	if _, ok := w.lastResp.(generated.CreateConcept201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) conceptHasNoParent() error {
	resp, ok := w.lastResp.(generated.CreateConcept201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.ParentId != nil {
		return fmt.Errorf("expected no parent, got %s", *resp.ParentId)
	}
	return nil
}

func (w *world) conceptParentIs(parentName string) error {
	resp, ok := w.lastResp.(generated.CreateConcept201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	want := w.conceptIDFor(parentName)
	if resp.ParentId == nil || *resp.ParentId != want {
		return fmt.Errorf("expected parent_id %s, got %#v", want, resp.ParentId)
	}
	return nil
}

func (w *world) twoConceptsAreDifferent(name string) error {
	resp, ok := w.lastResp.(generated.CreateConcept201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	existing, ok := w.conceptIDByName[name]
	if !ok {
		return fmt.Errorf("no previously seeded concept named %q to compare against", name)
	}
	if resp.ConceptId == existing {
		return fmt.Errorf("expected two different concept entities, both had id %s", existing)
	}
	return nil
}

func (w *world) listsAllConcepts(string) error {
	resp, err := w.handler.ListConcepts(w.ctx(), generated.ListConceptsRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthListsAllConcepts() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsAllConcepts("")
}

func (w *world) responseIncludesConceptNoParent(name string) error {
	resp, ok := w.lastResp.(generated.ListConcepts200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, c := range resp {
		if c.Name == name && c.ParentId == nil {
			return nil
		}
	}
	return fmt.Errorf("expected concepts to include %q with no parent, got %+v", name, resp)
}

func (w *world) responseIncludesConceptWithParent(name, parentName string) error {
	resp, ok := w.lastResp.(generated.ListConcepts200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	wantParent := w.conceptIDFor(parentName)
	for _, c := range resp {
		if c.Name == name && c.ParentId != nil && *c.ParentId == wantParent {
			return nil
		}
	}
	return fmt.Errorf("expected concepts to include %q with parent %s, got %+v", name, wantParent, resp)
}
