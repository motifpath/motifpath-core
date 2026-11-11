package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// AuditOutcome is what an evidenced attempt led to. Uncertain never implies
// that an external effect failed or succeeded.
type AuditOutcome string

const (
	AuditOutcomeApplied   AuditOutcome = "applied"
	AuditOutcomeFailed    AuditOutcome = "failed"
	AuditOutcomeDenied    AuditOutcome = "denied"
	AuditOutcomeDuplicate AuditOutcome = "duplicate"
	AuditOutcomeUncertain AuditOutcome = "uncertain"
)

func (o AuditOutcome) valid() bool {
	switch o {
	case AuditOutcomeApplied, AuditOutcomeFailed, AuditOutcomeDenied, AuditOutcomeDuplicate, AuditOutcomeUncertain:
		return true
	default:
		return false
	}
}

const (
	maxAuditResourceKindLength = 100
	maxAuditResourceIDLength   = 255
)

// ErrInvalidAuditEntry is returned for evidence that cannot be appended.
var ErrInvalidAuditEntry = errors.New("invalid commercial audit entry")

// AuditEntry is one immutable piece of commercial evidence: who did what to
// which resource, under which policy and configuration, and with what result.
// A nil state means no authoritative state was available, which is different
// from an empty one.
type AuditEntry struct {
	AuditEntryID          string
	Operation             OperationKey
	AttemptID             string
	OccurredAt            time.Time
	RecordedAt            time.Time
	Reason                string
	Outcome               AuditOutcome
	BeforeState           json.RawMessage
	AfterState            json.RawMessage
	PolicyRevision        string
	ConfigurationRevision string
	CorrelationID         string
	CausedByEntryIDs      []string
	ProviderReferences    []string
}

// Validate checks every field the evidence contract requires.
func (e AuditEntry) Validate() error {
	if err := e.Operation.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAuditEntry, err)
	}
	checks := []struct {
		ok     bool
		reason string
	}{
		{lowerUUIDPattern.MatchString(e.AuditEntryID), "entry id is not a lowercase UUID"},
		{lowerUUIDPattern.MatchString(e.AttemptID), "attempt id is not a lowercase UUID"},
		{lowerUUIDPattern.MatchString(e.CorrelationID), "correlation id is not a lowercase UUID"},
		{len(e.Operation.ResourceKind) <= maxAuditResourceKindLength, "resource kind is too long"},
		{len(e.Operation.ResourceID) <= maxAuditResourceIDLength, "resource id is too long"},
		{!e.OccurredAt.IsZero(), "no occurrence time"},
		{!e.RecordedAt.IsZero(), "no recording time"},
		{e.Reason != "", "no reason"},
		{e.Outcome.valid(), "unknown outcome"},
		{e.PolicyRevision != "", "no policy revision"},
		{e.ConfigurationRevision != "", "no configuration revision"},
		{isNilOrJSONObject(e.BeforeState), "before state is not an object"},
		{isNilOrJSONObject(e.AfterState), "after state is not an object"},
		{distinctUUIDs(e.CausedByEntryIDs), "causes are not distinct UUIDs"},
		{distinctNonEmpty(e.ProviderReferences), "provider references are not distinct and non-empty"},
	}
	for _, check := range checks {
		if !check.ok {
			return fmt.Errorf("%w: %s", ErrInvalidAuditEntry, check.reason)
		}
	}
	return nil
}

func isNilOrJSONObject(state json.RawMessage) bool {
	if state == nil {
		return true
	}
	trimmed := bytes.TrimSpace(state)
	return len(trimmed) > 0 && trimmed[0] == '{' && json.Valid(trimmed)
}

func distinctUUIDs(ids []string) bool {
	for _, id := range ids {
		if !lowerUUIDPattern.MatchString(id) {
			return false
		}
	}
	return distinctNonEmpty(ids)
}

func distinctNonEmpty(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
