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

// EntOwnershipBackfillStore finds content stored before ownership was
// recorded, and records its attribution.
type EntOwnershipBackfillStore struct {
	client *ent.Client
}

func NewEntOwnershipBackfillStore(client *ent.Client) *EntOwnershipBackfillStore {
	return &EntOwnershipBackfillStore{client: client}
}

// unownedContentQuery lists every course, path, content node and exercise
// without an ownership record, with its recorded author — created_by for
// courses and exercises, teacher_id for paths and content nodes — and whether
// that author is an admin now.
const unownedContentQuery = `
WITH content AS (
	SELECT 'course' AS kind, id, created_by AS author_id FROM courses
	UNION ALL SELECT 'path', id, teacher_id FROM learning_paths
	UNION ALL SELECT 'content_node', id, teacher_id FROM content_nodes
	UNION ALL SELECT 'exercise', id, created_by FROM exercises
)
SELECT c.kind, c.id::text, c.author_id::text, COALESCE(u.role = 'admin', false)
FROM content c
LEFT JOIN users u ON u.id = c.author_id
WHERE NOT EXISTS (
	SELECT 1 FROM content_ownerships o WHERE o.content_kind = c.kind AND o.content_id = c.id
)
ORDER BY c.kind, c.id
LIMIT $1`

func (s *EntOwnershipBackfillStore) ListUnowned(ctx context.Context, limit int) ([]ports.UnownedContent, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, unownedContentQuery, limit)
	if err != nil {
		return nil, rollback(tx, err)
	}
	unowned, err := scanUnownedContent(rows)
	if err != nil {
		return nil, rollback(tx, err)
	}
	return unowned, tx.Commit()
}

func scanUnownedContent(rows *sql.Rows) ([]ports.UnownedContent, error) {
	defer rows.Close() //nolint:errcheck // rows.Err below reports any failure that matters
	var unowned []ports.UnownedContent
	for rows.Next() {
		var kind, id string
		var authorID sql.NullString
		var isAdmin bool
		if err := rows.Scan(&kind, &id, &authorID, &isAdmin); err != nil {
			return nil, err
		}
		item := ports.UnownedContent{Content: domain.ContentRef{Kind: domain.ContentKind(kind), ID: id}}
		if authorID.Valid {
			item.Author = &domain.LegacyAuthor{ID: authorID.String, IsAdmin: isAdmin}
		}
		unowned = append(unowned, item)
	}
	return unowned, rows.Err()
}

func (s *EntOwnershipBackfillStore) RecordBackfilled(ctx context.Context, record ports.OwnershipRecord) error {
	contentID, err := uuid.Parse(record.Ownership.Content.ID)
	if err != nil {
		return fmt.Errorf("ownership content id: %w", err)
	}
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	recorded, err := tx.ContentOwnership.Query().
		Where(
			contentownership.ContentKindEQ(contentownership.ContentKind(record.Ownership.Content.Kind)),
			contentownership.ContentID(contentID),
		).
		Exist(ctx)
	if err != nil {
		return rollback(tx, err)
	}
	if recorded {
		return tx.Rollback()
	}
	if err := recordOwnership(ctx, tx, record); err != nil {
		// A concurrent run recorded the same item first: it stays as that
		// run recorded it.
		if ent.IsConstraintError(err) {
			return tx.Rollback()
		}
		return rollback(tx, err)
	}
	return tx.Commit()
}
