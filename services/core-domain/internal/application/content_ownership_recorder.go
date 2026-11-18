package application

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// Ownership evidence actions.
const (
	ActionRecordContentOwnership   = "record_content_ownership"
	ActionBackfillContentOwnership = "backfill_content_ownership"
	ActionCorrectContentOwnership  = "correct_content_ownership"
)

// ownershipPolicyRevision is the ownership policy evidence is decided under.
const ownershipPolicyRevision = "content-ownership-r1"

// ownershipConfigurationRevision is recorded on ownership evidence, which no
// commercial pricing configuration takes part in.
const ownershipConfigurationRevision = "not_applicable"

// OwnershipRecorder builds the ownership record, and its evidence, that a
// repository writes together with the content it creates.
type OwnershipRecorder struct {
	environment domain.Environment
	newID       func() string
}

func NewOwnershipRecorder(environment domain.Environment, newID func() string) *OwnershipRecorder {
	return &OwnershipRecorder{environment: environment, newID: newID}
}

// Native records content that author created at createdAt: the platform owns
// an admin's content and a teacher owns their own. Nothing in the request can
// change it.
func (r *OwnershipRecorder) Native(content domain.ContentRef, author domain.User, createdAt time.Time) ports.OwnershipRecord {
	ownership := domain.NativeOwnership(content, author, r.newID())
	key := domain.OperationKey{
		Environment: r.environment, ActorKind: domain.AuditActorUser, ActorID: author.ID,
		Action: ActionRecordContentOwnership, ResourceKind: string(content.Kind), ResourceID: content.ID,
		CallerKey: r.newID(),
	}
	return ports.OwnershipRecord{
		Ownership: ownership,
		Evidence:  r.evidence(key, ownership.EvidenceEntryID, createdAt, "content created", nil, ownershipState(ownership)),
	}
}

func (r *OwnershipRecorder) evidence(key domain.OperationKey, entryID string, at time.Time, reason string, before, after json.RawMessage) domain.AuditEntry {
	return domain.AuditEntry{
		AuditEntryID: entryID, Operation: key, AttemptID: r.newID(),
		OccurredAt: at, RecordedAt: at, Reason: reason, Outcome: domain.AuditOutcomeApplied,
		BeforeState: before, AfterState: after,
		PolicyRevision: ownershipPolicyRevision, ConfigurationRevision: ownershipConfigurationRevision,
		CorrelationID: r.newID(), CausedByEntryIDs: []string{}, ProviderReferences: []string{},
	}
}

// ownershipStateView is the allowlisted ownership state kept in evidence.
type ownershipStateView struct {
	ContentKind string  `json:"content_kind"`
	ContentID   string  `json:"content_id"`
	OwnerKind   string  `json:"owner_kind"`
	OwnerID     *string `json:"owner_id"`
	CreatorID   *string `json:"creator_id"`
	Source      string  `json:"source"`
	Revision    int     `json:"revision"`
}

func ownershipState(o domain.ContentOwnership) json.RawMessage {
	state, err := json.Marshal(ownershipStateView{
		ContentKind: string(o.Content.Kind), ContentID: o.Content.ID, OwnerKind: string(o.OwnerKind),
		OwnerID: o.OwnerID, CreatorID: o.CreatorID, Source: string(o.Source), Revision: o.Revision,
	})
	if err != nil {
		// Only strings, pointers to strings and an int are marshalled, which
		// cannot fail; a failure here is a programming error.
		panic(fmt.Sprintf("marshal ownership state: %v", err))
	}
	return state
}
