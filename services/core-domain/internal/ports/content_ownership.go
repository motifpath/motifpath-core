package ports

import (
	"context"

	"github.com/motifpath/core-domain/internal/domain"
)

// OwnershipRecord is an item's first ownership revision with the evidence
// that records it. A repository that creates the item writes both in the
// same transaction as the item, so content never exists without its owner.
type OwnershipRecord struct {
	Ownership domain.ContentOwnership
	Evidence  domain.AuditEntry
}

// UnownedContent is an item that has no ownership record yet, with its
// recorded author when one is known.
type UnownedContent struct {
	Content domain.ContentRef
	Author  *domain.LegacyAuthor
}

// OwnershipBackfillStore finds content created before ownership was recorded
// and records its attribution.
type OwnershipBackfillStore interface {
	// ListUnowned returns up to limit items with no ownership record, in a
	// stable order. Whether an author is an admin is read when it is listed.
	ListUnowned(ctx context.Context, limit int) ([]UnownedContent, error)
	// RecordBackfilled stores an item's first ownership revision and its
	// evidence together. An item that was recorded meanwhile is left as it is.
	RecordBackfilled(ctx context.Context, record OwnershipRecord) error
}
