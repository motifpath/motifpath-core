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
		"snapshot_version": int32(2),
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
		"options":            bson.A{},
		"updated_at":         bson.NewDateTimeFromTime(at),
		"snapshot_version":   int32(2),
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
		"snapshot_version": int32(2),
	}, doc)

	t.Run("an empty write stores nothing and succeeds", func(t *testing.T) {
		require.NoError(t, writer.PutDrillThresholds(ctx, nil))
	})
}

// TestMongoPracticeReferenceWriter_ExerciseOptions pins how an exercise's
// options are kept: each in the shape the API shows it, so a grader can copy
// what the student saw into the evidence as it is.
func TestMongoPracticeReferenceWriter_ExerciseOptions(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	writer := NewMongoPracticeReferenceWriter(db, func() time.Time { return time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC) })
	label, audio, diagramID, positionID := "C", "https://cdn.example.com/c.mp3", "d-1", "p-1"
	require.NoError(t, writer.PutExercises(ctx, []domain.ExerciseReference{{
		ID: "e-1", ExerciseType: domain.ExerciseTypeImageRecognition, OptionIDs: []string{"o-1", "o-2", "o-3"}, CorrectOptionIDs: []string{"o-1"},
		Options: []domain.Option{
			{ID: "o-1", IsCorrect: true, Label: &label, AudioURL: &audio},
			{ID: "o-2", Region: &domain.OptionRegion{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4, Shape: domain.OptionRegionShapeCircle}},
			{ID: "o-3", DiagramID: &diagramID, DiagramPositionID: &positionID, FretCell: &domain.FretCell{String: 5, Fret: 3},
				DiagramRef: &domain.DiagramRef{DiagramID: "d-2"}},
		},
	}}))

	var doc struct {
		Options []bson.M `bson:"options"`
	}
	require.NoError(t, db.Collection("practice_reference").FindOne(ctx, bson.D{{Key: "kind", Value: "exercise"}, {Key: "id", Value: "e-1"}}).Decode(&doc))

	require.Len(t, doc.Options, 3)
	assert.Equal(t, bson.M{"option_id": "o-1", "is_correct": true, "label": "C", "audio_url": audio}, doc.Options[0])
	assert.Equal(t, bson.M{"option_id": "o-2", "is_correct": false,
		"region": bson.D{{Key: "x", Value: 0.1}, {Key: "y", Value: 0.2}, {Key: "width", Value: 0.3}, {Key: "height", Value: 0.4}, {Key: "shape", Value: "circle"}}}, doc.Options[1])
	assert.Equal(t, "o-3", doc.Options[2]["option_id"])
	assert.Equal(t, "d-1", doc.Options[2]["diagram_id"])
	assert.Equal(t, "p-1", doc.Options[2]["diagram_position_id"])
	assert.Equal(t, bson.D{{Key: "string", Value: int32(5)}, {Key: "fret", Value: int32(3)}}, doc.Options[2]["fret_cell"])
	ref, ok := doc.Options[2]["diagram_ref"].(bson.D)
	require.True(t, ok, "the diagram ref is a document, as the API shows it")
	assert.Contains(t, ref, bson.E{Key: "diagram_id", Value: "d-2"})
}

// TestMongoPracticeReferenceWriter_InstrumentShape pins the instrument
// document the fretboard grader reads: its family, strings and tuning.
func TestMongoPracticeReferenceWriter_InstrumentShape(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	at := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	writer := NewMongoPracticeReferenceWriter(db, func() time.Time { return at })
	require.NoError(t, writer.EnsureIndexes(ctx))
	six := 6

	require.NoError(t, writer.PutInstruments(ctx, []domain.InstrumentReference{
		{ID: "i-guitar", Family: domain.InstrumentFamilyFretted, StringCount: &six, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}},
		{ID: "i-piano", Family: domain.InstrumentFamilyKeyboard},
	}))

	var doc bson.M
	require.NoError(t, db.Collection("practice_reference").FindOne(ctx, bson.D{{Key: "kind", Value: "instrument"}, {Key: "id", Value: "i-guitar"}}).Decode(&doc))
	delete(doc, "_id")
	assert.Equal(t, bson.M{
		"kind":             "instrument",
		"id":               "i-guitar",
		"family":           "fretted",
		"string_count":     int32(6),
		"tuning":           bson.A{"E2", "A2", "D3", "G3", "B3", "E4"},
		"updated_at":       bson.NewDateTimeFromTime(at),
		"snapshot_version": int32(2),
	}, doc)

	t.Run("an instrument without strings stores none and an empty tuning", func(t *testing.T) {
		var got bson.M
		require.NoError(t, db.Collection("practice_reference").FindOne(ctx, bson.D{{Key: "kind", Value: "instrument"}, {Key: "id", Value: "i-piano"}}).Decode(&got))
		assert.Nil(t, got["string_count"])
		assert.Equal(t, bson.A{}, got["tuning"])
	})

	t.Run("an empty write stores nothing and succeeds", func(t *testing.T) {
		require.NoError(t, writer.PutInstruments(ctx, nil))
	})
}
