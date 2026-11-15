//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent/contentownership"
	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

func TestEntOwnershipBackfillStore(t *testing.T) {
	client, db := setupMigratedPostgres(t)
	ctx := context.Background()
	store := NewEntOwnershipBackfillStore(client)
	users := NewEntUserRepository(client)

	admin := domain.User{ID: uuid.NewString(), ClerkUserID: "clerk-admin-" + uuid.NewString(), Role: domain.RoleAdmin, DisplayName: "Ada", Locale: domain.Language{Code: "en"}, RegisteredAt: fixedAt}
	require.NoError(t, users.Create(ctx, admin))
	teacherID := uuid.NewString()

	unownedPath := func(authorID string) string {
		t.Helper()
		id := uuid.NewString()
		_, err := db.ExecContext(ctx,
			`INSERT INTO learning_paths (id, teacher_id, title, status, created_at, updated_at) VALUES ($1, $2, 'Legacy', 'draft', now(), now())`,
			id, authorID)
		require.NoError(t, err)
		return id
	}
	unownedExercise := func() string {
		t.Helper()
		id := uuid.NewString()
		_, err := db.ExecContext(ctx,
			`INSERT INTO exercises (id, title, prompt, exercise_type, created_at, updated_at) VALUES ($1, 'Legacy', '{"type":"doc","content":[]}', 'text_response', now(), now())`,
			id)
		require.NoError(t, err)
		return id
	}

	adminPath := unownedPath(admin.ID)
	teacherPath := unownedPath(teacherID)
	catalogPath := unownedPath(domain.CatalogProfileID)
	orphanExercise := unownedExercise()

	t.Run("lists unowned content with its recorded author", func(t *testing.T) {
		unowned, err := store.ListUnowned(ctx, 100)

		require.NoError(t, err)
		byID := map[string]ports.UnownedContent{}
		for _, item := range unowned {
			byID[item.Content.ID] = item
		}
		require.Contains(t, byID, adminPath)
		assert.Equal(t, domain.ContentPath, byID[adminPath].Content.Kind)
		assert.Equal(t, &domain.LegacyAuthor{ID: admin.ID, IsAdmin: true}, byID[adminPath].Author)
		assert.Equal(t, &domain.LegacyAuthor{ID: teacherID}, byID[teacherPath].Author)
		require.NotNil(t, byID[catalogPath].Author)
		assert.Equal(t, domain.CatalogProfileID, byID[catalogPath].Author.ID)
		require.Contains(t, byID, orphanExercise)
		assert.Equal(t, domain.ContentExercise, byID[orphanExercise].Content.Kind)
		assert.Nil(t, byID[orphanExercise].Author)
	})

	t.Run("lists no more than the limit", func(t *testing.T) {
		unowned, err := store.ListUnowned(ctx, 2)

		require.NoError(t, err)
		assert.Len(t, unowned, 2)
	})

	t.Run("the backfill records everything once, and a rerun changes nothing", func(t *testing.T) {
		backfill := application.NewOwnershipBackfill(store, commercialEnvironment(t, "test"), func() time.Time { return fixedAt }, uuid.NewString).WithBatchSize(2)

		first, err := backfill.Run(ctx)
		require.NoError(t, err)
		assert.Equal(t, 4, first)
		second, err := backfill.Run(ctx)
		require.NoError(t, err)
		assert.Zero(t, second)

		unowned, err := store.ListUnowned(ctx, 100)
		require.NoError(t, err)
		assert.Empty(t, unowned)
		row, err := client.ContentOwnership.Query().Where(contentownership.ContentID(uuid.MustParse(orphanExercise))).Only(ctx)
		require.NoError(t, err)
		assert.Equal(t, contentownership.OwnerKindUnresolved, row.OwnerKind)
		assert.Nil(t, row.OwnerID)
		assert.Nil(t, row.CreatorID)
		assert.Equal(t, contentownership.SourceReviewedLegacy, row.Source)
	})

	t.Run("recording an item that was recorded meanwhile leaves it as it is", func(t *testing.T) {
		recorder := application.NewOwnershipRecorder(commercialEnvironment(t, "test"), uuid.NewString)
		record := recorder.Native(domain.ContentRef{Kind: domain.ContentPath, ID: teacherPath}, domain.User{ID: admin.ID, Role: domain.RoleAdmin}, fixedAt)

		require.NoError(t, store.RecordBackfilled(ctx, record))

		count, err := client.ContentOwnership.Query().Where(contentownership.ContentID(uuid.MustParse(teacherPath))).Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}
