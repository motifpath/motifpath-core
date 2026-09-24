//go:build integration

package bdd

import (
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerLocaleSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^"([^"]+)" has no locale preference set$`, func(name string) error { w.ensureRegistered(name, domain.RoleStudent); return nil })
	sc.Step(`^"([^"]+)" has locale "([^"]+)"$`, w.hasLocale)
	sc.Step(`^"([^"]+)" sets their locale to "([^"]+)"$`, w.updatesLocale)
	sc.Step(`^"([^"]+)" submits a locale update with locale "([^"]+)"$`, w.updatesLocale)
	sc.Step(`^"([^"]+)" attempts to set their locale to "([^"]+)"$`, w.updatesLocale)
	sc.Step(`^an unauthenticated request attempts to set the locale to "([^"]+)"$`, w.unauthUpdatesLocale)

	sc.Step(`^the response returns "([^"]+)"'s user_id, role "([^"]+)", and locale "([^"]+)"$`, w.localeUpdateResponseMatches)

	sc.Step(`^"([^"]+)" has content available in locale "([^"]+)"$`, w.nodeHasContentInLocale)
	sc.Step(`^"([^"]+)" has content available only in locale "([^"]+)"$`, w.nodeHasContentInLocale)
	sc.Step(`^"([^"]+)" has content available in any locale$`, w.nodeHasContentInAnyLocale)
}

// hasLocale sets name's locale preference directly against the repository —
// a Given-step fixture, not an exercise of the UpdateMyLocale endpoint
// itself (that's what updatesLocale is for).
func (w *world) hasLocale(name, locale string) error {
	studentID := w.ensureRegistered(name, domain.RoleStudent)
	return w.users.UpdateLocale(w.ctx(), studentID.String(), locale)
}

// updatesLocale authenticates as name and calls UpdateMyLocale. It backs
// every "sets/submits/attempts to set their locale" step wording: the
// endpoint call is identical in each case, only the scenario's expected
// outcome (success, validation failure, not-found) differs downstream.
func (w *world) updatesLocale(name, locale string) error {
	w.hasToken = true
	w.clerkSub = clerkSub(name)
	w.persona = name
	resp, err := w.handler.UpdateMyLocale(w.ctx(), generated.UpdateMyLocaleRequestObject{
		Body: &generated.UpdateMyLocaleRequest{Locale: locale},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unauthUpdatesLocale(locale string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	resp, err := w.handler.UpdateMyLocale(w.ctx(), generated.UpdateMyLocaleRequestObject{
		Body: &generated.UpdateMyLocaleRequest{Locale: locale},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) localeUpdateResponseMatches(name, role, locale string) error {
	resp, ok := w.lastResp.(generated.UpdateMyLocale200JSONResponse)
	if !ok {
		return fmt.Errorf("expected a 200 response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	if resp.UserId != w.userMotifID[name] {
		return fmt.Errorf("expected user_id %s for %q, got %s", w.userMotifID[name], name, resp.UserId)
	}
	if string(resp.Role) != role {
		return fmt.Errorf("expected role %q, got %q", role, resp.Role)
	}
	if resp.Locale.Code != locale {
		return fmt.Errorf("expected locale %q, got %q", locale, resp.Locale.Code)
	}
	return nil
}

// nodeHasContentInLocale overrides slug's default languages (set to "en" by
// putContentNode) to the single given locale — used by both the "only in
// locale" and "in locale" step wordings, which differ narratively but not
// in the fixture they need: a node tagged for exactly one language.
func (w *world) nodeHasContentInLocale(slug, locale string) error {
	return w.setNodeLanguages(slug, []domain.Language{{Code: locale}})
}

func (w *world) nodeHasContentInAnyLocale(slug string) error {
	return w.setNodeLanguages(slug, []domain.Language{{Code: domain.LanguageCodeAny}})
}

func (w *world) setNodeLanguages(slug string, languages []domain.Language) error {
	node, err := w.nodes.GetByID(w.ctx(), nodeID(slug).String())
	if err != nil {
		return err
	}
	node.Languages = languages
	w.nodes.put(node)
	return nil
}
