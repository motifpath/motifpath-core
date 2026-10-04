//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

const (
	studentA = "a11ce000-0000-4000-8000-000000000001"
	studentB = "b0b00000-0000-4000-8000-000000000002"
	diagram1 = "00000000-0000-4000-8000-0000000000f1"
	diagram2 = "00000000-0000-4000-8000-0000000000f2"
	guitar   = "6ea2d087-ab9c-59dc-9657-8546025414d2"
)

func intPtr(v int) *int { return &v }

func boolPtr(v bool) *bool { return &v }

func TestMongoPracticeReferenceReader_Diagrams(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	// The documents as core writes them: tempo_bpm is null for a diagram without playback.
	_, err := db.Collection("practice_reference").InsertMany(ctx, []bson.D{
		{{Key: "kind", Value: "diagram"}, {Key: "id", Value: diagram1}, {Key: "instrument_ids", Value: bson.A{guitar}},
			{Key: "tempo_bpm", Value: 90}, {Key: "updated_at", Value: time.Now()}, {Key: "snapshot_version", Value: 1}},
		{{Key: "kind", Value: "diagram"}, {Key: "id", Value: diagram2}, {Key: "instrument_ids", Value: bson.A{}},
			{Key: "tempo_bpm", Value: nil}, {Key: "updated_at", Value: time.Now()}, {Key: "snapshot_version", Value: 1}},
		{{Key: "kind", Value: "exercise"}, {Key: "id", Value: diagram1}, {Key: "snapshot_version", Value: 1}},
	})
	require.NoError(t, err)
	reader := NewMongoPracticeReferenceReader(db)

	got, err := reader.Diagrams(ctx, []string{diagram1, diagram2, "00000000-0000-4000-8000-0000000000ff"})
	require.NoError(t, err)

	assert.Equal(t, map[string]domain.DiagramReference{
		diagram1: {ID: diagram1, InstrumentIDs: []string{guitar}, TempoBPM: intPtr(90)},
		diagram2: {ID: diagram2, InstrumentIDs: []string{}},
	}, got)
}

func TestMongoPracticeReferenceReader_NoIDsReadsNothing(t *testing.T) {
	got, err := NewMongoPracticeReferenceReader(mongoDatabase(t)).Diagrams(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func takeEvidence(id, student string, at time.Time, rating domain.SelfRating) domain.PracticeEvidence {
	return domain.PracticeEvidence{
		EvidenceID:        id,
		StudentID:         student,
		ItemKey:           "play_along:" + diagram1,
		Source:            domain.EvidenceSourceSelfAssessed,
		OccurredAt:        at,
		PracticeSessionID: "5e551000-0000-4000-8000-000000000001",
		GraderID:          "self_rating.v1",
		Response:          domain.PracticeResponse{Type: domain.PracticeResponseSelfRating, Rating: rating, TempoBPM: intPtr(90)},
		Rating:            rating,
		TempoBPM:          intPtr(90),
	}
}

func TestMongoPracticeEvidenceRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMongoPracticeEvidenceRepository(mongoDatabase(t))
	require.NoError(t, repo.EnsureIndexes(ctx))
	monday := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

	later := takeEvidence("e0000000-0000-4000-8000-000000000002", studentA, monday.Add(time.Hour), domain.SelfRatingClean)
	earlier := takeEvidence("e0000000-0000-4000-8000-000000000001", studentA, monday, domain.SelfRatingStruggled)
	other := takeEvidence("e0000000-0000-4000-8000-000000000003", studentB, monday, domain.SelfRatingClean)
	timed := domain.PracticeEvidence{
		EvidenceID: "e0000000-0000-4000-8000-000000000004", StudentID: studentA,
		ItemKey: "fretboard_cell:" + guitar + ":6:0", Source: domain.EvidenceSourceAutoGraded, OccurredAt: monday,
		GraderID: "fretboard_cell.v1",
		Response: domain.PracticeResponse{Type: domain.PracticeResponseFindTheNote, String: intPtr(6), Fret: intPtr(0), LatencyMs: intPtr(1800)},
		Correct:  boolPtr(true), LatencyMs: intPtr(1800), TapMs: intPtr(350),
	}
	for _, e := range []domain.PracticeEvidence{later, earlier, other, timed} {
		inserted, err := repo.Insert(ctx, e)
		require.NoError(t, err)
		require.True(t, inserted)
	}

	t.Run("a second insert of the same evidence is reported, not stored", func(t *testing.T) {
		inserted, err := repo.Insert(ctx, later)
		require.NoError(t, err)
		assert.False(t, inserted)
	})

	t.Run("an item's evidence lists in the order it was stored", func(t *testing.T) {
		got, err := repo.ListForItem(ctx, studentA, "play_along:"+diagram1)
		require.NoError(t, err)
		assert.Equal(t, []domain.PracticeEvidence{later, earlier}, got)
	})

	t.Run("every field round-trips", func(t *testing.T) {
		got, err := repo.ListForItem(ctx, studentA, timed.ItemKey)
		require.NoError(t, err)
		assert.Equal(t, []domain.PracticeEvidence{timed}, got)
	})
}

func TestMongoPracticeItemStateRepository(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	repo := NewMongoPracticeItemStateRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))
	itemKey := "play_along:" + diagram1

	_, _, found, err := repo.Get(ctx, studentA, itemKey)
	require.NoError(t, err)
	assert.False(t, found)

	due := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	last := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	first := domain.ItemFold{Attempts: 1, Counted: 1, Accuracy: 1, Fluency: 0.9, Box: 1, DueAt: &due, LastAt: &last, BestCleanBPM: intPtr(90)}
	require.NoError(t, repo.Put(ctx, studentA, itemKey, first))
	replaced := first
	replaced.Attempts, replaced.Counted, replaced.Box = 6, 5, 3
	require.NoError(t, repo.Put(ctx, studentA, itemKey, replaced))

	got, version, found, err := repo.Get(ctx, studentA, itemKey)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, replaced, got)
	assert.Equal(t, domain.PracticeRulesVersion, version)

	t.Run("the document carries the rules version and the earned level for its readers", func(t *testing.T) {
		var doc bson.M
		require.NoError(t, db.Collection("practice_item_state").FindOne(ctx, bson.D{
			{Key: "student_id", Value: studentA}, {Key: "item_key", Value: itemKey},
		}).Decode(&doc))
		assert.EqualValues(t, domain.PracticeRulesVersion, doc["rules_version"])
		assert.Equal(t, string(replaced.Level()), doc["level"])
		count, err := db.Collection("practice_item_state").CountDocuments(ctx, bson.D{})
		require.NoError(t, err)
		assert.EqualValues(t, 1, count)
	})
}
