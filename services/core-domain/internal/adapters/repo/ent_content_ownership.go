package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentownership"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// recordOwnership writes an item's ownership revision and the evidence that
// records it inside tx, so they commit with the item or not at all.
func recordOwnership(ctx context.Context, tx *ent.Tx, record ports.OwnershipRecord) error {
	if err := (entCommercialAuditWriter{tx: tx}).Append(ctx, record.Evidence); err != nil {
		return fmt.Errorf("record ownership evidence: %w", err)
	}
	return insertOwnership(ctx, tx, record.Ownership)
}

func insertOwnership(ctx context.Context, tx *ent.Tx, ownership domain.ContentOwnership) error {
	contentID, err := uuid.Parse(ownership.Content.ID)
	if err != nil {
		return fmt.Errorf("ownership content id: %w", err)
	}
	evidenceID, err := uuid.Parse(ownership.EvidenceEntryID)
	if err != nil {
		return fmt.Errorf("ownership evidence id: %w", err)
	}
	ownerID, err := parseOptionalUUID(ownership.OwnerID)
	if err != nil {
		return fmt.Errorf("ownership owner id: %w", err)
	}
	creatorID, err := parseOptionalUUID(ownership.CreatorID)
	if err != nil {
		return fmt.Errorf("ownership creator id: %w", err)
	}
	return tx.ContentOwnership.Create().
		SetContentKind(contentownership.ContentKind(ownership.Content.Kind)).
		SetContentID(contentID).
		SetRevision(ownership.Revision).
		SetOwnerKind(contentownership.OwnerKind(ownership.OwnerKind)).
		SetNillableOwnerID(ownerID).
		SetNillableCreatorID(creatorID).
		SetSource(contentownership.Source(ownership.Source)).
		SetEvidenceEntryID(evidenceID).
		Exec(ctx)
}

// latestOwnershipQuery locks an item's current ownership revision.
const latestOwnershipQuery = `SELECT id FROM content_ownerships
WHERE content_kind = $1 AND content_id = $2
ORDER BY revision DESC LIMIT 1 FOR UPDATE`

func (s *EntCommercialStore) CurrentOwnership(ctx context.Context, content domain.ContentRef) (domain.ContentOwnership, error) {
	contentID, err := uuid.Parse(content.ID)
	if err != nil {
		return domain.ContentOwnership{}, domain.ErrNotFound
	}
	row, err := s.client.ContentOwnership.Query().
		Where(
			contentownership.ContentKindEQ(contentownership.ContentKind(content.Kind)),
			contentownership.ContentID(contentID),
		).
		Order(ent.Desc(contentownership.FieldRevision)).
		First(ctx)
	if ent.IsNotFound(err) {
		return domain.ContentOwnership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ContentOwnership{}, err
	}
	return ownershipFromRow(row), nil
}

type entContentOwnershipTx struct {
	tx *ent.Tx
}

func (t entCommercialTx) Ownership() ports.ContentOwnershipTx {
	return entContentOwnershipTx(t)
}

func (o entContentOwnershipTx) LockCurrent(ctx context.Context, content domain.ContentRef) (domain.ContentOwnership, error) {
	contentID, err := uuid.Parse(content.ID)
	if err != nil {
		return domain.ContentOwnership{}, domain.ErrNotFound
	}
	rows, err := o.tx.QueryContext(ctx, latestOwnershipQuery, string(content.Kind), contentID)
	if err != nil {
		return domain.ContentOwnership{}, err
	}
	id, err := scanSingleUUID(rows)
	if err != nil {
		return domain.ContentOwnership{}, err
	}
	row, err := o.tx.ContentOwnership.Get(ctx, id)
	if err != nil {
		return domain.ContentOwnership{}, err
	}
	return ownershipFromRow(row), nil
}

func (o entContentOwnershipTx) Append(ctx context.Context, ownership domain.ContentOwnership) error {
	err := insertOwnership(ctx, o.tx, ownership)
	if ent.IsConstraintError(err) {
		return fmt.Errorf("%w: ownership revision %d is already recorded", domain.ErrConflict, ownership.Revision)
	}
	return err
}

// scanSingleUUID reads the one id a query returns, or domain.ErrNotFound.
func scanSingleUUID(rows *sql.Rows) (uuid.UUID, error) {
	defer rows.Close() //nolint:errcheck // rows.Err below reports any failure that matters
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return uuid.Nil, err
		}
		return uuid.Nil, domain.ErrNotFound
	}
	var id uuid.UUID
	if err := rows.Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, rows.Err()
}

func ownershipFromRow(row *ent.ContentOwnership) domain.ContentOwnership {
	return domain.ContentOwnership{
		Content:         domain.ContentRef{Kind: domain.ContentKind(row.ContentKind), ID: row.ContentID.String()},
		OwnerKind:       domain.OwnerKind(row.OwnerKind),
		OwnerID:         optionalUUIDString(row.OwnerID),
		CreatorID:       optionalUUIDString(row.CreatorID),
		Source:          domain.OwnershipSource(row.Source),
		EvidenceEntryID: row.EvidenceEntryID.String(),
		Revision:        row.Revision,
	}
}

func optionalUUIDString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}
