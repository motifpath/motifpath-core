package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

func validAuditEntry(t *testing.T) domain.AuditEntry {
	t.Helper()
	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	return domain.AuditEntry{
		AuditEntryID:          "1a2b3c4d-0000-4000-8000-000000000001",
		Operation:             validOperationKey(t),
		AttemptID:             "1a2b3c4d-0000-4000-8000-000000000002",
		OccurredAt:            at,
		RecordedAt:            at,
		Reason:                "Foundation rehearsal",
		Outcome:               domain.AuditOutcomeApplied,
		BeforeState:           json.RawMessage(`{"enabled":false}`),
		AfterState:            json.RawMessage(`{"enabled":true}`),
		PolicyRevision:        domain.CommercialRegistryRevision,
		ConfigurationRevision: "1",
		CorrelationID:         "1a2b3c4d-0000-4000-8000-000000000003",
	}
}

func TestAuditEntry_Validate(t *testing.T) {
	t.Run("a complete entry is valid", func(t *testing.T) {
		require.NoError(t, validAuditEntry(t).Validate())
	})

	t.Run("a denied attempt may have no authoritative state", func(t *testing.T) {
		entry := validAuditEntry(t)
		entry.Outcome = domain.AuditOutcomeDenied
		entry.BeforeState = nil
		entry.AfterState = nil

		require.NoError(t, entry.Validate())
	})

	for name, mutate := range map[string]func(*domain.AuditEntry){
		"no entry id":                func(e *domain.AuditEntry) { e.AuditEntryID = "" },
		"an entry id not a UUID":     func(e *domain.AuditEntry) { e.AuditEntryID = "entry-1" },
		"no attempt id":              func(e *domain.AuditEntry) { e.AttemptID = "" },
		"no correlation id":          func(e *domain.AuditEntry) { e.CorrelationID = "" },
		"an invalid operation":       func(e *domain.AuditEntry) { e.Operation.ActorID = "" },
		"no reason":                  func(e *domain.AuditEntry) { e.Reason = "" },
		"an unknown outcome":         func(e *domain.AuditEntry) { e.Outcome = "maybe" },
		"no policy revision":         func(e *domain.AuditEntry) { e.PolicyRevision = "" },
		"no configuration revision":  func(e *domain.AuditEntry) { e.ConfigurationRevision = "" },
		"no occurrence time":         func(e *domain.AuditEntry) { e.OccurredAt = time.Time{} },
		"no recording time":          func(e *domain.AuditEntry) { e.RecordedAt = time.Time{} },
		"a state that is no object":  func(e *domain.AuditEntry) { e.AfterState = json.RawMessage(`[1]`) },
		"a resource kind too long":   func(e *domain.AuditEntry) { e.Operation.ResourceKind = strings.Repeat("k", 101) },
		"a resource id too long":     func(e *domain.AuditEntry) { e.Operation.ResourceID = strings.Repeat("r", 256) },
		"a cause that is not a UUID": func(e *domain.AuditEntry) { e.CausedByEntryIDs = []string{"x"} },
		"a repeated cause": func(e *domain.AuditEntry) {
			e.CausedByEntryIDs = []string{e.AuditEntryID, e.AuditEntryID}
		},
		"a repeated provider reference": func(e *domain.AuditEntry) { e.ProviderReferences = []string{"evt_1", "evt_1"} },
		"an empty provider reference":   func(e *domain.AuditEntry) { e.ProviderReferences = []string{""} },
	} {
		t.Run("invalid: "+name, func(t *testing.T) {
			entry := validAuditEntry(t)
			mutate(&entry)

			require.ErrorIs(t, entry.Validate(), domain.ErrInvalidAuditEntry)
		})
	}

	t.Run("every outcome in the contract is accepted", func(t *testing.T) {
		for _, outcome := range []domain.AuditOutcome{
			domain.AuditOutcomeApplied, domain.AuditOutcomeFailed, domain.AuditOutcomeDenied,
			domain.AuditOutcomeDuplicate, domain.AuditOutcomeUncertain,
		} {
			entry := validAuditEntry(t)
			entry.Outcome = outcome
			assert.NoError(t, entry.Validate(), outcome)
		}
	})
}
