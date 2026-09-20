//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
)

func registerSkillSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a root skill "([^"]+)" exists in the system$`, w.aRootSkillExists)
	sc.Step(`^a skill "([^"]+)" exists under skill "([^"]+)"$`, w.aSkillExistsUnder)

	sc.Step(`^"([^"]+)" creates a skill named "([^"]+)" with no parent$`, w.createsSkillNoParent)
	sc.Step(`^"([^"]+)" creates a skill named "([^"]+)" under skill "([^"]+)"$`, w.createsSkillUnder)
	sc.Step(`^"([^"]+)" attempts to create a skill$`, w.attemptsCreateSkill)
	sc.Step(`^an unauthenticated request attempts to create a skill$`, w.unauthCreatesSkill)
	sc.Step(`^"([^"]+)" submits a create skill request with the name field omitted$`, w.submitsSkillMissingName)
	sc.Step(`^"([^"]+)" submits a create skill request with a parent_id that does not exist$`, w.submitsSkillBadParent)

	sc.Step(`^the skill is created and assigned a stable identifier$`, w.skillCreated)
	sc.Step(`^the skill has no parent$`, w.skillHasNoParent)
	sc.Step(`^the skill's parent is "([^"]+)"$`, w.skillParentIs)
	sc.Step(`^the two skills named "([^"]+)" are different entities$`, w.twoSkillsAreDifferent)

	sc.Step(`^"([^"]+)" lists all known skills$`, w.listsAllSkills)
	sc.Step(`^an unauthenticated request attempts to list all known skills$`, w.unauthListsAllSkills)
	sc.Step(`^the response includes skill "([^"]+)" with no parent$`, w.responseIncludesSkillNoParent)
	sc.Step(`^the response includes skill "([^"]+)" with parent "([^"]+)"$`, w.responseIncludesSkillWithParent)
}

func (w *world) aRootSkillExists(name string) error {
	w.putSkill(name, nil)
	return nil
}

func (w *world) aSkillExistsUnder(name, parentName string) error {
	parentID := w.skillIDFor(parentName)
	w.putSkill(name, &parentID)
	return nil
}

func (w *world) createsSkillNoParent(name, skillName string) error {
	resp, err := w.handler.CreateSkill(w.ctx(), generated.CreateSkillRequestObject{
		Body: &generated.CreateSkillRequest{Name: skillName},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) createsSkillUnder(name, skillName, parentName string) error {
	parentID := w.skillIDFor(parentName)
	resp, err := w.handler.CreateSkill(w.ctx(), generated.CreateSkillRequestObject{
		Body: &generated.CreateSkillRequest{Name: skillName, ParentId: &parentID},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) attemptsCreateSkill(string) error {
	resp, err := w.handler.CreateSkill(w.ctx(), generated.CreateSkillRequestObject{
		Body: &generated.CreateSkillRequest{Name: "attempted-skill"},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthCreatesSkill() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.attemptsCreateSkill("")
}

func (w *world) submitsSkillMissingName(string) error {
	resp, err := w.handler.CreateSkill(w.ctx(), generated.CreateSkillRequestObject{
		Body: &generated.CreateSkillRequest{},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) submitsSkillBadParent(string) error {
	missing := deterministicUUID("skill", "does-not-exist")
	resp, err := w.handler.CreateSkill(w.ctx(), generated.CreateSkillRequestObject{
		Body: &generated.CreateSkillRequest{Name: "orphan", ParentId: &missing},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) skillCreated() error {
	if _, ok := w.lastResp.(generated.CreateSkill201JSONResponse); !ok {
		return fmt.Errorf("expected a 201 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}

func (w *world) skillHasNoParent() error {
	resp, ok := w.lastResp.(generated.CreateSkill201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	if resp.ParentId != nil {
		return fmt.Errorf("expected no parent, got %s", *resp.ParentId)
	}
	return nil
}

func (w *world) skillParentIs(parentName string) error {
	resp, ok := w.lastResp.(generated.CreateSkill201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	want := w.skillIDFor(parentName)
	if resp.ParentId == nil || *resp.ParentId != want {
		return fmt.Errorf("expected parent_id %s, got %#v", want, resp.ParentId)
	}
	return nil
}

func (w *world) twoSkillsAreDifferent(name string) error {
	resp, ok := w.lastResp.(generated.CreateSkill201JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 201 response, got %#v", w.lastResp)
	}
	existing, ok := w.skillIDByName[name]
	if !ok {
		return fmt.Errorf("no previously seeded skill named %q to compare against", name)
	}
	if resp.SkillId == existing {
		return fmt.Errorf("expected two different skill entities, both had id %s", existing)
	}
	return nil
}

func (w *world) listsAllSkills(string) error {
	resp, err := w.handler.ListSkills(w.ctx(), generated.ListSkillsRequestObject{})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthListsAllSkills() error {
	w.noAuthToken() //nolint:errcheck // never errors
	return w.listsAllSkills("")
}

func (w *world) responseIncludesSkillNoParent(name string) error {
	resp, ok := w.lastResp.(generated.ListSkills200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	for _, s := range resp {
		if s.Name == name && s.ParentId == nil {
			return nil
		}
	}
	return fmt.Errorf("expected skills to include %q with no parent, got %+v", name, resp)
}

func (w *world) responseIncludesSkillWithParent(name, parentName string) error {
	resp, ok := w.lastResp.(generated.ListSkills200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	wantParent := w.skillIDFor(parentName)
	for _, s := range resp {
		if s.Name == name && s.ParentId != nil && *s.ParentId == wantParent {
			return nil
		}
	}
	return fmt.Errorf("expected skills to include %q with parent %s, got %+v", name, wantParent, resp)
}
