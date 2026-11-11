//go:build integration

package bdd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/http/generated"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// The personas the commercial audit scenarios speak about without naming.
const (
	auditAdmin    = "Ada"
	auditCustomer = "Cleo"
	auditOperator = "Otto"
)

// "New purchases" in the rollout scenarios is the Plus checkout flag: every
// acquisition flag is changed through the same audited command.
const newPurchasesFlag = "plus_checkout"

// evidenceTraversal is what an audit scenario remembers between steps: the
// entries seeded for it, the pages read so far and where to continue.
type evidenceTraversal struct {
	seeded   []string
	appended string
	read     []string
	cursor   string
	scope    url.Values
}

func registerCommercialAuditSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^an authenticated customer without administrator permission$`, func() error {
		w.ensureRegistered(auditCustomer, domain.RoleStudent)
		w.seedEvidence("course", "Harmony", uuid.NewString(), auditAdmin)
		return nil
	})
	sc.Step(`^the customer requests commercial audit evidence$`, func() error {
		return w.readEvidence(auditCustomer, courseScope("Harmony"), "")
	})
	sc.Step(`^access is refused without disclosing matching records$`, w.evidenceReadRefused)

	sc.Step(`^an administrator has received the first page of a commercial history snapshot$`, w.firstPageOfSnapshot)
	sc.Step(`^new commercial evidence is appended$`, func() error {
		w.commercial.evidence.appended = w.seedEvidence("course", "Harmony", uuid.NewString(), auditAdmin)
		return nil
	})
	sc.Step(`^the administrator continues that snapshot to its end$`, w.continueSnapshotToEnd)
	sc.Step(`^each matching snapshot entry appears exactly once in stable order$`, func() error {
		return equalIDs("snapshot", w.commercial.evidence.seeded, w.commercial.evidence.read)
	})
	sc.Step(`^later entries require a new traversal$`, w.laterEntriesNeedNewTraversal)

	sc.Step(`^an administrator has a continuation for course "([^"]+)"$`, w.continuationFor)
	sc.Step(`^the administrator uses it to request course "([^"]+)"$`, func(course string) error {
		return w.readEvidence(auditAdmin, courseScope(course), w.commercial.evidence.cursor)
	})
	sc.Step(`^the continuation is refused$`, func() error { return w.commercialResponseIs(http.StatusBadRequest) })

	sc.Step(`^two trusted actors used the same operation UUID for different commercial resources$`, func() error {
		w.ensureRegistered(auditAdmin, domain.RoleAdmin)
		operationID := uuid.NewString()
		w.commercial.evidence.scope = url.Values{"operation_id": {operationID}}
		w.commercial.evidence.seeded = []string{
			w.seedEvidence("course", "Harmony", operationID, "Bea"),
			w.seedEvidence("path", "Guitar foundations", operationID, "Cy"),
		}
		return nil
	})
	sc.Step(`^an administrator filters commercial evidence by that UUID$`, func() error {
		return w.readEvidence(auditAdmin, w.commercial.evidence.scope, "")
	})
	sc.Step(`^both operations remain distinguishable by actor, action, resource and environment$`, w.operationsDistinguishable)
	sc.Step(`^each evidence entry has its own immutable identity$`, w.entriesHaveOwnIdentity)

	sc.Step(`^an administrator inspects a commercial history snapshot$`, func() error {
		w.ensureRegistered(auditAdmin, domain.RoleAdmin)
		w.commercial.evidence.seeded = []string{w.seedEvidence("course", "Harmony", uuid.NewString(), auditAdmin)}
		return w.readEvidence(auditAdmin, courseScope("Harmony"), "")
	})
	sc.Step(`^the inspection is recorded$`, func() error { return w.commercialResponseIs(http.StatusOK) })
	sc.Step(`^the access evidence identifies the administrator and requested scope$`, w.accessEvidenceIdentifies)
	sc.Step(`^that new entry is outside the inspected snapshot$`, w.accessEvidenceOutsideSnapshot)

	sc.Step(`^authoritative audit storage is available$`, func() error { w.commercial.store.storageFails = false; return nil })
	sc.Step(`^authoritative audit storage is unavailable$`, func() error { w.commercial.store.storageFails = true; return nil })
	sc.Step(`^no evidence matches the requested commercial resource$`, func() error {
		w.seedEvidence("course", "Rhythm", uuid.NewString(), auditAdmin)
		return nil
	})
	sc.Step(`^an administrator inspects (?:its|a) commercial history$`, func() error {
		w.ensureRegistered(auditAdmin, domain.RoleAdmin)
		return w.readEvidence(auditAdmin, courseScope("Harmony"), "")
	})
	sc.Step(`^the result is an empty completed traversal$`, w.emptyCompletedTraversal)
	sc.Step(`^the inspection reports that evidence is unavailable$`, func() error {
		return w.commercialResponseIs(http.StatusServiceUnavailable)
	})
	sc.Step(`^it does not return an apparently empty history$`, func() error {
		if strings.Contains(string(w.commercial.lastBody), `"items"`) {
			return fmt.Errorf("an unavailable read returned a history: %s", w.commercial.lastBody)
		}
		return nil
	})

	sc.Step(`^new purchases are disabled$`, func() error { return w.seedFlag(newPurchasesFlag, false, false) })
	sc.Step(`^durable activation-change evidence cannot be recorded$`, func() error { w.commercial.store.auditFails = true; return nil })
	sc.Step(`^an authorized operator attempts to enable new purchases$`, func() error {
		w.ensureRegistered(auditOperator, domain.RoleAdmin)
		revision := int(w.commercial.store.state.revision)
		return w.setFlag(auditOperator, newPurchasesFlag, revision, true, true, nil, "launch purchases", "")
	})
	sc.Step(`^new purchases remain disabled$`, func() error {
		if w.commercial.lastStatus == http.StatusOK {
			return fmt.Errorf("enabling purchases without evidence was accepted: %s", w.commercial.lastBody)
		}
		return w.flagEnabledIs(newPurchasesFlag, false)
	})
}

func courseScope(course string) url.Values {
	return url.Values{"resource_kind": {"course"}, "resource_id": {course}}
}

// seedEvidence appends one applied entry for a resource, as an earlier
// command would have, and returns its entry id. The administrator persona
// is registered as an admin first, so naming her never makes her a student.
func (w *world) seedEvidence(resourceKind, resourceID, operationID, actor string) string {
	if actor == auditAdmin {
		w.ensureRegistered(auditAdmin, domain.RoleAdmin)
	}
	env, err := domain.ParseEnvironment("test")
	if err != nil {
		panic(err)
	}
	entryID := uuid.NewString()
	state := &w.commercial.store.state
	state.audit = append(state.audit, ports.AuditRecord{
		OrderKey: int64(len(state.audit) + 1),
		Entry: domain.AuditEntry{
			AuditEntryID: entryID,
			Operation: domain.OperationKey{
				Environment: env, ActorKind: domain.AuditActorUser, ActorID: w.commercialUserID(actor),
				Action: resourceKind + ".price_changed", ResourceKind: resourceKind, ResourceID: resourceID,
				CallerKey: operationID,
			},
			AttemptID: uuid.NewString(), CorrelationID: uuid.NewString(),
			OccurredAt: fixedNow, RecordedAt: fixedNow,
			Reason: "seeded history", Outcome: domain.AuditOutcomeApplied,
			PolicyRevision: domain.CommercialRegistryRevision, ConfigurationRevision: "1",
		},
	})
	return entryID
}

func (w *world) readEvidence(persona string, scope url.Values, cursor string) error {
	query := url.Values{}
	for key, values := range scope {
		query[key] = values
	}
	query.Set("limit", "2")
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	return w.commercialSend(persona, http.MethodGet, "/admin/commercial-audit?"+query.Encode(), "", nil)
}

func (w *world) evidencePage() (generated.AuditPage, error) {
	if err := w.commercialResponseIs(http.StatusOK); err != nil {
		return generated.AuditPage{}, err
	}
	var page generated.AuditPage
	if err := json.Unmarshal(w.commercial.lastBody, &page); err != nil {
		return generated.AuditPage{}, fmt.Errorf("decode audit page: %w", err)
	}
	return page, nil
}

// readPage reads the last response as a page, remembers its entries and
// where to continue, and reports whether there is more.
func (w *world) readPage() (bool, error) {
	page, err := w.evidencePage()
	if err != nil {
		return false, err
	}
	for _, item := range page.Items {
		w.commercial.evidence.read = append(w.commercial.evidence.read, item.AuditEntryId.String())
	}
	w.commercial.evidence.cursor = ""
	if page.NextCursor != nil {
		w.commercial.evidence.cursor = *page.NextCursor
	}
	return page.NextCursor != nil, nil
}

func (w *world) evidenceReadRefused() error {
	if err := w.commercialResponseIs(http.StatusForbidden); err != nil {
		return err
	}
	for _, record := range w.commercial.store.state.audit {
		if strings.Contains(string(w.commercial.lastBody), record.Entry.AuditEntryID) {
			return fmt.Errorf("the refusal disclosed entry %s", record.Entry.AuditEntryID)
		}
	}
	return nil
}

func (w *world) firstPageOfSnapshot() error {
	w.ensureRegistered(auditAdmin, domain.RoleAdmin)
	for range 3 {
		w.commercial.evidence.seeded = append(w.commercial.evidence.seeded,
			w.seedEvidence("course", "Harmony", uuid.NewString(), auditAdmin))
	}
	if err := w.readEvidence(auditAdmin, courseScope("Harmony"), ""); err != nil {
		return err
	}
	more, err := w.readPage()
	if err != nil {
		return err
	}
	if !more {
		return fmt.Errorf("the first page already ended the snapshot")
	}
	return nil
}

func (w *world) continueSnapshotToEnd() error {
	for more := true; more; {
		if err := w.readEvidence(auditAdmin, courseScope("Harmony"), w.commercial.evidence.cursor); err != nil {
			return err
		}
		next, err := w.readPage()
		if err != nil {
			return err
		}
		more = next
	}
	return nil
}

func (w *world) laterEntriesNeedNewTraversal() error {
	if slices.Contains(w.commercial.evidence.read, w.commercial.evidence.appended) {
		return fmt.Errorf("the appended entry %s appeared in the older snapshot", w.commercial.evidence.appended)
	}
	w.commercial.evidence.read = nil
	if err := w.continueSnapshotToEnd(); err != nil {
		return err
	}
	if !slices.Contains(w.commercial.evidence.read, w.commercial.evidence.appended) {
		return fmt.Errorf("a new traversal does not include the appended entry %s", w.commercial.evidence.appended)
	}
	return nil
}

func (w *world) continuationFor(course string) error {
	w.ensureRegistered(auditAdmin, domain.RoleAdmin)
	for range 3 {
		w.seedEvidence("course", course, uuid.NewString(), auditAdmin)
	}
	if err := w.readEvidence(auditAdmin, courseScope(course), ""); err != nil {
		return err
	}
	more, err := w.readPage()
	if err != nil {
		return err
	}
	if !more {
		return fmt.Errorf("no continuation was returned for course %q", course)
	}
	return nil
}

func (w *world) operationsDistinguishable() error {
	page, err := w.evidencePage()
	if err != nil {
		return err
	}
	if len(page.Items) != 2 {
		return fmt.Errorf("want both operations, got %d entries", len(page.Items))
	}
	identities := map[string]bool{}
	for _, item := range page.Items {
		if item.ActorId == "" || item.Action == "" || item.ResourceKind == "" || item.ResourceId == "" || item.Environment == "" {
			return fmt.Errorf("entry %s lacks part of its operation identity", item.AuditEntryId)
		}
		identities[strings.Join([]string{item.ActorId, item.Action, item.ResourceKind, item.ResourceId, item.Environment}, "|")] = true
	}
	if len(identities) != 2 {
		return fmt.Errorf("the two operations cannot be told apart")
	}
	return nil
}

func (w *world) entriesHaveOwnIdentity() error {
	page, err := w.evidencePage()
	if err != nil {
		return err
	}
	ids := make([]string, len(page.Items))
	for i, item := range page.Items {
		ids[i] = item.AuditEntryId.String()
	}
	slices.Sort(ids)
	expected := slices.Clone(w.commercial.evidence.seeded)
	slices.Sort(expected)
	return equalIDs("entry identities", expected, ids)
}

func (w *world) accessEvidence() (domain.AuditEntry, error) {
	for _, record := range w.commercial.store.state.audit {
		if record.Entry.Operation.Action == "commercial_audit.read" {
			return record.Entry, nil
		}
	}
	return domain.AuditEntry{}, fmt.Errorf("the read left no access evidence")
}

func (w *world) accessEvidenceIdentifies() error {
	entry, err := w.accessEvidence()
	if err != nil {
		return err
	}
	if entry.Operation.ActorID != w.commercialUserID(auditAdmin) {
		return fmt.Errorf("access evidence names actor %s, not the administrator", entry.Operation.ActorID)
	}
	scope := string(entry.AfterState)
	if !strings.Contains(scope, `"course"`) || !strings.Contains(scope, `"Harmony"`) {
		return fmt.Errorf("access evidence does not record the requested scope: %s", scope)
	}
	return nil
}

func (w *world) accessEvidenceOutsideSnapshot() error {
	entry, err := w.accessEvidence()
	if err != nil {
		return err
	}
	if _, err := w.readPage(); err != nil {
		return err
	}
	if slices.Contains(w.commercial.evidence.read, entry.AuditEntryID) {
		return fmt.Errorf("the access evidence appeared in the snapshot it records")
	}
	return equalIDs("snapshot", w.commercial.evidence.seeded, w.commercial.evidence.read)
}

func (w *world) emptyCompletedTraversal() error {
	page, err := w.evidencePage()
	if err != nil {
		return err
	}
	if len(page.Items) != 0 || page.NextCursor != nil {
		return fmt.Errorf("want an empty, finished traversal, got %d entries and cursor %v", len(page.Items), page.NextCursor)
	}
	return nil
}

func equalIDs(what string, want, got []string) error {
	if !slices.Equal(want, got) {
		return fmt.Errorf("%s: want %s, got %s", what, strconv.Quote(strings.Join(want, ",")), strconv.Quote(strings.Join(got, ",")))
	}
	return nil
}
