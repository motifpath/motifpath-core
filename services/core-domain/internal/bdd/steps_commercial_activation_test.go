//go:build integration

package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v34"
	"github.com/google/uuid"

	appHTTP "github.com/motifpath/core-domain/internal/adapters/http"
	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// commercialWorld is the commercial part of a scenario: its store, its
// services, the last admin API response and the intents it authorized.
// Given steps seed the store directly, so a scenario's revision numbers hold
// exactly; When steps go through the generated router as the named persona.
type commercialWorld struct {
	store   *fakeCommercialStore
	rollout *application.CommercialRolloutService
	audit   *application.CommercialAuditService

	stepType messages.PickleStepType

	lastStatus  int
	lastBody    []byte
	lastRequest commercialRequest
	firstBody   []byte
	revisionAt  int64
	groupsAt    map[string]domain.CommercialGroup

	intents   map[string]int64
	lastAuthz error

	evidence evidenceTraversal
}

type commercialRequest struct {
	persona, method, path, key string
	body                       []byte
}

func newCommercialWorld(w *world, newID func() string) *commercialWorld {
	store := newFakeCommercialStore(func(id string) bool {
		_, err := w.users.GetByID(context.Background(), id)
		return err == nil
	})
	env, err := domain.ParseEnvironment("test")
	if err != nil {
		panic(err)
	}
	now := func() time.Time { return fixedNow }
	commands := application.NewCommercialCommands(store, now, newID)
	return &commercialWorld{
		store:   store,
		rollout: application.NewCommercialRolloutService(store, commands, env, now),
		audit:   application.NewCommercialAuditService(store, env, []byte("bdd-commercial-cursor-secret"), now, newID),
		intents: map[string]int64{},
	}
}

func registerCommercialActivationSteps(sc *godog.ScenarioContext, w *world) {
	sc.StepContext().Before(func(ctx context.Context, st *godog.Step) (context.Context, error) {
		w.commercial.stepType = st.Type
		return ctx, nil
	})

	sc.Step(`^(\w+) is an admin$`, func(name string) error { w.ensureRegistered(name, domain.RoleAdmin); return nil })
	sc.Step(`^(\w+) is a teacher$`, func(name string) error { w.ensureRegistered(name, domain.RoleTeacher); return nil })
	sc.Step(`^(\w+) is a student$`, func(name string) error { w.ensureRegistered(name, domain.RoleStudent); return nil })

	// Reads
	sc.Step(`^no activation change has been made in this environment$`, func() error { return w.revisionIs(0) })
	sc.Step(`^(\w+) reads the commercial rollout$`, func(name string) error {
		return w.commercialSend(name, http.MethodGet, "/admin/commercial-rollout", "", nil)
	})
	sc.Step(`^every one of the 17 registered flags is disabled, not effective, not open to all users and has no subjects$`, w.everyFlagStartsOff)
	sc.Step(`^there are no rollout groups$`, func() error {
		if n := len(w.commercial.store.state.groups); n != 0 {
			return fmt.Errorf("there are %d rollout groups", n)
		}
		return nil
	})
	sc.Step(`^activation storage is unavailable$`, func() error { w.commercial.store.storageFails = true; return nil })
	sc.Step(`^audit evidence cannot be written$`, func() error { w.commercial.store.auditFails = true; return nil })
	sc.Step(`^the response is (\d+)$`, w.commercialResponseIs)

	// Revisions
	sc.Step(`^the activation revision is (\d+)$`, func(revision int64) error {
		if w.commercial.stepType == messages.PickleStepType_CONTEXT {
			w.commercial.store.state.revision = revision
			return nil
		}
		return w.revisionIs(revision)
	})
	sc.Step(`^the activation revision is now (\d+)$`, func(revision int64) error { w.commercial.store.state.revision = revision; return nil })
	sc.Step(`^the activation revision is still (\d+)$`, w.revisionIs)
	sc.Step(`^the activation revision is unchanged$`, func() error { return w.revisionIs(w.commercial.revisionAt) })
	sc.Step(`^(\w+) last read the (?:groups|rollout) at revision (\d+)$`, func(string, int64) error { return nil })

	// Changing a flag
	sc.Step(`^(\w+) sets? "([^"]+)" to enabled for "([^"]+)" at revision (\d+)$`, func(name, flag, subject string, revision int) error {
		return w.setFlag(name, flag, revision, true, false, []string{subject}, "rehearsal", "")
	})
	sc.Step(`^(\w+) sets? "([^"]+)" to enabled for "([^"]+)" at revision (\d+) with reason "([^"]+)"$`, func(name, flag, subject string, revision int, reason string) error {
		return w.setFlag(name, flag, revision, true, false, []string{subject}, reason, "")
	})
	sc.Step(`^(\w+) sets? "([^"]+)" to enabled for "([^"]+)" at revision (\d+) with idempotency key (\w+)$`, func(name, flag, subject string, revision int, key string) error {
		return w.setFlag(name, flag, revision, true, false, []string{subject}, "rehearsal", key)
	})
	sc.Step(`^(\w+) sets "([^"]+)" to disabled at revision (\d+) with idempotency key (\w+)$`, func(name, flag string, revision int, key string) error {
		return w.setFlag(name, flag, revision, false, false, nil, "rehearsal", key)
	})
	sc.Step(`^(\w+) sets "([^"]+)" to enabled at revision (\d+)$`, func(name, flag string, revision int) error {
		return w.setFlag(name, flag, revision, true, false, nil, "rehearsal", "")
	})
	sc.Step(`^(\w+) sets "([^"]+)" to enabled for all users at revision (\d+)$`, func(name, flag string, revision int) error {
		return w.setFlag(name, flag, revision, true, true, nil, "rehearsal", "")
	})
	sc.Step(`^(\w+) sets "([^"]+)" to enabled for all users at revision (\d+) with reason "([^"]+)"$`, func(name, flag string, revision int, reason string) error {
		return w.setFlag(name, flag, revision, true, true, nil, reason, "")
	})
	sc.Step(`^(\w+) sets "([^"]+)" to enabled for all users and "([^"]+)" at revision (\d+)$`, func(name, flag, subject string, revision int) error {
		return w.setFlag(name, flag, revision, true, true, []string{subject}, "rehearsal", "")
	})
	sc.Step(`^(\w+) sends the same (?:membership )?change again with idempotency key (\w+)$`, func(string, string) error {
		w.commercial.firstBody = w.commercial.lastBody
		req := w.commercial.lastRequest
		return w.commercialSend(req.persona, req.method, req.path, req.key, req.body)
	})

	sc.Step(`^the change is applied with activation revision (\d+) and an evidence entry$`, func(revision int64) error {
		return w.changeApplied(&revision, true)
	})
	sc.Step(`^the change is applied with activation revision (\d+)$`, func(revision int64) error { return w.changeApplied(&revision, false) })
	sc.Step(`^the change is applied with a new activation revision and an evidence entry$`, func() error { return w.changeApplied(nil, true) })
	sc.Step(`^the change is applied with a new activation revision$`, func() error { return w.changeApplied(nil, false) })
	sc.Step(`^the response repeats the original receipt(?: with activation revision (\d+))?$`, w.receiptRepeated)

	// Flag state
	sc.Step(`^"([^"]+)" is enabled for "([^"]+)"$`, func(flag, subject string) error {
		return w.seedFlag(flag, true, false, subject)
	})
	sc.Step(`^"([^"]+)" is enabled for "([^"]+)" at activation revision (\d+)$`, func(flag, subject string, revision int64) error {
		w.commercial.store.state.revision = revision
		return w.seedFlag(flag, true, false, subject)
	})
	sc.Step(`^"([^"]+)" is enabled with no subjects$`, func(flag string) error { return w.seedFlag(flag, true, false) })
	sc.Step(`^"([^"]+)" is enabled for all users$`, func(flag string) error { return w.seedFlag(flag, true, true) })
	sc.Step(`^"([^"]+)" is disabled$`, func(flag string) error { return w.seedFlag(flag, false, false) })
	sc.Step(`^"([^"]+)" is enabled and effective with subjects "([^"]+)"$`, w.flagEnabledEffectiveWith)
	sc.Step(`^"([^"]+)" is still enabled$`, func(flag string) error { return w.flagEnabledIs(flag, true) })
	sc.Step(`^"([^"]+)" is still disabled$`, func(flag string) error { return w.flagEnabledIs(flag, false) })
	sc.Step(`^the activation revision is still (\d+) and "([^"]+)" is still disabled$`, func(revision int64, flag string) error {
		if err := w.revisionIs(revision); err != nil {
			return err
		}
		return w.flagEnabledIs(flag, false)
	})
	sc.Step(`^every flag is disabled$`, func() error {
		for _, flag := range domain.CommercialFlags() {
			if err := w.flagEnabledIs(string(flag), false); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^"([^"]+)" still lists subject "([^"]+)"$`, w.flagStillLists)
	sc.Step(`^"([^"]+)" is still open to all users$`, func(flag string) error {
		if !w.commercial.store.state.flags[domain.FlagName(flag)].Audience.AllUsers {
			return fmt.Errorf("%s is no longer open to all users", flag)
		}
		return nil
	})
	sc.Step(`^the rollout shows "([^"]+)" as enabled but not effective$`, w.rolloutShowsEnabledNotEffective)
	sc.Step(`^"([^"]+)" allows "([^"]+)"$`, func(flag, subject string) error { return w.flagAllows(flag, subject, true) })
	sc.Step(`^"([^"]+)" does not allow "([^"]+)"$`, func(flag, subject string) error { return w.flagAllows(flag, subject, false) })

	// Evidence
	sc.Step(`^the evidence records (\w+), the reason, and "([^"]+)" disabled before and enabled after$`, func(name, flag string) error {
		return w.evidenceRecords(name, `"enabled":false`, `"enabled":true`)
	})
	sc.Step(`^the evidence records (\w+), the reason, and "([^"]+)" not open to all users before and open after$`, func(name, flag string) error {
		return w.evidenceRecords(name, `"all_users":false`, `"all_users":true`)
	})

	// Groups
	sc.Step(`^(\w+) saves group "([^"]+)" named "([^"]+)"(?: with reason "([^"]+)")?$`, w.saveGroup)
	sc.Step(`^group "([^"]+)" exists with no members$`, func(slug string) error { w.seedGroup(slug); return nil })
	sc.Step(`^group "([^"]+)" has members? (\w+)(?: and (\w+))?$`, w.groupHasMembers)
	sc.Step(`^group "([^"]+)" has no members$`, func(slug string) error { return w.groupMembersAre(slug) })
	sc.Step(`^group "([^"]+)" is named "([^"]+)" and has no members$`, func(slug, name string) error {
		if err := w.groupNamed(slug, name); err != nil {
			return err
		}
		return w.groupMembersAre(slug)
	})
	sc.Step(`^group "([^"]+)" is named "([^"]+)" and has members (\w+) and (\w+)$`, func(slug, name, a, b string) error {
		if err := w.groupNamed(slug, name); err != nil {
			return err
		}
		return w.groupMembersAre(slug, a, b)
	})
	sc.Step(`^group "([^"]+)" is unchanged$`, w.groupUnchanged)
	sc.Step(`^(\w+) adds (\w+) to group "([^"]+)"(?: with reason "([^"]+)")?$`, func(name, member, slug, reason string) error {
		return w.changeMembers(name, slug, []string{w.commercialUserID(member)}, nil, reason, "")
	})
	sc.Step(`^(\w+) added (\w+) to group "([^"]+)" with idempotency key (\w+)$`, func(name, member, slug, key string) error {
		w.seedGroup(slug)
		return w.changeMembers(name, slug, []string{w.commercialUserID(member)}, nil, "invite", key)
	})
	sc.Step(`^(\w+) removes (\w+) from group "([^"]+)"$`, func(name, member, slug string) error {
		return w.changeMembers(name, slug, nil, []string{w.commercialUserID(member)}, "remove", "")
	})
	sc.Step(`^(\w+) adds (\w+) and removes (\w+) in group "([^"]+)"$`, func(name, add, remove, slug string) error {
		return w.changeMembers(name, slug, []string{w.commercialUserID(add)}, []string{w.commercialUserID(remove)}, "change", "")
	})
	sc.Step(`^(\w+) adds an unknown user to group "([^"]+)"$`, func(name, slug string) error {
		return w.changeMembers(name, slug, []string{uuid.NewSHA1(uuid.NameSpaceOID, []byte("nobody")).String()}, nil, "invite", "")
	})

	// Kill switch and intents
	sc.Step(`^(\w+) turns off all commercial flags(?: with reason "([^"]+)")?$`, w.killSwitch)
	sc.Step(`^a commercial intent for "([^"]+)" and "([^"]+)" was authorized at revision (\d+)$`, w.intentAuthorizedAt)
	sc.Step(`^a new commercial intent for "([^"]+)" and "([^"]+)" is (?:then )?authorized$`, func(flag, subject string) error {
		_, w.commercial.lastAuthz = w.authorizeIntent(flag, subject)
		return nil
	})
	sc.Step(`^a new commercial intent for "([^"]+)" and "([^"]+)" is refused$`, func(flag, subject string) error {
		_, err := w.authorizeIntent(flag, subject)
		return expectRefused(err)
	})
	sc.Step(`^the intent is refused$`, func() error { return expectRefused(w.commercial.lastAuthz) })
	sc.Step(`^(?:the earlier|that) intent still records activation revision (\d+) and can be completed$`, w.intentKeepsRevision)
	sc.Step(`^(\w+) turns off all commercial flags while a new intent for "([^"]+)" and "([^"]+)" is being authorized$`, w.killSwitchRacingIntent)
	sc.Step(`^either the intent records the revision before OFF and committed before it, or it is refused$`, w.raceOutcomeIsSafe)
}

// ── Requests ─────────────────────────────────────────────────────────────────

func (w *world) commercialSend(persona, method, path, key string, body []byte) error {
	w.commercial.revisionAt = w.commercial.store.state.revision
	w.commercial.groupsAt = w.commercial.store.state.clone().groups
	w.commercial.lastRequest = commercialRequest{persona: persona, method: method, path: path, key: key, body: body}

	router := generated.HandlerWithOptions(appHTTP.NewStrictHandler(w.handler), generated.ChiServerOptions{})
	ctx := appHTTP.WithClerkUserID(context.Background(), clerkSub(persona))
	req := httptest.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	w.commercial.lastStatus, w.commercial.lastBody = rec.Code, rec.Body.Bytes()
	return nil
}

// idempotencyKey names a scenario's key ("K") as a stable UUID; an unnamed
// key is a fresh one.
func idempotencyKey(name string) string {
	if name == "" {
		return uuid.NewString()
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("idempotency-"+name)).String()
}

func (w *world) setFlag(persona, flag string, revision int, enabled, allUsers bool, subjects []string, reason, key string) error {
	ids := make([]string, len(subjects))
	for i, subject := range subjects {
		ids[i] = w.commercialSubject(subject)
	}
	body, err := json.Marshal(generated.FlagChange{
		ExpectedRevision: revision, Enabled: enabled, AllUsers: allUsers, SubjectIds: ids, Reason: reason,
	})
	if err != nil {
		return err
	}
	return w.commercialSend(persona, http.MethodPut, "/admin/commercial-rollout/"+flag, idempotencyKey(key), body)
}

func (w *world) saveGroup(persona, slug, name, reason string) error {
	if reason == "" {
		reason = "cohort"
	}
	body, err := json.Marshal(generated.GroupChange{Name: name, Reason: reason})
	if err != nil {
		return err
	}
	return w.commercialSend(persona, http.MethodPut, "/admin/commercial-groups/"+slug, idempotencyKey(""), body)
}

func (w *world) changeMembers(persona, slug string, add, remove []string, reason, key string) error {
	if reason == "" {
		reason = "membership"
	}
	raw, err := json.Marshal(struct {
		Add    []string `json:"add"`
		Remove []string `json:"remove"`
		Reason string   `json:"reason"`
	}{nonNil(add), nonNil(remove), reason})
	if err != nil {
		return err
	}
	return w.commercialSend(persona, http.MethodPost, "/admin/commercial-groups/"+slug+"/membership", idempotencyKey(key), raw)
}

func (w *world) killSwitch(persona, reason string) error {
	if reason == "" {
		reason = "Incident"
	}
	body, err := json.Marshal(generated.KillSwitch{Reason: reason})
	if err != nil {
		return err
	}
	return w.commercialSend(persona, http.MethodPost, "/admin/commercial-rollout/kill-switch", idempotencyKey(""), body)
}

// ── Assertions on responses ──────────────────────────────────────────────────

func (w *world) commercialResponseIs(status int) error {
	if w.commercial.lastStatus != status {
		return fmt.Errorf("expected %d, got %d: %s", status, w.commercial.lastStatus, w.commercial.lastBody)
	}
	return nil
}

func (w *world) lastReceipt() (generated.RolloutCommand, error) {
	var receipt generated.RolloutCommand
	if w.commercial.lastStatus != http.StatusOK {
		return receipt, fmt.Errorf("expected an applied change, got %d: %s", w.commercial.lastStatus, w.commercial.lastBody)
	}
	return receipt, json.Unmarshal(w.commercial.lastBody, &receipt)
}

func (w *world) changeApplied(revision *int64, withEvidence bool) error {
	receipt, err := w.lastReceipt()
	if err != nil {
		return err
	}
	if receipt.Status != generated.RolloutCommandStatusApplied {
		return fmt.Errorf("status is %q", receipt.Status)
	}
	want := w.commercial.revisionAt + 1
	if revision != nil {
		want = *revision
	}
	if int64(receipt.Revision) != want || w.commercial.store.state.revision != want {
		return fmt.Errorf("receipt revision %d, store revision %d, want %d", receipt.Revision, w.commercial.store.state.revision, want)
	}
	if withEvidence {
		_, err := w.evidenceEntry(receipt.EvidenceEntryId.String())
		return err
	}
	return nil
}

func (w *world) receiptRepeated(revision string) error {
	if !bytes.Equal(w.commercial.firstBody, w.commercial.lastBody) {
		return fmt.Errorf("replay answered %s, the original was %s", w.commercial.lastBody, w.commercial.firstBody)
	}
	if revision == "" {
		return nil
	}
	receipt, err := w.lastReceipt()
	if err != nil {
		return err
	}
	if fmt.Sprint(receipt.Revision) != revision {
		return fmt.Errorf("receipt revision %d, want %s", receipt.Revision, revision)
	}
	return nil
}

func (w *world) everyFlagStartsOff() error {
	var rollout generated.CommercialRollout
	if err := json.Unmarshal(w.commercial.lastBody, &rollout); err != nil {
		return err
	}
	if len(rollout.Flags) != 17 {
		return fmt.Errorf("the rollout has %d flags", len(rollout.Flags))
	}
	for _, flag := range rollout.Flags {
		if flag.Enabled || flag.Effective || flag.AllUsers || len(flag.SubjectIds) != 0 {
			return fmt.Errorf("flag %s does not start OFF: %+v", flag.Flag, flag)
		}
	}
	return nil
}

func (w *world) rolloutShowsEnabledNotEffective(flag string) error {
	if err := w.commercialSend("Ada", http.MethodGet, "/admin/commercial-rollout", "", nil); err != nil {
		return err
	}
	var rollout generated.CommercialRollout
	if err := json.Unmarshal(w.commercial.lastBody, &rollout); err != nil {
		return err
	}
	for _, state := range rollout.Flags {
		if string(state.Flag) == flag {
			if !state.Enabled || state.Effective {
				return fmt.Errorf("%s shows enabled=%v effective=%v", flag, state.Enabled, state.Effective)
			}
			return nil
		}
	}
	return fmt.Errorf("the rollout does not show %s", flag)
}

func (w *world) evidenceEntry(entryID string) (domain.AuditEntry, error) {
	for _, record := range w.commercial.store.state.audit {
		if record.Entry.AuditEntryID == entryID {
			return record.Entry, nil
		}
	}
	return domain.AuditEntry{}, fmt.Errorf("no evidence entry %s", entryID)
}

func (w *world) evidenceRecords(persona, before, after string) error {
	receipt, err := w.lastReceipt()
	if err != nil {
		return err
	}
	entry, err := w.evidenceEntry(receipt.EvidenceEntryId.String())
	if err != nil {
		return err
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(w.commercial.lastRequest.body, &request); err != nil {
		return err
	}
	switch {
	case entry.Operation.ActorID != w.userMotifID[persona].String():
		return fmt.Errorf("evidence actor is %s, not %s", entry.Operation.ActorID, persona)
	case entry.Reason != request.Reason:
		return fmt.Errorf("evidence reason is %q, not %q", entry.Reason, request.Reason)
	case !strings.Contains(string(entry.BeforeState), before):
		return fmt.Errorf("before state %s lacks %s", entry.BeforeState, before)
	case !strings.Contains(string(entry.AfterState), after):
		return fmt.Errorf("after state %s lacks %s", entry.AfterState, after)
	}
	return nil
}

// ── State ────────────────────────────────────────────────────────────────────

func (w *world) revisionIs(revision int64) error {
	if got := w.commercial.store.state.revision; got != revision {
		return fmt.Errorf("activation revision is %d, want %d", got, revision)
	}
	return nil
}

// commercialUserID is a persona's core user id; the persona is registered
// as a student if the scenario has not met them yet.
func (w *world) commercialUserID(persona string) string {
	if id, ok := w.userMotifID[persona]; ok {
		return id.String()
	}
	return w.ensureRegistered(persona, domain.RoleStudent).String()
}

// commercialSubject turns "user:Bea" into "user:<Bea's id>"; groups and
// services are already literal.
func (w *world) commercialSubject(subject string) string {
	if name, isUser := strings.CutPrefix(subject, "user:"); isUser {
		return "user:" + w.commercialUserID(name)
	}
	return subject
}

func (w *world) parsedSubject(subject string) (domain.RolloutSubject, error) {
	return domain.ParseRolloutSubject(w.commercialSubject(subject))
}

func (w *world) seedFlag(flag string, enabled, allUsers bool, subjects ...string) error {
	entries := make([]domain.RolloutSubject, len(subjects))
	for i, subject := range subjects {
		parsed, err := w.parsedSubject(subject)
		if err != nil {
			return err
		}
		if parsed.Kind == domain.RolloutSubjectGroup {
			w.seedGroup(parsed.ID)
		}
		entries[i] = parsed
	}
	w.commercial.store.state.flags[domain.FlagName(flag)] = domain.FlagSetting{
		Enabled: enabled, Audience: domain.RolloutAudience{AllUsers: allUsers, Entries: entries},
	}
	return nil
}

func (w *world) flagEnabledIs(flag string, enabled bool) error {
	if got := w.commercial.store.state.flags[domain.FlagName(flag)].Enabled; got != enabled {
		return fmt.Errorf("%s enabled is %v, want %v", flag, got, enabled)
	}
	return nil
}

func (w *world) flagEnabledEffectiveWith(flag, subject string) error {
	activation, err := w.commercial.store.ReadActivation(context.Background())
	if err != nil {
		return err
	}
	setting := activation.Setting(domain.FlagName(flag))
	if !setting.Enabled || !activation.Effective(domain.FlagName(flag)) {
		return fmt.Errorf("%s is not enabled and effective", flag)
	}
	if len(setting.Audience.Entries) != 1 || setting.Audience.Entries[0].String() != w.commercialSubject(subject) {
		return fmt.Errorf("%s lists %v, want %s", flag, setting.Audience.Entries, subject)
	}
	return nil
}

func (w *world) flagStillLists(flag, subject string) error {
	for _, entry := range w.commercial.store.state.flags[domain.FlagName(flag)].Audience.Entries {
		if entry.String() == w.commercialSubject(subject) {
			return nil
		}
	}
	return fmt.Errorf("%s no longer lists %s", flag, subject)
}

func (w *world) flagAllows(flag, subject string, want bool) error {
	parsed, err := w.parsedSubject(subject)
	if err != nil {
		return err
	}
	allowed, err := w.commercial.rollout.Allowed(context.Background(), domain.FlagName(flag), parsed)
	if err != nil && !errors.Is(err, domain.ErrUnavailable) {
		return err
	}
	if allowed != want {
		return fmt.Errorf("%s allows %s: %v, want %v (err=%v)", flag, subject, allowed, want, err)
	}
	return nil
}

func (w *world) seedGroup(slug string) {
	if _, ok := w.commercial.store.state.groups[slug]; ok {
		return
	}
	w.commercial.store.state.groups[slug] = domain.CommercialGroup{
		Slug: slug, Name: slug, MemberIDs: []string{}, UpdatedAt: fixedNow, UpdatedBy: w.commercialUserID("Ada"),
	}
}

func (w *world) groupHasMembers(slug, first, second string) error {
	members := []string{first}
	if second != "" {
		members = append(members, second)
	}
	if w.commercial.stepType != messages.PickleStepType_CONTEXT {
		return w.groupMembersAre(slug, members...)
	}
	w.seedGroup(slug)
	group := w.commercial.store.state.groups[slug]
	for _, member := range members {
		group.MemberIDs = append(group.MemberIDs, w.commercialUserID(member))
	}
	w.commercial.store.state.groups[slug] = group
	return nil
}

func (w *world) groupMembersAre(slug string, members ...string) error {
	group, ok := w.commercial.store.state.groups[slug]
	if !ok {
		return fmt.Errorf("there is no group %s", slug)
	}
	want := map[string]bool{}
	for _, member := range members {
		want[w.commercialUserID(member)] = true
	}
	if len(group.MemberIDs) != len(want) {
		return fmt.Errorf("group %s has members %v, want %v", slug, group.MemberIDs, members)
	}
	for _, id := range group.MemberIDs {
		if !want[id] {
			return fmt.Errorf("group %s has member %s, want %v", slug, id, members)
		}
	}
	return nil
}

func (w *world) groupNamed(slug, name string) error {
	if got := w.commercial.store.state.groups[slug].Name; got != name {
		return fmt.Errorf("group %s is named %q, want %q", slug, got, name)
	}
	return nil
}

func (w *world) groupUnchanged(slug string) error {
	before, existed := w.commercial.groupsAt[slug]
	after, exists := w.commercial.store.state.groups[slug]
	if existed != exists || before.Name != after.Name || strings.Join(before.MemberIDs, ",") != strings.Join(after.MemberIDs, ",") {
		return fmt.Errorf("group %s changed from %+v to %+v", slug, before, after)
	}
	return nil
}

// ── Intents ──────────────────────────────────────────────────────────────────

// authorizeIntent authorizes a test-only commercial intent the way an
// operation will: inside its own transaction, under the shared lock.
func (w *world) authorizeIntent(flag, subject string) (int64, error) {
	parsed, err := w.parsedSubject(subject)
	if err != nil {
		return 0, err
	}
	var revision int64
	err = w.commercial.store.InTx(context.Background(), func(ctx context.Context, tx ports.CommercialTx) error {
		var err error
		revision, err = w.commercial.rollout.AuthorizeIntent(ctx, tx, domain.FlagName(flag), parsed)
		return err
	})
	return revision, err
}

func (w *world) intentAuthorizedAt(flag, subject string, revision int64) error {
	got, err := w.authorizeIntent(flag, subject)
	if err != nil {
		return err
	}
	if got != revision {
		return fmt.Errorf("intent authorized at revision %d, want %d", got, revision)
	}
	w.commercial.intents[flag+"|"+subject] = got
	return nil
}

func (w *world) intentKeepsRevision(revision int64) error {
	for _, recorded := range w.commercial.intents {
		if recorded == revision {
			return nil
		}
	}
	return fmt.Errorf("no recorded intent at revision %d (have %v)", revision, w.commercial.intents)
}

func expectRefused(err error) error {
	if !errors.Is(err, domain.ErrForbidden) {
		return fmt.Errorf("expected the intent to be refused, got %v", err)
	}
	return nil
}

// killSwitchRacingIntent runs the OFF change and the intent one after the
// other. The in-memory store cannot interleave them; the real interleaving,
// with Postgres locks, is proven by the commercial store's integration test.
func (w *world) killSwitchRacingIntent(persona, flag, subject string) error {
	before := w.commercial.store.state.revision
	if err := w.killSwitch(persona, "Incident"); err != nil {
		return err
	}
	revision, err := w.authorizeIntent(flag, subject)
	w.commercial.lastAuthz = err
	if err == nil {
		w.commercial.intents["race"] = revision
	}
	w.commercial.revisionAt = before
	return nil
}

func (w *world) raceOutcomeIsSafe() error {
	if errors.Is(w.commercial.lastAuthz, domain.ErrForbidden) {
		return nil
	}
	if revision, ok := w.commercial.intents["race"]; ok && revision == w.commercial.revisionAt {
		return nil
	}
	return fmt.Errorf("the intent was authorized after OFF (err=%v, intents=%v)", w.commercial.lastAuthz, w.commercial.intents)
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
