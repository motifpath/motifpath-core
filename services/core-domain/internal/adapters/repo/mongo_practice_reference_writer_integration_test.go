//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/motifpath/core-domain/internal/domain"
)

// TestMongoPracticeReferenceWriter_DiagramShape pins the document shape
// ADR-047 fixes for the graders: one document per {kind, id}, upserted in
// place, with tempo_bpm null for a diagram without playback.
func TestMongoPracticeReferenceWriter_DiagramShape(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	at := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	writer := NewMongoPracticeReferenceWriter(db, func() time.Time { return at })
	require.NoError(t, writer.EnsureIndexes(ctx))

	tempo := 90
	require.NoError(t, writer.PutDiagrams(ctx, []domain.DiagramReference{
		{ID: "d-1", InstrumentIDs: []string{"guitar", "electric"}, TempoBPM: &tempo},
		{ID: "d-2", InstrumentIDs: []string{"guitar"}},
	}))

	var doc bson.M
	require.NoError(t, db.Collection("practice_reference").FindOne(ctx, bson.D{{Key: "kind", Value: "diagram"}, {Key: "id", Value: "d-1"}}).Decode(&doc))
	delete(doc, "_id")
	assert.Equal(t, bson.M{
		"kind":             "diagram",
		"id":               "d-1",
		"instrument_ids":   bson.A{"guitar", "electric"},
		"tempo_bpm":        int32(90),
		"updated_at":       bson.NewDateTimeFromTime(at),
		"snapshot_version": int32(1),
	}, doc)

	t.Run("every reference in one write is stored", func(t *testing.T) {
		count, err := db.Collection("practice_reference").CountDocuments(ctx, bson.D{{Key: "kind", Value: "diagram"}})
		require.NoError(t, err)
		assert.EqualValues(t, 2, count)
	})

	t.Run("an empty write stores nothing and succeeds", func(t *testing.T) {
		require.NoError(t, writer.PutDiagrams(ctx, nil))
	})

	t.Run("a second write replaces the document in place", func(t *testing.T) {
		require.NoError(t, writer.PutDiagrams(ctx, []domain.DiagramReference{{ID: "d-1", InstrumentIDs: []string{"guitar"}}}))

		count, err := db.Collection("practice_reference").CountDocuments(ctx, bson.D{{Key: "kind", Value: "diagram"}, {Key: "id", Value: "d-1"}})
		require.NoError(t, err)
		assert.EqualValues(t, 1, count)
		var got bson.M
		require.NoError(t, db.Collection("practice_reference").FindOne(ctx, bson.D{{Key: "id", Value: "d-1"}}).Decode(&got))
		assert.Nil(t, got["tempo_bpm"], "no playback → tempo_bpm null")
		assert.Equal(t, bson.A{"guitar"}, got["instrument_ids"])
	})

	t.Run("{kind, id} is unique", func(t *testing.T) {
		_, err := db.Collection("practice_reference").InsertOne(ctx, bson.D{{Key: "kind", Value: "diagram"}, {Key: "id", Value: "d-1"}})
		assert.True(t, mongo.IsDuplicateKeyError(err), "want a duplicate key error, got %v", err)
	})
}
