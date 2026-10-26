//go:build integration

package bdd

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
)

func registerUserEmailPreferencesSteps(sc *godog.ScenarioContext, w *world) {
	// The stored email address.
	sc.Step(`^"([^"]+)"'s email address is "([^"]*)"$`, w.emailAddressIs)
	sc.Step(`^"([^"]+)"'s email address is now "([^"]*)"$`, w.emailAddressIsNow)
	sc.Step(`^"([^"]+)"'s identity token carries the email address "([^"]*)"$`, w.emailAddressIsNow)
	sc.Step(`^the identity token of "([^"]+)" (?:now )?carries the email "([^"]*)"$`, w.emailAddressIsNow)
	sc.Step(`^the identity token of "([^"]+)" (?:now )?carries no email address$`, w.carriesNoEmail)
	sc.Step(`^"([^"]+)" registered as a student before email addresses were stored$`, w.registeredBeforeEmails)
	sc.Step(`^the response includes the email address "([^"]+)"$`, w.responseIncludesEmail)
	sc.Step(`^the response shows no email address$`, w.responseShowsNoEmail)

	// Time zone.
	sc.Step(`^"([^"]+)" has no time zone set$`, func(name string) error { w.ensureRegistered(name, domain.RoleStudent); return nil })
	sc.Step(`^"([^"]+)"'s time zone is "([^"]+)"$`, w.timeZoneIs)
	sc.Step(`^"([^"]+)" sets their time zone to "([^"]+)"$`, w.setsTimeZone)
	sc.Step(`^"([^"]+)" attempts to set their time zone to "([^"]+)"$`, w.setsTimeZone)
	sc.Step(`^"([^"]+)" submits a profile update with time zone "([^"]+)"$`, w.setsTimeZone)
	sc.Step(`^"([^"]+)" sets their locale to "([^"]+)" and their time zone to "([^"]+)" in one update$`, w.setsLocaleAndTimeZone)
	sc.Step(`^an unauthenticated request attempts to set the time zone to "([^"]+)"$`, w.unauthSetsTimeZone)
	sc.Step(`^"([^"]+)" submits a profile update with no fields$`, w.submitsEmptyProfileUpdate)
	sc.Step(`^the response returns "([^"]+)"'s time zone "([^"]+)"$`, w.responseReturnsTimeZone)
	sc.Step(`^the response returns "([^"]+)"'s locale "([^"]+)"$`, w.responseReturnsLocale)
	sc.Step(`^the response returns "([^"]+)"'s locale "([^"]+)" and time zone "([^"]+)"$`, w.responseReturnsLocaleAndTimeZone)
	sc.Step(`^the response shows no time zone$`, w.responseShowsNoTimeZone)

	// Email preferences. The same sentence sets a preference in a Given and
	// checks it in a Then, so these two are bound by keyword.
	sc.Given(`^"([^"]+)" has the email preference "([^"]+)" (on|off)$`, w.hasEmailPreference)
	sc.Then(`^"([^"]+)" has the email preference "([^"]+)" (on|off)$`, w.storedEmailPreferenceIs)
	sc.Step(`^"([^"]+)" has the email preferences "([^"]+)" (on|off) and "([^"]+)" (on|off)$`, w.hasEmailPreferences)
	sc.Step(`^"([^"]+)" turns the email preference "([^"]+)" (on|off)$`, w.turnsEmailPreference)
	sc.Step(`^"([^"]+)" submits a profile update with the email preference "([^"]+)" (on|off)$`, w.turnsEmailPreference)
	sc.Step(`^the response shows the email preference "([^"]+)" (on|off)$`, w.responseShowsEmailPreference)

	// Unsubscribe links.
	sc.Step(`^"([^"]+)" holds the unsubscribe link from their progress report$`, w.holdsUnsubscribeLink(domain.EmailKindProgressReport))
	sc.Step(`^"([^"]+)" holds the unsubscribe link from a progress report sent \d+ months ago$`, w.holdsUnsubscribeLink(domain.EmailKindProgressReport))
	sc.Step(`^"([^"]+)" holds the unsubscribe link from their inactivity reminder$`, w.holdsUnsubscribeLink(domain.EmailKindReminders))
	sc.Step(`^the unsubscribe link has already been used$`, w.useUnsubscribeLink)
	sc.Step(`^"([^"]+)" used it to turn progress reports off, then turned them back on in settings$`, w.unsubscribedThenResubscribed)
	sc.Step(`^the unsubscribe link is used without signing in$`, w.useUnsubscribeLink)
	sc.Step(`^the unsubscribe link is used with one character of its token changed$`, w.useAlteredUnsubscribeLink)
	sc.Step(`^an unsubscribe request is sent with no token$`, func() error { return w.sendUnsubscribe("") })
	sc.Step(`^the unsubscribe succeeds$`, w.unsubscribeSucceeded)
}

// ── The stored email address ────────────────────────────────────────────────

// emailAddressIs sets the address persona's session token carries. Like
// isNamed, an already-registered persona makes one request with it straight
// away, which is what stores it.
func (w *world) emailAddressIs(persona, email string) error {
	w.emailClaims[persona] = email
	if _, registered := w.userMotifID[persona]; !registered {
		return nil
	}
	resp, err := w.handler.GetMyProfile(w.identityCtx(persona), generated.GetMyProfileRequestObject{})
	if err != nil {
		return err
	}
	if _, ok := resp.(generated.GetMyProfile200JSONResponse); !ok {
		return fmt.Errorf("setup: expected %q's profile request to succeed, got %#v", persona, resp)
	}
	return nil
}

// emailAddressIsNow changes the address persona's token carries without
// making a request — a change at the identity provider — so a scenario can
// show the stored address only follows on persona's next request.
func (w *world) emailAddressIsNow(persona, email string) error {
	w.emailClaims[persona] = email
	return nil
}

func (w *world) carriesNoEmail(persona string) error {
	w.emailClaims[persona] = ""
	return nil
}

// registeredBeforeEmails registers persona and then clears the address the
// registration stored, which is the state of a user whose row predates the
// email column.
func (w *world) registeredBeforeEmails(persona string) error {
	id := w.ensureRegistered(persona, domain.RoleStudent)
	user, err := w.users.GetByID(context.Background(), id.String())
	if err != nil {
		return err
	}
	user.Email = ""
	w.users.put(user)
	return nil
}

// lastProfile reads the profile out of whichever profile-shaped response the
// last step got: a registration, a profile request or a profile update.
func (w *world) lastProfile() (generated.UserProfile, error) {
	switch resp := w.lastResp.(type) {
	case generated.RegisterUser201JSONResponse:
		return generated.UserProfile(resp), nil
	case generated.GetMyProfile200JSONResponse:
		return generated.UserProfile(resp), nil
	case generated.UpdateMyProfile200JSONResponse:
		return generated.UserProfile(resp), nil
	default:
		return generated.UserProfile{}, fmt.Errorf("expected a profile response, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
}

func (w *world) responseIncludesEmail(want string) error {
	profile, err := w.lastProfile()
	if err != nil {
		return err
	}
	if profile.Email == nil || string(*profile.Email) != want {
		return fmt.Errorf("expected email address %q, got %v", want, profile.Email)
	}
	return nil
}

func (w *world) responseShowsNoEmail() error {
	profile, err := w.lastProfile()
	if err != nil {
		return err
	}
	if profile.Email != nil {
		return fmt.Errorf("expected no email address, got %q", *profile.Email)
	}
	return nil
}

// ── Time zone ───────────────────────────────────────────────────────────────

// timeZoneIs sets persona's stored time zone directly — a Given fixture,
// not an exercise of the profile update.
func (w *world) timeZoneIs(persona, zone string) error {
	id := w.ensureRegistered(persona, domain.RoleStudent)
	return w.users.UpdatePreferences(context.Background(), id.String(), domain.ProfileUpdate{TimeZone: &zone})
}

func (w *world) setsTimeZone(persona, zone string) error {
	return w.updateProfileAs(persona, &generated.UpdateMyProfileRequest{TimeZone: &zone})
}

func (w *world) setsLocaleAndTimeZone(persona, locale, zone string) error {
	return w.updateProfileAs(persona, &generated.UpdateMyProfileRequest{Locale: &locale, TimeZone: &zone})
}

func (w *world) submitsEmptyProfileUpdate(persona string) error {
	return w.updateProfileAs(persona, &generated.UpdateMyProfileRequest{})
}

func (w *world) unauthSetsTimeZone(zone string) error {
	w.noAuthToken() //nolint:errcheck // never errors
	resp, err := w.handler.UpdateMyProfile(w.ctx(), generated.UpdateMyProfileRequestObject{
		Body: &generated.UpdateMyProfileRequest{TimeZone: &zone},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

// updateProfileAs authenticates as persona and sends body as a profile
// update. persona need not be registered: a not-found scenario sends it as
// an unregistered identity.
func (w *world) updateProfileAs(persona string, body *generated.UpdateMyProfileRequest) error {
	w.hasToken = true
	w.clerkSub = clerkSub(persona)
	w.persona = persona
	resp, err := w.handler.UpdateMyProfile(w.identityCtx(persona), generated.UpdateMyProfileRequestObject{Body: body})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) responseReturnsTimeZone(persona, want string) error {
	profile, err := w.profileOf(persona)
	if err != nil {
		return err
	}
	if profile.TimeZone == nil || *profile.TimeZone != want {
		return fmt.Errorf("expected time zone %q, got %v", want, profile.TimeZone)
	}
	return nil
}

func (w *world) responseReturnsLocale(persona, want string) error {
	profile, err := w.profileOf(persona)
	if err != nil {
		return err
	}
	if profile.Locale.Code != want {
		return fmt.Errorf("expected locale %q, got %q", want, profile.Locale.Code)
	}
	return nil
}

func (w *world) responseReturnsLocaleAndTimeZone(persona, locale, zone string) error {
	if err := w.responseReturnsLocale(persona, locale); err != nil {
		return err
	}
	return w.responseReturnsTimeZone(persona, zone)
}

func (w *world) responseShowsNoTimeZone() error {
	profile, err := w.lastProfile()
	if err != nil {
		return err
	}
	if profile.TimeZone != nil {
		return fmt.Errorf("expected no time zone, got %q", *profile.TimeZone)
	}
	return nil
}

// profileOf returns the last response's profile after checking it is
// persona's.
func (w *world) profileOf(persona string) (generated.UserProfile, error) {
	profile, err := w.lastProfile()
	if err != nil {
		return generated.UserProfile{}, err
	}
	if profile.UserId != w.userMotifID[persona] {
		return generated.UserProfile{}, fmt.Errorf("expected %q's profile, got user %s", persona, profile.UserId)
	}
	return profile, nil
}

// ── Email preferences ───────────────────────────────────────────────────────

func (w *world) hasEmailPreference(persona, kind, state string) error {
	id := w.ensureRegistered(persona, domain.RoleStudent)
	update, err := emailPreferenceUpdate(kind, state == "on")
	if err != nil {
		return err
	}
	return w.users.UpdatePreferences(context.Background(), id.String(), domain.ProfileUpdate{EmailPreferences: &update})
}

func (w *world) hasEmailPreferences(persona, firstKind, firstState, secondKind, secondState string) error {
	if err := w.hasEmailPreference(persona, firstKind, firstState); err != nil {
		return err
	}
	return w.hasEmailPreference(persona, secondKind, secondState)
}

func (w *world) storedEmailPreferenceIs(persona, kind, state string) error {
	user, err := w.users.GetByID(context.Background(), w.userMotifID[persona].String())
	if err != nil {
		return err
	}
	got, err := emailPreferenceValue(user.EmailPreferences, kind)
	if err != nil {
		return err
	}
	if want := state == "on"; got != want {
		return fmt.Errorf("expected %q's email preference %q %s, got %t", persona, kind, state, got)
	}
	return nil
}

// turnsEmailPreference sends a profile update that switches kind. A kind the
// API doesn't know, such as "newsletter", is what the JSON decoder drops, so
// it arrives as an email_preferences object with no switch in it.
func (w *world) turnsEmailPreference(persona, kind, state string) error {
	w.ensureRegistered(persona, domain.RoleStudent)
	value := state == "on"
	prefs := &generated.EmailPreferencesUpdate{}
	switch domain.EmailKind(kind) {
	case domain.EmailKindProgressReport:
		prefs.ProgressReport = &value
	case domain.EmailKindReminders:
		prefs.Reminders = &value
	}
	return w.updateProfileAs(persona, &generated.UpdateMyProfileRequest{EmailPreferences: prefs})
}

func (w *world) responseShowsEmailPreference(kind, state string) error {
	profile, err := w.lastProfile()
	if err != nil {
		return err
	}
	prefs := domain.EmailPreferences{ProgressReport: profile.EmailPreferences.ProgressReport, Reminders: profile.EmailPreferences.Reminders}
	got, err := emailPreferenceValue(prefs, kind)
	if err != nil {
		return err
	}
	if want := state == "on"; got != want {
		return fmt.Errorf("expected email preference %q %s, got %t", kind, state, got)
	}
	return nil
}

func emailPreferenceUpdate(kind string, value bool) (domain.EmailPreferencesUpdate, error) {
	switch domain.EmailKind(kind) {
	case domain.EmailKindProgressReport:
		return domain.EmailPreferencesUpdate{ProgressReport: &value}, nil
	case domain.EmailKindReminders:
		return domain.EmailPreferencesUpdate{Reminders: &value}, nil
	default:
		return domain.EmailPreferencesUpdate{}, fmt.Errorf("unknown email preference %q", kind)
	}
}

func emailPreferenceValue(prefs domain.EmailPreferences, kind string) (bool, error) {
	switch domain.EmailKind(kind) {
	case domain.EmailKindProgressReport:
		return prefs.ProgressReport, nil
	case domain.EmailKindReminders:
		return prefs.Reminders, nil
	default:
		return false, fmt.Errorf("unknown email preference %q", kind)
	}
}

// ── Unsubscribe links ───────────────────────────────────────────────────────

// holdsUnsubscribeLink gives persona the token the unsubscribe link of a
// kind's email carries. Tokens don't expire, so the email's age changes
// nothing about the token.
func (w *world) holdsUnsubscribeLink(kind domain.EmailKind) func(string) error {
	return func(persona string) error {
		id := w.ensureRegistered(persona, domain.RoleStudent)
		w.unsubscribeToken = w.emailPreferences.UnsubscribeToken(id.String(), kind)
		return nil
	}
}

func (w *world) useUnsubscribeLink() error {
	return w.sendUnsubscribe(w.unsubscribeToken)
}

func (w *world) useAlteredUnsubscribeLink() error {
	token := w.unsubscribeToken
	last := token[len(token)-1]
	replacement := byte('A')
	if last == 'A' {
		replacement = 'B'
	}
	return w.sendUnsubscribe(token[:len(token)-1] + string(replacement))
}

func (w *world) unsubscribedThenResubscribed(persona string) error {
	if err := w.useUnsubscribeLink(); err != nil {
		return err
	}
	return w.hasEmailPreference(persona, string(domain.EmailKindProgressReport), "on")
}

// sendUnsubscribe sends what a mail client's one-click unsubscribe sends: a
// form POST with no session token.
func (w *world) sendUnsubscribe(token string) error {
	oneClick := "One-Click"
	resp, err := w.handler.UnsubscribeFromEmails(context.Background(), generated.UnsubscribeFromEmailsRequestObject{
		Params:       generated.UnsubscribeFromEmailsParams{Token: token},
		FormdataBody: &generated.UnsubscribeFromEmailsFormdataRequestBody{ListUnsubscribe: &oneClick},
	})
	w.lastResp, w.lastErr = resp, err
	return err
}

func (w *world) unsubscribeSucceeded() error {
	if _, ok := w.lastResp.(generated.UnsubscribeFromEmails200JSONResponse); !ok {
		return fmt.Errorf("expected the unsubscribe to succeed, got %#v (err=%v)", w.lastResp, w.lastErr)
	}
	return nil
}
