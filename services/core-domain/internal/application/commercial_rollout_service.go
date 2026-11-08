package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

const (
	maxRolloutSubjects    = 500
	maxRolloutReasonChars = 1000
	maxMembershipChange   = 500

	rolloutIntentVersion = 1

	actionFlagChanged         = "commercial_rollout.flag_changed"
	actionKillSwitch          = "commercial_rollout.kill_switch"
	actionGroupSaved          = "commercial_rollout.group_saved"
	actionGroupMembersChanged = "commercial_rollout.group_members_changed"

	resourceFlag    = "commercial_flag"
	resourceGroup   = "commercial_group"
	resourceRollout = "commercial_rollout"
	rolloutAll      = "all"
)

// FlagState is one flag's configured and effective activation.
type FlagState struct {
	Flag          domain.FlagName
	Enabled       bool
	AllUsers      bool
	SubjectIDs    []string
	Prerequisites []domain.FlagName
	// Effective is true when the flag and every prerequisite are enabled;
	// the audience still decides who it allows.
	Effective bool
	UpdatedAt *time.Time
	UpdatedBy *string
}

// CommercialRollout is every registered flag at one activation revision.
type CommercialRollout struct {
	Revision int64
	Flags    []FlagState
}

// CommercialGroups is every rollout group at one activation revision.
type CommercialGroups struct {
	Revision int64
	Groups   []domain.CommercialGroup
}

// FlagChangeRequest replaces one flag's switch and audience.
type FlagChangeRequest struct {
	ExpectedRevision int64    `json:"expected_revision"`
	Enabled          bool     `json:"enabled"`
	AllUsers         bool     `json:"all_users"`
	SubjectIDs       []string `json:"subject_ids"`
	Reason           string   `json:"reason"`
}

// GroupChangeRequest creates or renames a group.
type GroupChangeRequest struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// MembershipChangeRequest adds and removes group members in one change.
type MembershipChangeRequest struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
	Reason string   `json:"reason"`
}

// RolloutReceipt is the durable receipt of an applied activation change.
type RolloutReceipt struct {
	OperationID     string `json:"operation_id"`
	Status          string `json:"status"`
	Revision        int64  `json:"revision"`
	EvidenceEntryID string `json:"evidence_entry_id"`
}

// CommercialRolloutService administers commercial activation and answers
// whether a flag allows a subject. Every change is idempotent, bumps the
// activation revision and commits with its evidence.
type CommercialRolloutService struct {
	store       ports.CommercialStore
	commands    *CommercialCommands
	environment domain.Environment
	now         func() time.Time
}

func NewCommercialRolloutService(store ports.CommercialStore, commands *CommercialCommands, environment domain.Environment, now func() time.Time) *CommercialRolloutService {
	return &CommercialRolloutService{store: store, commands: commands, environment: environment, now: now}
}

// ── Reads ────────────────────────────────────────────────────────────────────

// Get returns every registered flag in registry order. An unreadable store
// is domain.ErrUnavailable, never flags reported as OFF.
func (s *CommercialRolloutService) Get(ctx context.Context, caller domain.User) (CommercialRollout, error) {
	if caller.Role != domain.RoleAdmin {
		return CommercialRollout{}, domain.ErrForbidden
	}
	var rollout CommercialRollout
	err := s.store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		activation, err := tx.Activation().Read(ctx)
		if err != nil {
			return err
		}
		changes, err := tx.Activation().FlagChanges(ctx)
		if err != nil {
			return err
		}
		rollout = rolloutView(activation, changes)
		return nil
	})
	if err != nil {
		return CommercialRollout{}, fmt.Errorf("%w: read commercial activation: %w", domain.ErrUnavailable, err)
	}
	return rollout, nil
}

func rolloutView(activation domain.CommercialActivation, changes map[domain.FlagName]ports.FlagChange) CommercialRollout {
	flags := domain.CommercialFlags()
	rollout := CommercialRollout{Revision: activation.Revision(), Flags: make([]FlagState, len(flags))}
	for i, flag := range flags {
		setting := activation.Setting(flag)
		state := FlagState{
			Flag: flag, Enabled: setting.Enabled, AllUsers: setting.Audience.AllUsers,
			SubjectIDs: subjectStrings(setting.Audience.Entries), Prerequisites: nonNilFlags(flag.Prerequisites()),
			Effective: activation.Effective(flag),
		}
		if change, ok := changes[flag]; ok {
			at, by := change.At, change.By
			state.UpdatedAt, state.UpdatedBy = &at, &by
		}
		rollout.Flags[i] = state
	}
	return rollout
}

// ListGroups returns every group ordered by slug, with the activation revision.
func (s *CommercialRolloutService) ListGroups(ctx context.Context, caller domain.User) (CommercialGroups, error) {
	if caller.Role != domain.RoleAdmin {
		return CommercialGroups{}, domain.ErrForbidden
	}
	var result CommercialGroups
	err := s.store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		activation, err := tx.Activation().Read(ctx)
		if err != nil {
			return err
		}
		groups, err := tx.Groups().List(ctx)
		if err != nil {
			return err
		}
		result = CommercialGroups{Revision: activation.Revision(), Groups: groups}
		return nil
	})
	if err != nil {
		return CommercialGroups{}, fmt.Errorf("%w: read commercial groups: %w", domain.ErrUnavailable, err)
	}
	return result, nil
}

// Allowed reports whether a flag allows a user or service. When activation
// cannot be read it denies and returns domain.ErrUnavailable.
func (s *CommercialRolloutService) Allowed(ctx context.Context, flag domain.FlagName, subject domain.RolloutSubject) (bool, error) {
	activation, err := s.store.ReadActivation(ctx)
	if err != nil {
		return false, fmt.Errorf("%w: read commercial activation: %w", domain.ErrUnavailable, err)
	}
	return activation.Allows(flag, subject), nil
}

// AuthorizeIntent authorizes a new commercial intent inside the caller's
// transaction. It holds the shared activation lock until that transaction
// ends, so an OFF change committing meanwhile is either seen or waited for.
// It returns the revision the intent must record, or domain.ErrForbidden.
func (s *CommercialRolloutService) AuthorizeIntent(ctx context.Context, tx ports.CommercialTx, flag domain.FlagName, subject domain.RolloutSubject) (int64, error) {
	revision, err := tx.Activation().LockShared(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: lock commercial activation: %w", domain.ErrUnavailable, err)
	}
	activation, err := tx.Activation().Read(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: read commercial activation: %w", domain.ErrUnavailable, err)
	}
	if !activation.Allows(flag, subject) {
		return 0, fmt.Errorf("%w: %s does not allow %s", domain.ErrForbidden, flag, subject)
	}
	return revision, nil
}

// ── Changes ──────────────────────────────────────────────────────────────────

// ChangeFlag replaces one flag's switch and audience at the expected
// revision. A replay of the same change returns its original receipt even
// once the revision has moved on.
func (s *CommercialRolloutService) ChangeFlag(ctx context.Context, caller domain.User, flagName, idempotencyKey string, change FlagChangeRequest) (RolloutReceipt, error) {
	intent, err := s.intent(caller, actionFlagChanged, resourceFlag, flagName, idempotencyKey, change.Reason, change)
	if err != nil {
		return RolloutReceipt{}, err
	}
	return s.run(ctx, caller, intent, func(ctx context.Context, tx ports.CommercialTx) (json.RawMessage, json.RawMessage, error) {
		flag, err := domain.ParseFlagName(flagName)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %w", domain.ErrNotFound, err)
		}
		if err := validateReason(change.Reason); err != nil {
			return nil, nil, err
		}
		subjects, err := parseAllowlist(ctx, tx, change.SubjectIDs)
		if err != nil {
			return nil, nil, err
		}
		revision, err := tx.Activation().LockExclusive(ctx)
		if err != nil {
			return nil, nil, err
		}
		if revision != change.ExpectedRevision {
			return nil, nil, fmt.Errorf("%w: activation is at revision %d, not %d", domain.ErrConflict, revision, change.ExpectedRevision)
		}
		activation, err := tx.Activation().Read(ctx)
		if err != nil {
			return nil, nil, err
		}
		next := domain.FlagSetting{Enabled: change.Enabled, Audience: domain.RolloutAudience{AllUsers: change.AllUsers, Entries: subjects}}
		if err := tx.Activation().SaveFlag(ctx, flag, next, s.now(), caller.ID); err != nil {
			return nil, nil, err
		}
		return flagState(activation.Setting(flag)), flagState(next), nil
	})
}

// KillSwitch turns every flag OFF in one change, keeping each audience. It
// needs no revision, so a stale caller can always use it.
func (s *CommercialRolloutService) KillSwitch(ctx context.Context, caller domain.User, idempotencyKey, reason string) (RolloutReceipt, error) {
	intent, err := s.intent(caller, actionKillSwitch, resourceRollout, rolloutAll, idempotencyKey, reason, killSwitchBody{Reason: reason})
	if err != nil {
		return RolloutReceipt{}, err
	}
	return s.run(ctx, caller, intent, func(ctx context.Context, tx ports.CommercialTx) (json.RawMessage, json.RawMessage, error) {
		if err := validateReason(reason); err != nil {
			return nil, nil, err
		}
		if _, err := tx.Activation().LockExclusive(ctx); err != nil {
			return nil, nil, err
		}
		activation, err := tx.Activation().Read(ctx)
		if err != nil {
			return nil, nil, err
		}
		before, after := map[domain.FlagName]bool{}, map[domain.FlagName]bool{}
		for _, flag := range domain.CommercialFlags() {
			setting := activation.Setting(flag)
			before[flag], after[flag] = setting.Enabled, false
			setting.Enabled = false
			if err := tx.Activation().SaveFlag(ctx, flag, setting, s.now(), caller.ID); err != nil {
				return nil, nil, err
			}
		}
		return mustJSON(map[string]map[domain.FlagName]bool{"enabled": before}), mustJSON(map[string]map[domain.FlagName]bool{"enabled": after}), nil
	})
}

// SaveGroup creates a group with no members or renames one; membership is
// never changed here.
func (s *CommercialRolloutService) SaveGroup(ctx context.Context, caller domain.User, slug, idempotencyKey string, change GroupChangeRequest) (RolloutReceipt, error) {
	intent, err := s.intent(caller, actionGroupSaved, resourceGroup, slug, idempotencyKey, change.Reason, change)
	if err != nil {
		return RolloutReceipt{}, err
	}
	return s.run(ctx, caller, intent, func(ctx context.Context, tx ports.CommercialTx) (json.RawMessage, json.RawMessage, error) {
		if err := domain.ValidateGroupChange(slug, change.Name); err != nil {
			return nil, nil, domain.NewValidationError("name", err.Error())
		}
		if err := validateReason(change.Reason); err != nil {
			return nil, nil, err
		}
		if _, err := tx.Activation().LockExclusive(ctx); err != nil {
			return nil, nil, err
		}
		var before json.RawMessage
		existing, err := tx.Groups().Find(ctx, slug)
		switch {
		case err == nil:
			before = mustJSON(map[string]string{"name": existing.Name})
		case !errors.Is(err, domain.ErrNotFound):
			return nil, nil, err
		}
		if err := tx.Groups().Save(ctx, slug, change.Name, s.now(), caller.ID); err != nil {
			return nil, nil, err
		}
		return before, mustJSON(map[string]string{"name": change.Name}), nil
	})
}

// ChangeMembers adds and removes members in one change. It needs no
// revision; it bumps the revision, so a removed member is fenced like an OFF.
func (s *CommercialRolloutService) ChangeMembers(ctx context.Context, caller domain.User, slug, idempotencyKey string, change MembershipChangeRequest) (RolloutReceipt, error) {
	intent, err := s.intent(caller, actionGroupMembersChanged, resourceGroup, slug, idempotencyKey, change.Reason, change)
	if err != nil {
		return RolloutReceipt{}, err
	}
	return s.run(ctx, caller, intent, func(ctx context.Context, tx ports.CommercialTx) (json.RawMessage, json.RawMessage, error) {
		group, members, err := checkedMembership(ctx, tx, slug, change)
		if err != nil {
			return nil, nil, err
		}
		if _, err := tx.Activation().LockExclusive(ctx); err != nil {
			return nil, nil, err
		}
		if err := tx.Groups().SetMembers(ctx, slug, members, s.now(), caller.ID); err != nil {
			return nil, nil, err
		}
		before := mustJSON(map[string][]string{"member_ids": nonNilStrings(group.MemberIDs)})
		after := mustJSON(map[string][]string{
			"member_ids": members, "added": nonNilStrings(change.Add), "removed": nonNilStrings(change.Remove),
		})
		return before, after, nil
	})
}

// checkedMembership validates a membership change against its group and
// returns the group as it was and the membership that results.
func checkedMembership(ctx context.Context, tx ports.CommercialTx, slug string, change MembershipChangeRequest) (domain.CommercialGroup, []string, error) {
	if err := validateReason(change.Reason); err != nil {
		return domain.CommercialGroup{}, nil, err
	}
	if len(change.Add) > maxMembershipChange || len(change.Remove) > maxMembershipChange {
		return domain.CommercialGroup{}, nil, domain.NewValidationError("add", fmt.Sprintf("at most %d users per list", maxMembershipChange))
	}
	// The request is checked on its own before the group is looked up, so a
	// malformed change is a 400 whether or not the group exists.
	if _, err := domain.ApplyMembershipChange(nil, change.Add, change.Remove); err != nil {
		return domain.CommercialGroup{}, nil, domain.NewValidationError("add", err.Error())
	}
	missing, err := tx.Groups().MissingUsers(ctx, append(append([]string{}, change.Add...), change.Remove...))
	if err != nil {
		return domain.CommercialGroup{}, nil, err
	}
	if len(missing) > 0 {
		return domain.CommercialGroup{}, nil, domain.NewValidationError("add", "unknown users: "+strings.Join(missing, ", "))
	}
	group, err := tx.Groups().Find(ctx, slug)
	if err != nil {
		return domain.CommercialGroup{}, nil, err
	}
	members, err := domain.ApplyMembershipChange(group.MemberIDs, change.Add, change.Remove)
	if err != nil {
		return domain.CommercialGroup{}, nil, domain.NewValidationError("add", err.Error())
	}
	return group, members, nil
}

// rolloutEffect changes activation and returns the evidence's before and
// after states; run bumps the revision and builds the receipt.
type rolloutEffect func(ctx context.Context, tx ports.CommercialTx) (before, after json.RawMessage, err error)

func (s *CommercialRolloutService) run(ctx context.Context, caller domain.User, intent CommercialIntent, change rolloutEffect) (RolloutReceipt, error) {
	authorize := func(context.Context) error {
		if caller.Role != domain.RoleAdmin {
			return domain.ErrForbidden
		}
		return nil
	}
	receipt, err := s.commands.Run(ctx, intent, authorize, func(ctx context.Context, tx ports.CommercialTx, run CommercialRun) (CommercialEffectResult, error) {
		before, after, err := change(ctx, tx)
		if err != nil {
			return CommercialEffectResult{}, err
		}
		revision, err := tx.Activation().BumpRevision(ctx)
		if err != nil {
			return CommercialEffectResult{}, err
		}
		result := mustJSON(RolloutReceipt{
			OperationID: run.OperationID, Status: string(domain.AuditOutcomeApplied),
			Revision: revision, EvidenceEntryID: run.EvidenceEntryID,
		})
		return CommercialEffectResult{
			Result: result, AuthorizedRevision: revision - 1,
			ConfigurationRevision: strconv.FormatInt(revision, 10), Before: before, After: after,
		}, nil
	})
	if err != nil {
		return RolloutReceipt{}, err
	}
	var rollout RolloutReceipt
	if err := json.Unmarshal(receipt.Result, &rollout); err != nil {
		return RolloutReceipt{}, fmt.Errorf("%w: recorded receipt is unreadable: %w", domain.ErrUnavailable, err)
	}
	return rollout, nil
}

func (s *CommercialRolloutService) intent(caller domain.User, action, resourceKind, resourceID, idempotencyKey, reason string, body interface{ isRolloutBody() }) (CommercialIntent, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return CommercialIntent{}, fmt.Errorf("encode rollout intent: %w", err)
	}
	if resourceID == "" {
		return CommercialIntent{}, domain.NewValidationError("resource", "is required")
	}
	return CommercialIntent{
		Key: domain.OperationKey{
			Environment: s.environment, ActorKind: domain.AuditActorUser, ActorID: caller.ID,
			Action: action, ResourceKind: resourceKind, ResourceID: resourceID, CallerKey: idempotencyKey,
		},
		Version: rolloutIntentVersion, Body: encoded, Reason: reasonOrDefault(reason),
		PolicyRevision: domain.CommercialRegistryRevision,
	}, nil
}

func (FlagChangeRequest) isRolloutBody()       {}
func (GroupChangeRequest) isRolloutBody()      {}
func (MembershipChangeRequest) isRolloutBody() {}
func (killSwitchBody) isRolloutBody()          {}

type killSwitchBody struct {
	Reason string `json:"reason"`
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func validateReason(reason string) error {
	if strings.TrimSpace(reason) == "" || len([]rune(reason)) > maxRolloutReasonChars {
		return domain.NewValidationError("reason", fmt.Sprintf("must be 1 to %d characters", maxRolloutReasonChars))
	}
	return nil
}

// reasonOrDefault keeps evidence well-formed for a request refused for
// having no reason.
func reasonOrDefault(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return "no reason given"
	}
	return reason
}

// parseAllowlist parses allowlist entries; each group entry must name an
// existing group.
func parseAllowlist(ctx context.Context, tx ports.CommercialTx, values []string) ([]domain.RolloutSubject, error) {
	if len(values) > maxRolloutSubjects {
		return nil, domain.NewValidationError("subject_ids", fmt.Sprintf("at most %d subjects", maxRolloutSubjects))
	}
	seen := make(map[string]bool, len(values))
	subjects := make([]domain.RolloutSubject, 0, len(values))
	for _, value := range values {
		if seen[value] {
			return nil, domain.NewValidationError("subject_ids", "repeats "+value)
		}
		seen[value] = true
		subject, err := domain.ParseRolloutSubject(value)
		if err != nil {
			return nil, domain.NewValidationError("subject_ids", err.Error())
		}
		if subject.Kind == domain.RolloutSubjectGroup {
			if _, err := tx.Groups().Find(ctx, subject.ID); errors.Is(err, domain.ErrNotFound) {
				return nil, domain.NewValidationError("subject_ids", "no group "+subject.ID)
			} else if err != nil {
				return nil, err
			}
		}
		subjects = append(subjects, subject)
	}
	return subjects, nil
}

func flagState(setting domain.FlagSetting) json.RawMessage {
	return mustJSON(flagStateJSON{setting.Enabled, setting.Audience.AllUsers, subjectStrings(setting.Audience.Entries)})
}

// flagStateJSON is a flag's switch and audience as recorded in evidence.
type flagStateJSON struct {
	Enabled    bool     `json:"enabled"`
	AllUsers   bool     `json:"all_users"`
	SubjectIDs []string `json:"subject_ids"`
}

func subjectStrings(subjects []domain.RolloutSubject) []string {
	values := make([]string, len(subjects))
	for i, subject := range subjects {
		values[i] = subject.String()
	}
	return values
}

func nonNilFlags(flags []domain.FlagName) []domain.FlagName {
	if flags == nil {
		return []domain.FlagName{}
	}
	return flags
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// rolloutJSON lists the shapes this service writes as JSON: receipts and
// evidence states built from strings, booleans and integers.
type rolloutJSON interface {
	RolloutReceipt | flagStateJSON | map[string]string | map[string][]string | map[string]map[domain.FlagName]bool
}

// mustJSON marshals one of those shapes, which always marshal.
func mustJSON[T rolloutJSON](value T) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal rollout value: %v", err))
	}
	return encoded
}
