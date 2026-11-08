package application_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

var errFakeStorage = errors.New("fake storage unavailable")

// fakeCommercialState is everything the fake store holds. A transaction works
// on a copy and replaces the committed state only when it succeeds, so the
// fake has the same all-or-nothing behaviour as Postgres.
type fakeCommercialState struct {
	revision   int64
	flags      map[domain.FlagName]domain.FlagSetting
	flagMeta   map[domain.FlagName]fakeFlagMeta
	groups     map[string]domain.CommercialGroup
	operations []domain.CommercialOperation
	attempts   []domain.OperationAttempt
	audit      []ports.AuditRecord
	users      map[string]bool
}

type fakeFlagMeta struct {
	at time.Time
	by string
}

func (s fakeCommercialState) clone() fakeCommercialState {
	c := s
	c.flags = make(map[domain.FlagName]domain.FlagSetting, len(s.flags))
	for k, v := range s.flags {
		c.flags[k] = v
	}
	c.flagMeta = make(map[domain.FlagName]fakeFlagMeta, len(s.flagMeta))
	for k, v := range s.flagMeta {
		c.flagMeta[k] = v
	}
	c.groups = make(map[string]domain.CommercialGroup, len(s.groups))
	for k, v := range s.groups {
		v.MemberIDs = append([]string{}, v.MemberIDs...)
		c.groups[k] = v
	}
	c.operations = append([]domain.CommercialOperation{}, s.operations...)
	c.attempts = append([]domain.OperationAttempt{}, s.attempts...)
	c.audit = append([]ports.AuditRecord{}, s.audit...)
	c.users = make(map[string]bool, len(s.users))
	for k, v := range s.users {
		c.users[k] = v
	}
	return c
}

type fakeCommercialStore struct {
	state fakeCommercialState

	// Failure switches for the scenarios that need them.
	auditFails   bool
	storageFails bool
	// onInsert runs before an operation insert, to simulate a concurrent
	// submission committing the same key first.
	onInsert func(state *fakeCommercialState)
}

func newFakeCommercialStore() *fakeCommercialStore {
	return &fakeCommercialStore{state: fakeCommercialState{
		flags: map[domain.FlagName]domain.FlagSetting{}, flagMeta: map[domain.FlagName]fakeFlagMeta{},
		groups: map[string]domain.CommercialGroup{}, users: map[string]bool{},
	}}
}

func (s *fakeCommercialStore) InTx(ctx context.Context, fn func(ctx context.Context, tx ports.CommercialTx) error) error {
	if s.storageFails {
		return errFakeStorage
	}
	working := s.state.clone()
	if err := fn(ctx, &fakeCommercialTx{store: s, state: &working}); err != nil {
		return err
	}
	s.state = working
	return nil
}

func (s *fakeCommercialStore) ReadActivation(context.Context) (domain.CommercialActivation, error) {
	if s.storageFails {
		return domain.CommercialActivation{}, errFakeStorage
	}
	return activationOf(s.state), nil
}

func (s *fakeCommercialStore) Audit() ports.CommercialAuditReader {
	return fakeCommercialAuditReader{store: s}
}

func activationOf(state fakeCommercialState) domain.CommercialActivation {
	groups := make(map[string][]string, len(state.groups))
	for slug, group := range state.groups {
		groups[slug] = group.MemberIDs
	}
	return domain.NewCommercialActivation(state.revision, state.flags, groups)
}

type fakeCommercialTx struct {
	store *fakeCommercialStore
	state *fakeCommercialState
}

func (t *fakeCommercialTx) Activation() ports.CommercialActivationTx { return fakeActivationTx(*t) }
func (t *fakeCommercialTx) Groups() ports.CommercialGroupsTx         { return fakeGroupsTx(*t) }
func (t *fakeCommercialTx) Operations() ports.CommercialOperationsTx { return fakeOperationsTx(*t) }
func (t *fakeCommercialTx) Audit() ports.CommercialAuditWriter       { return fakeAuditWriter(*t) }

type fakeActivationTx fakeCommercialTx

func (a fakeActivationTx) LockShared(context.Context) (int64, error)    { return a.state.revision, nil }
func (a fakeActivationTx) LockExclusive(context.Context) (int64, error) { return a.state.revision, nil }
func (a fakeActivationTx) Read(context.Context) (domain.CommercialActivation, error) {
	return activationOf(*a.state), nil
}

func (a fakeActivationTx) FlagChanges(context.Context) (map[domain.FlagName]ports.FlagChange, error) {
	changes := make(map[domain.FlagName]ports.FlagChange, len(a.state.flagMeta))
	for flag, meta := range a.state.flagMeta {
		changes[flag] = ports.FlagChange{At: meta.at, By: meta.by}
	}
	return changes, nil
}

func (a fakeActivationTx) SaveFlag(_ context.Context, flag domain.FlagName, setting domain.FlagSetting, at time.Time, by string) error {
	a.state.flags[flag] = setting
	a.state.flagMeta[flag] = fakeFlagMeta{at: at, by: by}
	return nil
}

func (a fakeActivationTx) BumpRevision(context.Context) (int64, error) {
	a.state.revision++
	return a.state.revision, nil
}

type fakeGroupsTx fakeCommercialTx

func (g fakeGroupsTx) List(context.Context) ([]domain.CommercialGroup, error) {
	slugs := make([]string, 0, len(g.state.groups))
	for slug := range g.state.groups {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	groups := make([]domain.CommercialGroup, len(slugs))
	for i, slug := range slugs {
		groups[i] = g.state.groups[slug]
	}
	return groups, nil
}

func (g fakeGroupsTx) Find(_ context.Context, slug string) (domain.CommercialGroup, error) {
	group, ok := g.state.groups[slug]
	if !ok {
		return domain.CommercialGroup{}, fmt.Errorf("group %q: %w", slug, domain.ErrNotFound)
	}
	return group, nil
}

func (g fakeGroupsTx) Save(_ context.Context, slug, name string, at time.Time, by string) error {
	group, ok := g.state.groups[slug]
	if !ok {
		group = domain.CommercialGroup{Slug: slug, MemberIDs: []string{}}
	}
	group.Name, group.UpdatedAt, group.UpdatedBy = name, at, by
	g.state.groups[slug] = group
	return nil
}

func (g fakeGroupsTx) SetMembers(_ context.Context, slug string, memberIDs []string, at time.Time, by string) error {
	group, ok := g.state.groups[slug]
	if !ok {
		return fmt.Errorf("group %q: %w", slug, domain.ErrNotFound)
	}
	group.MemberIDs = append([]string{}, memberIDs...)
	group.UpdatedAt, group.UpdatedBy = at, by
	g.state.groups[slug] = group
	return nil
}

func (g fakeGroupsTx) MissingUsers(_ context.Context, userIDs []string) ([]string, error) {
	missing := []string{}
	for _, userID := range userIDs {
		if !g.state.users[userID] {
			missing = append(missing, userID)
		}
	}
	return missing, nil
}

type fakeOperationsTx fakeCommercialTx

func (o fakeOperationsTx) Find(_ context.Context, key domain.OperationKey) (domain.CommercialOperation, error) {
	for _, operation := range o.state.operations {
		if operation.Key == key {
			return operation, nil
		}
	}
	return domain.CommercialOperation{}, fmt.Errorf("operation: %w", domain.ErrNotFound)
}

func (o fakeOperationsTx) Insert(_ context.Context, operation domain.CommercialOperation) error {
	if o.store.onInsert != nil {
		race := o.store.onInsert
		o.store.onInsert = nil
		race(&o.store.state)
	}
	for _, existing := range append(o.state.operations, o.store.state.operations...) {
		if existing.Key == operation.Key {
			return fmt.Errorf("operation already recorded: %w", domain.ErrConflict)
		}
	}
	o.state.operations = append(o.state.operations, operation)
	return nil
}

func (o fakeOperationsTx) RecordAttempt(_ context.Context, attempt domain.OperationAttempt) error {
	o.state.attempts = append(o.state.attempts, attempt)
	return nil
}

type fakeAuditWriter fakeCommercialTx

func (w fakeAuditWriter) Append(_ context.Context, entry domain.AuditEntry) error {
	if w.store.auditFails {
		return errFakeStorage
	}
	if err := entry.Validate(); err != nil {
		return err
	}
	w.state.audit = append(w.state.audit, ports.AuditRecord{OrderKey: int64(len(w.state.audit) + 1), Entry: entry})
	return nil
}

type fakeCommercialAuditReader struct {
	store *fakeCommercialStore
}

func (r fakeCommercialAuditReader) LastOrderKey(_ context.Context, environment domain.Environment) (int64, error) {
	if r.store.storageFails {
		return 0, errFakeStorage
	}
	var last int64
	for _, record := range r.store.state.audit {
		if record.Entry.Operation.Environment == environment {
			last = record.OrderKey
		}
	}
	return last, nil
}

func (r fakeCommercialAuditReader) List(_ context.Context, environment domain.Environment, filter ports.AuditFilter, afterKey, snapshotKey int64, limit int) ([]ports.AuditRecord, error) {
	if r.store.storageFails {
		return nil, errFakeStorage
	}
	records := []ports.AuditRecord{}
	for _, record := range r.store.state.audit {
		entry := record.Entry
		matches := entry.Operation.Environment == environment && record.OrderKey > afterKey && record.OrderKey <= snapshotKey
		if filter.OperationID != "" {
			matches = matches && entry.Operation.OperationID() == filter.OperationID
		}
		if filter.ResourceKind != "" {
			matches = matches && entry.Operation.ResourceKind == filter.ResourceKind && entry.Operation.ResourceID == filter.ResourceID
		}
		if matches && len(records) < limit {
			records = append(records, record)
		}
	}
	return records, nil
}

// auditOutcomes lists the outcome of every evidence entry, in append order.
func (s *fakeCommercialStore) auditOutcomes() []domain.AuditOutcome {
	outcomes := make([]domain.AuditOutcome, len(s.state.audit))
	for i, record := range s.state.audit {
		outcomes[i] = record.Entry.Outcome
	}
	return outcomes
}

// attemptOutcomes lists the outcome of every recorded attempt, in order.
func (s *fakeCommercialStore) attemptOutcomes() []domain.AuditOutcome {
	outcomes := make([]domain.AuditOutcome, len(s.state.attempts))
	for i, attempt := range s.state.attempts {
		outcomes[i] = attempt.Outcome
	}
	return outcomes
}
