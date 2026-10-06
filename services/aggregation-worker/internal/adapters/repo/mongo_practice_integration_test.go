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
	studentA  = "a11ce000-0000-4000-8000-000000000001"
	studentB  = "b0b00000-0000-4000-8000-000000000002"
	studentC  = "c0c00000-0000-4000-8000-000000000003"
	diagram1  = "00000000-0000-4000-8000-0000000000f1"
	diagram2  = "00000000-0000-4000-8000-0000000000f2"
	exercise1 = "00000000-0000-4000-8000-0000000000e1"
	exercise2 = "00000000-0000-4000-8000-0000000000e2"
	guitar    = "6ea2d087-ab9c-59dc-9657-8546025414d2"
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

// shownOption is an exercise option as core keeps it in the reference snapshot.
func shownOption(t *testing.T, id string, correct bool, label string) domain.AnswerOption {
	t.Helper()
	raw, err := bson.Marshal(bson.D{{Key: "option_id", Value: id}, {Key: "is_correct", Value: correct}, {Key: "label", Value: label}})
	require.NoError(t, err)
	return domain.AnswerOption{OptionID: id, IsCorrect: correct, Shown: raw}
}

func TestMongoPracticeReferenceReader_ExerciseOptions(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	_, err := db.Collection("practice_reference").InsertOne(ctx, bson.D{
		{Key: "kind", Value: "exercise"}, {Key: "id", Value: exercise1}, {Key: "exercise_type", Value: "text_response"},
		{Key: "option_ids", Value: bson.A{"o-1", "o-2"}}, {Key: "correct_option_ids", Value: bson.A{"o-2"}}, {Key: "instrument_ids", Value: bson.A{}},
		{Key: "options", Value: bson.A{
			bson.D{{Key: "option_id", Value: "o-1"}, {Key: "is_correct", Value: false}, {Key: "label", Value: "B"}},
			bson.D{{Key: "option_id", Value: "o-2"}, {Key: "is_correct", Value: true}, {Key: "label", Value: "C"}},
		}},
		{Key: "snapshot_version", Value: 2},
	})
	require.NoError(t, err)

	got, err := NewMongoPracticeReferenceReader(db).Exercises(ctx, []string{exercise1})
	require.NoError(t, err)

	assert.Equal(t, []domain.AnswerOption{shownOption(t, "o-1", false, "B"), shownOption(t, "o-2", true, "C")}, got[exercise1].Options,
		"each option keeps its id, its verdict and exactly what core stored")
}

func TestMongoPracticeReferenceReader_Instruments(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	_, err := db.Collection("practice_reference").InsertMany(ctx, []bson.D{
		{{Key: "kind", Value: "instrument"}, {Key: "id", Value: guitar}, {Key: "family", Value: "fretted"}, {Key: "string_count", Value: 6},
			{Key: "tuning", Value: bson.A{"E2", "A2", "D3", "G3", "B3", "E4"}}, {Key: "snapshot_version", Value: 2}},
		{{Key: "kind", Value: "diagram"}, {Key: "id", Value: diagram1}, {Key: "instrument_ids", Value: bson.A{}}, {Key: "snapshot_version", Value: 1}},
	})
	require.NoError(t, err)
	reader := NewMongoPracticeReferenceReader(db)

	got, err := reader.Instruments(ctx, []string{guitar, diagram1})
	require.NoError(t, err)

	assert.Equal(t, map[string]domain.InstrumentReference{guitar: {ID: guitar, Tuning: []string{"E2", "A2", "D3", "G3", "B3", "E4"}}}, got)

	t.Run("no ids reads nothing", func(t *testing.T) {
		got, err := reader.Instruments(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestMongoPracticeReferenceReader_Exercises(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	// The documents as core writes them: an empty instrument list means every instrument.
	_, err := db.Collection("practice_reference").InsertMany(ctx, []bson.D{
		{{Key: "kind", Value: "exercise"}, {Key: "id", Value: exercise1}, {Key: "exercise_type", Value: "text_response"},
			{Key: "option_ids", Value: bson.A{"o-1", "o-2"}}, {Key: "correct_option_ids", Value: bson.A{"o-2"}},
			{Key: "instrument_ids", Value: bson.A{}}, {Key: "updated_at", Value: time.Now()}, {Key: "snapshot_version", Value: 1}},
		{{Key: "kind", Value: "diagram"}, {Key: "id", Value: exercise2}, {Key: "instrument_ids", Value: bson.A{}}, {Key: "snapshot_version", Value: 1}},
	})
	require.NoError(t, err)
	reader := NewMongoPracticeReferenceReader(db)

	got, err := reader.Exercises(ctx, []string{exercise1, exercise2})
	require.NoError(t, err)

	assert.Equal(t, map[string]domain.ExerciseReference{
		exercise1: {ID: exercise1, ExerciseType: "text_response", OptionIDs: []string{"o-1", "o-2"}, CorrectOptionIDs: []string{"o-2"}, InstrumentIDs: []string{}},
	}, got, "a diagram with the same id is not an exercise")

	t.Run("no ids reads nothing", func(t *testing.T) {
		got, err := reader.Exercises(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestMongoPracticeReferenceReader_FluentTimes(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	october := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	november := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	_, err := db.Collection("practice_reference").InsertMany(ctx, []bson.D{
		{{Key: "kind", Value: "drill_threshold"}, {Key: "id", Value: "th-2"}, {Key: "template_key", Value: "exercise:text_response"},
			{Key: "version", Value: 2}, {Key: "effective_from", Value: november}, {Key: "fluent_net_ms", Value: 4500}, {Key: "source", Value: "benchmark"}},
		{{Key: "kind", Value: "drill_threshold"}, {Key: "id", Value: "th-1"}, {Key: "template_key", Value: "exercise:text_response"},
			{Key: "version", Value: 1}, {Key: "effective_from", Value: october}, {Key: "fluent_net_ms", Value: 6000}, {Key: "source", Value: "default"}},
		{{Key: "kind", Value: "drill_threshold"}, {Key: "id", Value: "th-3"}, {Key: "template_key", Value: "exercise:image_choice"},
			{Key: "version", Value: 1}, {Key: "effective_from", Value: october}, {Key: "fluent_net_ms", Value: 5000}, {Key: "source", Value: "default"}},
	})
	require.NoError(t, err)
	reader := NewMongoPracticeReferenceReader(db)

	got, err := reader.FluentTimes(ctx, "exercise:text_response")
	require.NoError(t, err)

	assert.Equal(t, []domain.FluentTime{
		{Version: 1, EffectiveFrom: october, FluentNetMs: 6000},
		{Version: 2, EffectiveFrom: november, FluentNetMs: 4500},
	}, got)

	t.Run("a template with no versions has none", func(t *testing.T) {
		got, err := reader.FluentTimes(ctx, "fretboard_cell:name_the_note")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
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
		AnswerKey: &domain.AnswerKey{String: intPtr(6), Fret: intPtr(0), NoteName: "E"},
	}
	challenge := domain.PracticeEvidence{
		EvidenceID: "e0000000-0000-4000-8000-000000000005", StudentID: studentA,
		ItemKey: "exercise:" + exercise1, Source: domain.EvidenceSourceAutoGraded, OccurredAt: monday,
		TriggerContext: &domain.TriggerContext{Source: "challenge_sequence", ChallengeID: "c4a11e00-0000-4000-8000-000000000001", ContentNodeID: "c0de0000-0000-4000-8000-000000000001"},
		GraderID:       "exercise_option.v1",
		Response:       domain.PracticeResponse{Type: domain.PracticeResponseOptionChoice, OptionIDs: []string{"o-2"}, LatencyMs: intPtr(9500), AudioMs: intPtr(5000)},
		Correct:        boolPtr(true), LatencyMs: intPtr(9500), AudioMs: intPtr(5000),
		AnswerKey: &domain.AnswerKey{Options: []domain.AnswerOption{
			shownOption(t, "o-1", false, "B"), shownOption(t, "o-2", true, "C"),
		}},
	}
	for _, e := range []domain.PracticeEvidence{later, earlier, other, timed, challenge} {
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

	t.Run("a challenge answer keeps its trigger context and audio length", func(t *testing.T) {
		got, err := repo.ListForItem(ctx, studentA, challenge.ItemKey)
		require.NoError(t, err)
		assert.Equal(t, []domain.PracticeEvidence{challenge}, got)
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
	first := domain.ItemFold{Attempts: 1, Counted: 1, Accuracy: 1, Fluency: 0.9, Box: 1, DueAt: &due, LastAt: &last, BestCleanBPM: intPtr(90),
		RightByResponse: map[domain.PracticeResponseType]int{domain.PracticeResponseNameTheNote: 2, domain.PracticeResponseFindTheNote: 1}}
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
