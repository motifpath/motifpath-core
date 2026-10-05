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

// TestMongoPracticeReferenceWriter_ExerciseShape pins the exercise document
// the exercise_option grader reads: its type, every option, the correct ones
// and its instruments, with an empty list (never null) for every instrument.
func TestMongoPracticeReferenceWriter_ExerciseShape(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	at := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	writer := NewMongoPracticeReferenceWriter(db, func() time.Time { return at })
	require.NoError(t, writer.EnsureIndexes(ctx))

	require.NoError(t, writer.PutExercises(ctx, []domain.ExerciseReference{
		{ID: "e-1", ExerciseType: domain.ExerciseTypeTextResponse, OptionIDs: []string{"o-1", "o-2"}, CorrectOptionIDs: []string{"o-2"}, InstrumentIDs: []string{"guitar"}},
		{ID: "e-2", ExerciseType: domain.ExerciseTypeAudioSelection, OptionIDs: []string{"o-3"}, CorrectOptionIDs: []string{"o-3"}, InstrumentIDs: []string{}},
	}))

	var doc bson.M
	require.NoError(t, db.Collection("practice_reference").FindOne(ctx, bson.D{{Key: "kind", Value: "exercise"}, {Key: "id", Value: "e-1"}}).Decode(&doc))
	delete(doc, "_id")
	assert.Equal(t, bson.M{
		"kind":               "exercise",
		"id":                 "e-1",
		"exercise_type":      "text_response",
		"option_ids":         bson.A{"o-1", "o-2"},
		"correct_option_ids": bson.A{"o-2"},
		"instrument_ids":     bson.A{"guitar"},
		"updated_at":         bson.NewDateTimeFromTime(at),
		"snapshot_version":   int32(1),
	}, doc)

	t.Run("an exercise for every instrument stores an empty list", func(t *testing.T) {
		var got bson.M
		require.NoError(t, db.Collection("practice_reference").FindOne(ctx, bson.D{{Key: "kind", Value: "exercise"}, {Key: "id", Value: "e-2"}}).Decode(&got))
		assert.Equal(t, bson.A{}, got["instrument_ids"])
	})

	t.Run("an empty write stores nothing and succeeds", func(t *testing.T) {
		require.NoError(t, writer.PutExercises(ctx, nil))
	})

	t.Run("a second write replaces the document in place", func(t *testing.T) {
		require.NoError(t, writer.PutExercises(ctx, []domain.ExerciseReference{
			{ID: "e-1", ExerciseType: domain.ExerciseTypeTextResponse, OptionIDs: []string{"o-1", "o-2"}, CorrectOptionIDs: []string{"o-1"}, InstrumentIDs: []string{}},
		}))

		count, err := db.Collection("practice_reference").CountDocuments(ctx, bson.D{{Key: "kind", Value: "exercise"}, {Key: "id", Value: "e-1"}})
		require.NoError(t, err)
		assert.EqualValues(t, 1, count)
		var got bson.M
		require.NoError(t, db.Collection("practice_reference").FindOne(ctx, bson.D{{Key: "kind", Value: "exercise"}, {Key: "id", Value: "e-1"}}).Decode(&got))
		assert.Equal(t, bson.A{"o-1"}, got["correct_option_ids"])
	})
}

// TestMongoPracticeReferenceWriter_DrillThresholdShape pins the fluent time
// document the worker judges timed answers by: one per version, under its
// template's key, in force from effective_from.
func TestMongoPracticeReferenceWriter_DrillThresholdShape(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	at := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	writer := NewMongoPracticeReferenceWriter(db, func() time.Time { return at })
	require.NoError(t, writer.EnsureIndexes(ctx))
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	require.NoError(t, writer.PutDrillThresholds(ctx, []domain.DrillThreshold{
		{ID: "th-1", TemplateKey: "exercise:text_response", Version: 1, EffectiveFrom: from, FluentNetMs: 6000, Source: domain.DrillThresholdSourceDefault},
	}))

	var doc bson.M
	require.NoError(t, db.Collection("practice_reference").FindOne(ctx, bson.D{{Key: "kind", Value: "drill_threshold"}, {Key: "id", Value: "th-1"}}).Decode(&doc))
	delete(doc, "_id")
	assert.Equal(t, bson.M{
		"kind":             "drill_threshold",
		"id":               "th-1",
		"template_key":     "exercise:text_response",
		"version":          int32(1),
		"effective_from":   bson.NewDateTimeFromTime(from),
		"fluent_net_ms":    int32(6000),
		"source":           "default",
		"updated_at":       bson.NewDateTimeFromTime(at),
		"snapshot_version": int32(1),
	}, doc)

	t.Run("an empty write stores nothing and succeeds", func(t *testing.T) {
		require.NoError(t, writer.PutDrillThresholds(ctx, nil))
	})
}
