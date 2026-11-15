package application

import (
	"context"
	"fmt"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// ownershipBackfillActor is the service identity backfill evidence names.
const ownershipBackfillActor = "content-ownership-backfill"

const defaultOwnershipBackfillBatch = 200

// OwnershipBackfill gives every item created before ownership was recorded
// its attribution: an admin's or the catalog profile's content goes to the
// platform, any other author's to that author, and content with no recorded
// author stays unresolved. It only ever records unowned items, so running it
// again changes nothing already recorded.
type OwnershipBackfill struct {
	store       ports.OwnershipBackfillStore
	environment domain.Environment
	now         func() time.Time
	newID       func() string
	batchSize   int
}

func NewOwnershipBackfill(store ports.OwnershipBackfillStore, environment domain.Environment, now func() time.Time, newID func() string) *OwnershipBackfill {
	return &OwnershipBackfill{store: store, environment: environment, now: now, newID: newID, batchSize: defaultOwnershipBackfillBatch}
}

// WithBatchSize sets how many items are read at a time.
func (b *OwnershipBackfill) WithBatchSize(size int) *OwnershipBackfill {
	b.batchSize = size
	return b
}

// Run records every unowned item and returns how many it recorded. It stops
// at the first item it cannot record; what was recorded before stays.
func (b *OwnershipBackfill) Run(ctx context.Context) (int, error) {
	recorded := 0
	for {
		batch, err := b.store.ListUnowned(ctx, b.batchSize)
		if err != nil {
			return recorded, fmt.Errorf("list unowned content: %w", err)
		}
		if len(batch) == 0 {
			return recorded, nil
		}
		for _, item := range batch {
			if err := b.store.RecordBackfilled(ctx, b.record(item)); err != nil {
				return recorded, fmt.Errorf("backfill ownership of %s %s: %w", item.Content.Kind, item.Content.ID, err)
			}
			recorded++
		}
	}
}

func (b *OwnershipBackfill) record(item ports.UnownedContent) ports.OwnershipRecord {
	recorder := NewOwnershipRecorder(b.environment, b.newID)
	ownership := domain.LegacyOwnership(item.Content, item.Author, b.newID())
	key := domain.OperationKey{
		Environment: b.environment, ActorKind: domain.AuditActorService, ActorID: ownershipBackfillActor,
		Action: ActionBackfillContentOwnership, ResourceKind: string(item.Content.Kind), ResourceID: item.Content.ID,
		CallerKey: b.newID(),
	}
	return ports.OwnershipRecord{
		Ownership: ownership,
		Evidence: recorder.evidence(key, ownership.EvidenceEntryID, b.now(),
			"ownership attributed from the recorded author", nil, ownershipState(ownership)),
	}
}
