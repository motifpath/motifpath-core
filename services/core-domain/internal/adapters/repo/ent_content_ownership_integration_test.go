//go:build integration

package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent/commercialauditentry"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentownership"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

func TestEntContentCreation_RecordsOwnershipWithTheContent(t *testing.T) {
	client, db := setupMigratedPostgres(t)
	ctx := context.Background()
	nodes := NewEntContentNodeRepository(client)
	paths := NewEntLearningPathRepository(client)
	node := seedContentNode(t, ctx, nodes)
	admin := domain.User{ID: uuid.NewString(), Role: domain.RoleAdmin}

	t.Run("a created item is stored with its first ownership revision and its evidence", func(t *testing.T) {
		path := domain.LearningPath{
			ID: uuid.NewString(), TeacherID: admin.ID, Title: "First chords", CreatedAt: fixedAt,
			Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: node.ID, Title: node.Title, ContentType: node.ContentType}},
		}
		record := ownershipBy(t, domain.ContentPath, path.ID, admin)

		require.NoError(t, paths.Create(ctx, path, record))

		row, err := client.ContentOwnership.Query().
			Where(contentownership.ContentKindEQ(contentownership.ContentKindPath), contentownership.ContentID(uuid.MustParse(path.ID))).
			Only(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, row.Revision)
		assert.Equal(t, contentownership.OwnerKindPlatform, row.OwnerKind)
		assert.Equal(t, domain.CatalogProfileID, row.OwnerID.String())
		assert.Equal(t, admin.ID, row.CreatorID.String())
		assert.Equal(t, contentownership.SourceNative, row.Source)

		evidence, err := client.CommercialAuditEntry.Query().
			Where(commercialauditentry.AuditEntryID(row.EvidenceEntryID)).
			Only(ctx)
		require.NoError(t, err)
		assert.Equal(t, "path", evidence.ResourceKind)
		assert.Equal(t, path.ID, evidence.ResourceID)
		assert.Equal(t, admin.ID, evidence.ActorID)
	})

	t.Run("content whose ownership evidence cannot be stored is not created", func(t *testing.T) {
		path := domain.LearningPath{
			ID: uuid.NewString(), TeacherID: admin.ID, Title: "Unrecorded", CreatedAt: fixedAt,
			Items: []domain.LearningPathItem{{Position: 1, ContentNodeID: node.ID, Title: node.Title, ContentType: node.ContentType}},
		}
		record := ownershipBy(t, domain.ContentPath, path.ID, admin)
		record.Evidence.Operation.Environment = domain.Environment{}

		require.Error(t, paths.Create(ctx, path, record))

		_, err := paths.GetByID(ctx, path.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)
		count, err := client.ContentOwnership.Query().Where(contentownership.ContentID(uuid.MustParse(path.ID))).Count(ctx)
		require.NoError(t, err)
		assert.Zero(t, count)
	})

	t.Run("a recorded ownership revision cannot be changed", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `UPDATE content_ownerships SET owner_kind = 'teacher' WHERE content_id = $1`, node.ID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "append-only")
	})
}

func TestEntCommercialStore_Ownership(t *testing.T) {
	client, _ := setupMigratedPostgres(t)
	ctx := context.Background()
	store := NewEntCommercialStore(client)
	node := seedContentNode(t, ctx, NewEntContentNodeRepository(client))
	ref := domain.ContentRef{Kind: domain.ContentContentNode, ID: node.ID}

	t.Run("reads the current revision of recorded content", func(t *testing.T) {
		current, err := store.CurrentOwnership(ctx, ref)

		require.NoError(t, err)
		assert.Equal(t, 1, current.Revision)
		assert.Equal(t, domain.OwnerTeacher, current.OwnerKind)
	})

	t.Run("content without an ownership record is not found", func(t *testing.T) {
		_, err := store.CurrentOwnership(ctx, domain.ContentRef{Kind: domain.ContentPath, ID: uuid.NewString()})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a correction appends the next revision and keeps the first", func(t *testing.T) {
		err := store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			current, err := tx.Ownership().LockCurrent(ctx, ref)
			if err != nil {
				return err
			}
			next, err := current.Corrected(domain.OwnershipCorrection{ExpectedRevision: 1, OwnerKind: domain.OwnerPlatform, OwnerID: domain.CatalogProfileID}, uuid.NewString())
			if err != nil {
				return err
			}
			return tx.Ownership().Append(ctx, next)
		})
		require.NoError(t, err)

		current, err := store.CurrentOwnership(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, 2, current.Revision)
		assert.Equal(t, domain.OwnerPlatform, current.OwnerKind)
		count, err := client.ContentOwnership.Query().Where(contentownership.ContentID(uuid.MustParse(node.ID))).Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("appending a revision already recorded is a conflict", func(t *testing.T) {
		err := store.InTx(ctx, func(ctx context.Context, tx ports.CommercialTx) error {
			current, err := tx.Ownership().LockCurrent(ctx, ref)
			if err != nil {
				return err
			}
			current.EvidenceEntryID = uuid.NewString()
			return tx.Ownership().Append(ctx, current)
		})

		assert.ErrorIs(t, err, domain.ErrConflict)
	})
}
