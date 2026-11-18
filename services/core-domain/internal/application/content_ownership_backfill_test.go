package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/application"
	"github.com/motifpath/core-domain/internal/domain"
	"github.com/motifpath/core-domain/internal/ports"
)

// fakeOwnershipBackfillStore holds unowned content and the records the
// backfill stores for it. A stored item is no longer unowned.
type fakeOwnershipBackfillStore struct {
	unowned  []ports.UnownedContent
	recorded []ports.OwnershipRecord
	failOn   string
}

func (f *fakeOwnershipBackfillStore) ListUnowned(_ context.Context, limit int) ([]ports.UnownedContent, error) {
	if len(f.unowned) < limit {
		limit = len(f.unowned)
	}
	return append([]ports.UnownedContent{}, f.unowned[:limit]...), nil
}

func (f *fakeOwnershipBackfillStore) RecordBackfilled(_ context.Context, record ports.OwnershipRecord) error {
	if record.Ownership.Content.ID == f.failOn {
		return errors.New("storage unavailable")
	}
	f.recorded = append(f.recorded, record)
	for i, item := range f.unowned {
		if item.Content == record.Ownership.Content {
			f.unowned = append(f.unowned[:i], f.unowned[i+1:]...)
			break
		}
	}
	return nil
}

var backfillAt = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

func newOwnershipBackfill(t *testing.T, store *fakeOwnershipBackfillStore) *application.OwnershipBackfill {
	t.Helper()
	environment, err := domain.ParseEnvironment("test")
	require.NoError(t, err)
	return application.NewOwnershipBackfill(store, environment, func() time.Time { return backfillAt }, idSequence()).WithBatchSize(2)
}

func TestOwnershipBackfill(t *testing.T) {
	t.Run("attributes every unowned item to its recorded author, in batches", func(t *testing.T) {
		store := &fakeOwnershipBackfillStore{unowned: []ports.UnownedContent{
			{Content: domain.ContentRef{Kind: domain.ContentCourse, ID: "course-1"}, Author: &domain.LegacyAuthor{ID: "admin-1", IsAdmin: true}},
			{Content: domain.ContentRef{Kind: domain.ContentPath, ID: "path-1"}, Author: &domain.LegacyAuthor{ID: "teacher-1"}},
			{Content: domain.ContentRef{Kind: domain.ContentExercise, ID: "exercise-1"}, Author: nil},
		}}

		recorded, err := newOwnershipBackfill(t, store).Run(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 3, recorded)
		assert.Empty(t, store.unowned)
		require.Len(t, store.recorded, 3)
		byID := map[string]domain.ContentOwnership{}
		for _, r := range store.recorded {
			byID[r.Ownership.Content.ID] = r.Ownership
		}
		assert.Equal(t, domain.OwnerPlatform, byID["course-1"].OwnerKind)
		assert.Equal(t, domain.OwnerTeacher, byID["path-1"].OwnerKind)
		assert.Equal(t, domain.OwnerUnresolved, byID["exercise-1"].OwnerKind)
		for _, ownership := range byID {
			assert.Equal(t, domain.SourceReviewedLegacy, ownership.Source)
		}
	})

	t.Run("each record carries its own evidence, attributed to the backfill service", func(t *testing.T) {
		store := &fakeOwnershipBackfillStore{unowned: []ports.UnownedContent{
			{Content: domain.ContentRef{Kind: domain.ContentPath, ID: "path-1"}, Author: &domain.LegacyAuthor{ID: "teacher-1"}},
		}}

		_, err := newOwnershipBackfill(t, store).Run(context.Background())

		require.NoError(t, err)
		record := store.recorded[0]
		evidence := record.Evidence
		assert.Equal(t, record.Ownership.EvidenceEntryID, evidence.AuditEntryID)
		assert.Equal(t, domain.AuditActorService, evidence.Operation.ActorKind)
		assert.Equal(t, application.ActionBackfillContentOwnership, evidence.Operation.Action)
		assert.Equal(t, "path", evidence.Operation.ResourceKind)
		assert.Equal(t, "path-1", evidence.Operation.ResourceID)
		assert.Equal(t, backfillAt, evidence.OccurredAt)
		assert.Contains(t, string(evidence.AfterState), `"source":"reviewed_legacy"`)
	})

	t.Run("nothing unowned records nothing", func(t *testing.T) {
		store := &fakeOwnershipBackfillStore{}

		recorded, err := newOwnershipBackfill(t, store).Run(context.Background())

		require.NoError(t, err)
		assert.Zero(t, recorded)
		assert.Empty(t, store.recorded)
	})

	t.Run("a storage failure stops the run and keeps what was already recorded", func(t *testing.T) {
		store := &fakeOwnershipBackfillStore{failOn: "path-2", unowned: []ports.UnownedContent{
			{Content: domain.ContentRef{Kind: domain.ContentPath, ID: "path-1"}, Author: &domain.LegacyAuthor{ID: "teacher-1"}},
			{Content: domain.ContentRef{Kind: domain.ContentPath, ID: "path-2"}, Author: &domain.LegacyAuthor{ID: "teacher-1"}},
		}}

		recorded, err := newOwnershipBackfill(t, store).Run(context.Background())

		require.Error(t, err)
		assert.Equal(t, 1, recorded)
		require.Len(t, store.recorded, 1)
		assert.Equal(t, "path-1", store.recorded[0].Ownership.Content.ID)
	})
}
