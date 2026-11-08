package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// CommercialIntent is one submitted commercial command: its scoped key and
// the intent that key is bound to.
type CommercialIntent struct {
	Key domain.OperationKey
	// Version is the intent's schema version; a new version is a new intent.
	Version int
	// Body is the full request intent. It is fingerprinted canonically, so a
	// missing field and an explicit null are different intents.
	Body json.RawMessage
	// Reason is the caller's business reason, recorded in the evidence.
	Reason string
	// PolicyRevision is the business policy the command is decided under.
	PolicyRevision string
}

// CommercialRun identifies the run an effect belongs to, so its result can
// name the evidence committed with it.
type CommercialRun struct {
	OperationID     string
	EvidenceEntryID string
}

// CommercialEffectResult is what an applied effect reports.
type CommercialEffectResult struct {
	// Result is the response; every same-intent replay gets it back unchanged.
	Result json.RawMessage
	// AuthorizedRevision is the activation revision the effect was authorized
	// under.
	AuthorizedRevision int64
	// ConfigurationRevision is the commercial configuration the effect applied.
	ConfigurationRevision string
	// Before and After are the allowlisted states for the evidence.
	Before, After json.RawMessage
}

// CommercialEffect applies a command inside its transaction. It takes the
// locks it needs. Returning a domain refusal (validation, not found, conflict,
// forbidden) refuses the command; any other error makes it unavailable.
type CommercialEffect func(ctx context.Context, tx ports.CommercialTx, run CommercialRun) (CommercialEffectResult, error)

// CommercialReceipt is the answer to a command: its result, and whether it is
// the replay of an operation already applied.
type CommercialReceipt struct {
	OperationID string
	Result      json.RawMessage
	Replayed    bool
}

// unresolvedConfiguration is the configuration revision recorded for a
// refusal decided before any configuration was read.
const unresolvedConfiguration = "unresolved"

// errOperationRaced means a concurrent submission recorded the same key
// first; the run starts over and finds that operation.
var errOperationRaced = errors.New("commercial operation raced")

// CommercialCommands runs commercial commands idempotently, committing each
// change with its operation record and its evidence, or nothing at all.
type CommercialCommands struct {
	store ports.CommercialStore
	now   func() time.Time
	newID func() string
}

func NewCommercialCommands(store ports.CommercialStore, now func() time.Time, newID func() string) *CommercialCommands {
	return &CommercialCommands{store: store, now: now, newID: newID}
}

// Run authorizes the caller, then answers a known key from its record —
// before the effect could refuse it as stale — or applies the effect once.
// Refusals are evidenced too. If the change or its evidence cannot be
// written, nothing changes and the error is domain.ErrUnavailable.
func (c *CommercialCommands) Run(ctx context.Context, intent CommercialIntent, authorize func(context.Context) error, apply CommercialEffect) (CommercialReceipt, error) {
	if err := intent.Key.Validate(); err != nil {
		return CommercialReceipt{}, domain.NewValidationError("idempotency_key", err.Error())
	}
	fingerprint, err := domain.FingerprintIntent(intent.Key.Action, intent.Version,
		intent.Key.ResourceKind+":"+intent.Key.ResourceID, intent.Body)
	if err != nil {
		return CommercialReceipt{}, domain.NewValidationError("body", err.Error())
	}
	if err := authorize(ctx); err != nil {
		return CommercialReceipt{}, c.refuse(ctx, intent, err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		receipt, err := c.runOnce(ctx, intent, fingerprint, apply)
		if !errors.Is(err, errOperationRaced) {
			return receipt, err
		}
	}
	return CommercialReceipt{}, fmt.Errorf("%w: operation %s kept racing", domain.ErrUnavailable, intent.Key.OperationID())
}

func (c *CommercialCommands) runOnce(ctx context.Context, intent CommercialIntent, fingerprint string, apply CommercialEffect) (CommercialReceipt, error) {
	var receipt CommercialReceipt
	var reused bool
	var refusal error
	err := c.store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		existing, err := tx.Operations().Find(ctx, intent.Key)
		if err == nil {
			if existing.Fingerprint == fingerprint {
				receipt = CommercialReceipt{OperationID: intent.Key.OperationID(), Result: existing.Result, Replayed: true}
				return c.recordReplay(ctx, tx, intent, fingerprint, domain.AuditOutcomeDuplicate, "same-intent replay of a recorded operation")
			}
			reused = true
			return c.recordReplay(ctx, tx, intent, fingerprint, domain.AuditOutcomeDenied, "idempotency key reused for a different intent")
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return err
		}

		run := CommercialRun{OperationID: intent.Key.OperationID(), EvidenceEntryID: c.newID()}
		effect, err := apply(ctx, tx, run)
		if err != nil {
			refusal = err
			return err
		}
		receipt = CommercialReceipt{OperationID: run.OperationID, Result: effect.Result}
		return c.recordApplied(ctx, tx, intent, fingerprint, run, effect)
	})

	switch {
	case reused && err == nil:
		return CommercialReceipt{}, fmt.Errorf("%w: idempotency key was used for a different intent", domain.ErrConflict)
	case err == nil:
		return receipt, nil
	case errors.Is(err, errOperationRaced):
		return CommercialReceipt{}, err
	case refusal != nil && isCommercialRefusal(refusal):
		return CommercialReceipt{}, c.refuse(ctx, intent, refusal)
	default:
		return CommercialReceipt{}, fmt.Errorf("%w: %w", domain.ErrUnavailable, err)
	}
}

func (c *CommercialCommands) recordApplied(ctx context.Context, tx ports.CommercialTx, intent CommercialIntent, fingerprint string, run CommercialRun, effect CommercialEffectResult) error {
	now := c.now()
	err := tx.Operations().Insert(ctx, domain.CommercialOperation{
		Key: intent.Key, Fingerprint: fingerprint, Result: effect.Result,
		AuthorizedRevision: effect.AuthorizedRevision, CreatedAt: now,
	})
	if errors.Is(err, domain.ErrConflict) {
		return errOperationRaced
	}
	if err != nil {
		return err
	}
	attemptID := c.newID()
	if err := tx.Operations().RecordAttempt(ctx, domain.OperationAttempt{
		AttemptID: attemptID, Key: intent.Key, Fingerprint: fingerprint, Outcome: domain.AuditOutcomeApplied, At: now,
	}); err != nil {
		return err
	}
	return tx.Audit().Append(ctx, c.evidence(intent, run.EvidenceEntryID, attemptID, domain.AuditOutcomeApplied,
		intent.Reason, effect.ConfigurationRevision, effect.Before, effect.After))
}

func (c *CommercialCommands) recordReplay(ctx context.Context, tx ports.CommercialTx, intent CommercialIntent, fingerprint string, outcome domain.AuditOutcome, reason string) error {
	attemptID := c.newID()
	if err := tx.Operations().RecordAttempt(ctx, domain.OperationAttempt{
		AttemptID: attemptID, Key: intent.Key, Fingerprint: fingerprint, Outcome: outcome, At: c.now(),
	}); err != nil {
		return err
	}
	return tx.Audit().Append(ctx, c.evidence(intent, c.newID(), attemptID, outcome, reason, unresolvedConfiguration, nil, nil))
}

// refuse evidences a refused command in a transaction of its own and returns
// the refusal, or domain.ErrUnavailable if the evidence cannot be written.
func (c *CommercialCommands) refuse(ctx context.Context, intent CommercialIntent, refusal error) error {
	entry := c.evidence(intent, c.newID(), c.newID(), domain.AuditOutcomeDenied, refusal.Error(), unresolvedConfiguration, nil, nil)
	err := c.store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
		return tx.Audit().Append(ctx, entry)
	})
	if err != nil {
		return fmt.Errorf("%w: refusal could not be evidenced: %w", domain.ErrUnavailable, err)
	}
	return refusal
}

func (c *CommercialCommands) evidence(intent CommercialIntent, entryID, attemptID string, outcome domain.AuditOutcome, reason, configurationRevision string, before, after json.RawMessage) domain.AuditEntry {
	now := c.now()
	return domain.AuditEntry{
		AuditEntryID:          entryID,
		Operation:             intent.Key,
		AttemptID:             attemptID,
		OccurredAt:            now,
		RecordedAt:            now,
		Reason:                reason,
		Outcome:               outcome,
		BeforeState:           before,
		AfterState:            after,
		PolicyRevision:        intent.PolicyRevision,
		ConfigurationRevision: configurationRevision,
		CorrelationID:         c.newID(),
		CausedByEntryIDs:      []string{},
		ProviderReferences:    []string{},
	}
}

func isCommercialRefusal(err error) bool {
	return errors.Is(err, domain.ErrValidation) || errors.Is(err, domain.ErrNotFound) ||
		errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrForbidden)
}
