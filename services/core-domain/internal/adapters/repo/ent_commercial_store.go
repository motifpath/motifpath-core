package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/commercialauditentry"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/commercialgroupmember"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/commercialoperation"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/commercialoperationattempt"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/predicate"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/user"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// commercialActivationRowID is the single activation row the migration inserts.
const commercialActivationRowID = 1

// commercialAuditAppendLock is the transaction advisory lock every audit
// append takes. Order keys come from an identity, which is allocated at insert
// but becomes visible at commit; without this lock a later key could commit
// first and an earlier one would then slip into a traversal already past it.
const commercialAuditAppendLock int64 = 0x636f6d6d61756469 // "commaudi"

// EntCommercialStore keeps commercial activation, operations and evidence in
// Postgres, committing each command's changes and evidence in one transaction.
type EntCommercialStore struct {
	client *ent.Client
}

func NewEntCommercialStore(client *ent.Client) *EntCommercialStore {
	return &EntCommercialStore{client: client}
}

var _ ports.CommercialStore = (*EntCommercialStore)(nil)

func (s *EntCommercialStore) InTx(ctx context.Context, fn func(ctx context.Context, tx ports.CommercialTx) error) error {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	if err := fn(ctx, entCommercialTx{tx: tx}); err != nil {
		return rollback(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit commercial change: %w", err)
	}
	return nil
}

// ReadActivation reads revision, flags and groups in one transaction, so they
// describe the same moment.
func (s *EntCommercialStore) ReadActivation(ctx context.Context) (domain.CommercialActivation, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return domain.CommercialActivation{}, err
	}
	activation, err := readActivation(ctx, tx)
	if err != nil {
		return domain.CommercialActivation{}, rollback(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return domain.CommercialActivation{}, err
	}
	return activation, nil
}

func (s *EntCommercialStore) Audit() ports.CommercialAuditReader {
	return entCommercialAuditReader{client: s.client}
}

type entCommercialTx struct {
	tx *ent.Tx
}

func (t entCommercialTx) Activation() ports.CommercialActivationTx {
	return entCommercialActivationTx(t)
}

func (t entCommercialTx) Groups() ports.CommercialGroupsTx {
	return entCommercialGroupsTx(t)
}

func (t entCommercialTx) Operations() ports.CommercialOperationsTx {
	return entCommercialOperationsTx(t)
}

func (t entCommercialTx) Audit() ports.CommercialAuditWriter {
	return entCommercialAuditWriter(t)
}

// ── Activation ───────────────────────────────────────────────────────────────

type entCommercialActivationTx struct {
	tx *ent.Tx
}

func (a entCommercialActivationTx) LockShared(ctx context.Context) (int64, error) {
	return a.lockedRevision(ctx, "SELECT revision FROM commercial_activations WHERE id = $1 FOR SHARE")
}

func (a entCommercialActivationTx) LockExclusive(ctx context.Context) (int64, error) {
	return a.lockedRevision(ctx, "SELECT revision FROM commercial_activations WHERE id = $1 FOR UPDATE")
}

func (a entCommercialActivationTx) lockedRevision(ctx context.Context, query string) (int64, error) {
	rows, err := a.tx.QueryContext(ctx, query, commercialActivationRowID)
	if err != nil {
		return 0, err
	}
	return scanSingleInt64(rows, "commercial activation row")
}

func (a entCommercialActivationTx) Read(ctx context.Context) (domain.CommercialActivation, error) {
	return readActivation(ctx, a.tx)
}

func (a entCommercialActivationTx) SaveFlag(ctx context.Context, flag domain.FlagName, setting domain.FlagSetting, at time.Time, by string) error {
	updatedBy, err := uuid.Parse(by)
	if err != nil {
		return fmt.Errorf("flag changed by %q: %w", by, err)
	}
	subjectIDs := make([]string, len(setting.Audience.Entries))
	for i, subject := range setting.Audience.Entries {
		subjectIDs[i] = subject.String()
	}
	err = a.tx.CommercialFlag.UpdateOneID(string(flag)).
		SetEnabled(setting.Enabled).
		SetAllUsers(setting.Audience.AllUsers).
		SetSubjectIds(subjectIDs).
		SetUpdatedAt(at).
		SetUpdatedBy(updatedBy).
		Exec(ctx)
	if ent.IsNotFound(err) {
		return fmt.Errorf("flag %s has no row: %w", flag, domain.ErrNotFound)
	}
	return err
}

func (a entCommercialActivationTx) BumpRevision(ctx context.Context) (int64, error) {
	rows, err := a.tx.QueryContext(ctx,
		"UPDATE commercial_activations SET revision = revision + 1 WHERE id = $1 RETURNING revision",
		commercialActivationRowID)
	if err != nil {
		return 0, err
	}
	return scanSingleInt64(rows, "commercial activation row")
}

func readActivation(ctx context.Context, tx *ent.Tx) (domain.CommercialActivation, error) {
	row, err := tx.CommercialActivation.Get(ctx, commercialActivationRowID)
	if err != nil {
		return domain.CommercialActivation{}, fmt.Errorf("read commercial activation row: %w", err)
	}
	flagRows, err := tx.CommercialFlag.Query().All(ctx)
	if err != nil {
		return domain.CommercialActivation{}, err
	}
	flags := make(map[domain.FlagName]domain.FlagSetting, len(flagRows))
	for _, flagRow := range flagRows {
		setting, err := flagSetting(flagRow)
		if err != nil {
			return domain.CommercialActivation{}, err
		}
		flags[domain.FlagName(flagRow.ID)] = setting
	}
	members, err := tx.CommercialGroupMember.Query().All(ctx)
	if err != nil {
		return domain.CommercialActivation{}, err
	}
	groups := map[string][]string{}
	for _, member := range members {
		groups[member.GroupSlug] = append(groups[member.GroupSlug], member.UserID.String())
	}
	return domain.NewCommercialActivation(row.Revision, flags, groups), nil
}

// flagSetting maps a stored flag. An entry that no longer parses is an error,
// never silently dropped: dropping one could change who a flag allows.
func flagSetting(row *ent.CommercialFlag) (domain.FlagSetting, error) {
	entries := make([]domain.RolloutSubject, len(row.SubjectIds))
	for i, value := range row.SubjectIds {
		subject, err := domain.ParseRolloutSubject(value)
		if err != nil {
			return domain.FlagSetting{}, fmt.Errorf("stored flag %s: %w", row.ID, err)
		}
		entries[i] = subject
	}
	return domain.FlagSetting{
		Enabled:  row.Enabled,
		Audience: domain.RolloutAudience{AllUsers: row.AllUsers, Entries: entries},
	}, nil
}

// ── Groups ───────────────────────────────────────────────────────────────────

type entCommercialGroupsTx struct {
	tx *ent.Tx
}

func (g entCommercialGroupsTx) List(ctx context.Context) ([]domain.CommercialGroup, error) {
	rows, err := g.tx.CommercialGroup.Query().Order(ent.Asc("slug")).All(ctx)
	if err != nil {
		return nil, err
	}
	members, err := g.tx.CommercialGroupMember.Query().
		Order(ent.Asc(commercialgroupmember.FieldUserID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	bySlug := map[string][]string{}
	for _, member := range members {
		bySlug[member.GroupSlug] = append(bySlug[member.GroupSlug], member.UserID.String())
	}
	groups := make([]domain.CommercialGroup, len(rows))
	for i, row := range rows {
		groups[i] = commercialGroup(row, bySlug[row.ID])
	}
	return groups, nil
}

func (g entCommercialGroupsTx) Find(ctx context.Context, slug string) (domain.CommercialGroup, error) {
	row, err := g.tx.CommercialGroup.Get(ctx, slug)
	if ent.IsNotFound(err) {
		return domain.CommercialGroup{}, fmt.Errorf("commercial group %q: %w", slug, domain.ErrNotFound)
	}
	if err != nil {
		return domain.CommercialGroup{}, err
	}
	members, err := g.tx.CommercialGroupMember.Query().
		Where(commercialgroupmember.GroupSlug(slug)).
		Order(ent.Asc(commercialgroupmember.FieldUserID)).
		All(ctx)
	if err != nil {
		return domain.CommercialGroup{}, err
	}
	memberIDs := make([]string, len(members))
	for i, member := range members {
		memberIDs[i] = member.UserID.String()
	}
	return commercialGroup(row, memberIDs), nil
}

func (g entCommercialGroupsTx) Save(ctx context.Context, slug, name string, at time.Time, by string) error {
	updatedBy, err := uuid.Parse(by)
	if err != nil {
		return fmt.Errorf("group changed by %q: %w", by, err)
	}
	err = g.tx.CommercialGroup.UpdateOneID(slug).
		SetName(name).
		SetUpdatedAt(at).
		SetUpdatedBy(updatedBy).
		Exec(ctx)
	if !ent.IsNotFound(err) {
		return err
	}
	return g.tx.CommercialGroup.Create().
		SetID(slug).
		SetName(name).
		SetUpdatedAt(at).
		SetUpdatedBy(updatedBy).
		Exec(ctx)
}

func (g entCommercialGroupsTx) SetMembers(ctx context.Context, slug string, memberIDs []string, at time.Time, by string) error {
	updatedBy, err := uuid.Parse(by)
	if err != nil {
		return fmt.Errorf("group changed by %q: %w", by, err)
	}
	if _, err := g.tx.CommercialGroupMember.Delete().Where(commercialgroupmember.GroupSlug(slug)).Exec(ctx); err != nil {
		return err
	}
	creates := make([]*ent.CommercialGroupMemberCreate, len(memberIDs))
	for i, memberID := range memberIDs {
		userID, err := uuid.Parse(memberID)
		if err != nil {
			return fmt.Errorf("group member %q: %w", memberID, err)
		}
		creates[i] = g.tx.CommercialGroupMember.Create().SetGroupSlug(slug).SetUserID(userID)
	}
	if err := g.tx.CommercialGroupMember.CreateBulk(creates...).Exec(ctx); err != nil {
		return err
	}
	return g.tx.CommercialGroup.UpdateOneID(slug).SetUpdatedAt(at).SetUpdatedBy(updatedBy).Exec(ctx)
}

func (g entCommercialGroupsTx) MissingUsers(ctx context.Context, userIDs []string) ([]string, error) {
	ids := make([]uuid.UUID, 0, len(userIDs))
	for _, userID := range userIDs {
		id, err := uuid.Parse(userID)
		if err != nil {
			return nil, fmt.Errorf("user %q: %w", userID, err)
		}
		ids = append(ids, id)
	}
	existing, err := g.tx.User.Query().Where(user.IDIn(ids...)).IDs(ctx)
	if err != nil {
		return nil, err
	}
	found := make(map[string]bool, len(existing))
	for _, id := range existing {
		found[id.String()] = true
	}
	missing := []string{}
	for _, userID := range userIDs {
		if !found[userID] {
			missing = append(missing, userID)
		}
	}
	return missing, nil
}

func commercialGroup(row *ent.CommercialGroup, memberIDs []string) domain.CommercialGroup {
	if memberIDs == nil {
		memberIDs = []string{}
	}
	return domain.CommercialGroup{
		Slug: row.ID, Name: row.Name, MemberIDs: memberIDs, UpdatedAt: row.UpdatedAt, UpdatedBy: row.UpdatedBy.String(),
	}
}

// ── Operations ───────────────────────────────────────────────────────────────

type entCommercialOperationsTx struct {
	tx *ent.Tx
}

func (o entCommercialOperationsTx) Find(ctx context.Context, key domain.OperationKey) (domain.CommercialOperation, error) {
	row, err := o.find(ctx, key)
	if err != nil {
		return domain.CommercialOperation{}, err
	}
	return domain.CommercialOperation{
		Key: key, Fingerprint: row.Fingerprint, Result: row.Result,
		AuthorizedRevision: row.AuthorizedRevision, CreatedAt: row.CreatedAt,
	}, nil
}

func (o entCommercialOperationsTx) find(ctx context.Context, key domain.OperationKey) (*ent.CommercialOperation, error) {
	match, err := operationKeyPredicate(key)
	if err != nil {
		return nil, err
	}
	row, err := o.tx.CommercialOperation.Query().Where(match).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, fmt.Errorf("commercial operation: %w", domain.ErrNotFound)
	}
	return row, err
}

// Insert records a new operation. Postgres aborts the transaction on the
// unique violation, so a caller that gets domain.ErrConflict must read the
// winner's record in a new transaction.
func (o entCommercialOperationsTx) Insert(ctx context.Context, operation domain.CommercialOperation) error {
	callerKey, err := uuid.Parse(operation.Key.CallerKey)
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrInvalidOperationKey, err)
	}
	err = o.tx.CommercialOperation.Create().
		SetEnvironment(operation.Key.Environment.String()).
		SetActorKind(commercialoperation.ActorKind(operation.Key.ActorKind)).
		SetActorID(operation.Key.ActorID).
		SetAction(operation.Key.Action).
		SetResourceKind(operation.Key.ResourceKind).
		SetResourceID(operation.Key.ResourceID).
		SetCallerKey(callerKey).
		SetFingerprint(operation.Fingerprint).
		SetResult(operation.Result).
		SetAuthorizedRevision(operation.AuthorizedRevision).
		SetCreatedAt(operation.CreatedAt).
		Exec(ctx)
	if ent.IsConstraintError(err) {
		return fmt.Errorf("commercial operation already recorded: %w", domain.ErrConflict)
	}
	return err
}

func (o entCommercialOperationsTx) RecordAttempt(ctx context.Context, attempt domain.OperationAttempt) error {
	operation, err := o.find(ctx, attempt.Key)
	if err != nil {
		return err
	}
	attemptID, err := uuid.Parse(attempt.AttemptID)
	if err != nil {
		return fmt.Errorf("attempt id %q: %w", attempt.AttemptID, err)
	}
	return o.tx.CommercialOperationAttempt.Create().
		SetID(attemptID).
		SetOperationID(operation.ID).
		SetFingerprint(attempt.Fingerprint).
		SetOutcome(commercialOperationAttemptOutcome(attempt.Outcome)).
		SetAt(attempt.At).
		Exec(ctx)
}

func operationKeyPredicate(key domain.OperationKey) (predicate.CommercialOperation, error) {
	callerKey, err := uuid.Parse(key.CallerKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidOperationKey, err)
	}
	return commercialoperation.And(
		commercialoperation.Environment(key.Environment.String()),
		commercialoperation.ActorKindEQ(commercialoperation.ActorKind(key.ActorKind)),
		commercialoperation.ActorID(key.ActorID),
		commercialoperation.Action(key.Action),
		commercialoperation.ResourceKind(key.ResourceKind),
		commercialoperation.ResourceID(key.ResourceID),
		commercialoperation.CallerKey(callerKey),
	), nil
}

// ── Audit ────────────────────────────────────────────────────────────────────

type entCommercialAuditWriter struct {
	tx *ent.Tx
}

func (w entCommercialAuditWriter) Append(ctx context.Context, entry domain.AuditEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	if _, err := w.tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", commercialAuditAppendLock); err != nil {
		return fmt.Errorf("serialize audit append: %w", err)
	}
	ids, err := auditEntryIDs(entry)
	if err != nil {
		return err
	}
	create := w.tx.CommercialAuditEntry.Create().
		SetAuditEntryID(ids.entry).
		SetEnvironment(entry.Operation.Environment.String()).
		SetOperationID(ids.operation).
		SetAttemptID(ids.attempt).
		SetActorKind(commercialauditentry.ActorKind(entry.Operation.ActorKind)).
		SetActorID(entry.Operation.ActorID).
		SetAction(entry.Operation.Action).
		SetResourceKind(entry.Operation.ResourceKind).
		SetResourceID(entry.Operation.ResourceID).
		SetOccurredAt(entry.OccurredAt).
		SetRecordedAt(entry.RecordedAt).
		SetReason(entry.Reason).
		SetOutcome(commercialauditentry.Outcome(entry.Outcome)).
		SetPolicyRevision(entry.PolicyRevision).
		SetConfigurationRevision(entry.ConfigurationRevision).
		SetCorrelationID(ids.correlation).
		SetCausedByEntryIds(nonNilStrings(entry.CausedByEntryIDs)).
		SetProviderReferences(nonNilStrings(entry.ProviderReferences))
	// A state is only stored when one was available, so "no authoritative
	// state" stays SQL NULL rather than becoming a JSON null.
	if entry.BeforeState != nil {
		create.SetBeforeState(entry.BeforeState)
	}
	if entry.AfterState != nil {
		create.SetAfterState(entry.AfterState)
	}
	return create.Exec(ctx)
}

type auditUUIDs struct {
	entry, operation, attempt, correlation uuid.UUID
}

func auditEntryIDs(entry domain.AuditEntry) (auditUUIDs, error) {
	var ids auditUUIDs
	for _, field := range []struct {
		target *uuid.UUID
		value  string
	}{
		{&ids.entry, entry.AuditEntryID},
		{&ids.operation, entry.Operation.OperationID()},
		{&ids.attempt, entry.AttemptID},
		{&ids.correlation, entry.CorrelationID},
	} {
		parsed, err := uuid.Parse(field.value)
		if err != nil {
			return auditUUIDs{}, fmt.Errorf("%w: %w", domain.ErrInvalidAuditEntry, err)
		}
		*field.target = parsed
	}
	return ids, nil
}

type entCommercialAuditReader struct {
	client *ent.Client
}

func (r entCommercialAuditReader) LastOrderKey(ctx context.Context, environment domain.Environment) (int64, error) {
	last, err := r.client.CommercialAuditEntry.Query().
		Where(commercialauditentry.Environment(environment.String())).
		Order(ent.Desc(commercialauditentry.FieldID)).
		FirstID(ctx)
	if ent.IsNotFound(err) {
		return 0, nil
	}
	return last, err
}

func (r entCommercialAuditReader) List(ctx context.Context, environment domain.Environment, filter ports.AuditFilter, afterKey, snapshotKey int64, limit int) ([]ports.AuditRecord, error) {
	match, err := auditFilterPredicate(filter)
	if err != nil {
		return nil, err
	}
	rows, err := r.client.CommercialAuditEntry.Query().
		Where(
			commercialauditentry.Environment(environment.String()),
			commercialauditentry.IDGT(afterKey),
			commercialauditentry.IDLTE(snapshotKey),
			match,
		).
		Order(ent.Asc(commercialauditentry.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]ports.AuditRecord, len(rows))
	for i, row := range rows {
		entry, err := auditEntry(row)
		if err != nil {
			return nil, err
		}
		records[i] = ports.AuditRecord{OrderKey: row.ID, Entry: entry}
	}
	return records, nil
}

func auditFilterPredicate(filter ports.AuditFilter) (predicate.CommercialAuditEntry, error) {
	if filter.OperationID != "" {
		operationID, err := uuid.Parse(filter.OperationID)
		if err != nil {
			return nil, fmt.Errorf("audit filter operation id: %w", err)
		}
		return commercialauditentry.OperationID(operationID), nil
	}
	if filter.ResourceKind == "" || filter.ResourceID == "" {
		return nil, errors.New("audit filter needs an operation id or a resource kind and id")
	}
	return commercialauditentry.And(
		commercialauditentry.ResourceKind(filter.ResourceKind),
		commercialauditentry.ResourceID(filter.ResourceID),
	), nil
}

func auditEntry(row *ent.CommercialAuditEntry) (domain.AuditEntry, error) {
	environment, err := domain.ParseEnvironment(row.Environment)
	if err != nil {
		return domain.AuditEntry{}, fmt.Errorf("stored audit entry %s: %w", row.AuditEntryID, err)
	}
	return domain.AuditEntry{
		AuditEntryID: row.AuditEntryID.String(),
		Operation: domain.OperationKey{
			Environment: environment, ActorKind: domain.AuditActorKind(row.ActorKind), ActorID: row.ActorID,
			Action: row.Action, ResourceKind: row.ResourceKind, ResourceID: row.ResourceID,
			CallerKey: row.OperationID.String(),
		},
		AttemptID:             row.AttemptID.String(),
		OccurredAt:            row.OccurredAt,
		RecordedAt:            row.RecordedAt,
		Reason:                row.Reason,
		Outcome:               domain.AuditOutcome(row.Outcome),
		BeforeState:           storedState(row.BeforeState),
		AfterState:            storedState(row.AfterState),
		PolicyRevision:        row.PolicyRevision,
		ConfigurationRevision: row.ConfigurationRevision,
		CorrelationID:         row.CorrelationID.String(),
		CausedByEntryIDs:      nonNilStrings(row.CausedByEntryIds),
		ProviderReferences:    nonNilStrings(row.ProviderReferences),
	}, nil
}

// storedState maps SQL NULL (and a JSON null, which is never written) to nil.
func storedState(state json.RawMessage) json.RawMessage {
	if len(state) == 0 || string(state) == "null" {
		return nil
	}
	return state
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func commercialOperationAttemptOutcome(outcome domain.AuditOutcome) commercialoperationattempt.Outcome {
	return commercialoperationattempt.Outcome(outcome)
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func scanSingleInt64(rows *sql.Rows, what string) (int64, error) {
	defer rows.Close() //nolint:errcheck // rows.Err below reports any failure that matters
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("%s is missing", what)
	}
	var value int64
	if err := rows.Scan(&value); err != nil {
		return 0, err
	}
	return value, rows.Err()
}
