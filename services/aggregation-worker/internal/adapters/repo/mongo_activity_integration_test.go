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

const session1 = "5e551000-0000-4000-8000-000000000001"

func TestMongoPracticeSessionRepository(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	repo := NewMongoPracticeSessionRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))
	start := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)

	_, found, err := repo.Get(ctx, studentA, session1)
	require.NoError(t, err)
	assert.False(t, found)

	t.Run("a session without its start or end round-trips", func(t *testing.T) {
		answeredOnly := domain.PracticeSession{ID: session1, StudentID: studentB, LastEventAt: start}
		require.NoError(t, repo.Put(ctx, answeredOnly))

		got, found, err := repo.Get(ctx, studentB, session1)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, answeredOnly, got)
	})

	t.Run("every field round-trips and a put replaces the session", func(t *testing.T) {
		started := domain.PracticeSession{}.Started(domain.PracticeSessionStart{
			EventID: "e1", StudentID: studentA, PracticeSessionID: session1, OccurredAt: start,
			InstrumentID: guitar, Minutes: 10,
			PlannedItems: []domain.PlannedPracticeItem{{ItemKey: "play_along:" + diagram1, Reason: "due"}},
		})
		require.NoError(t, repo.Put(ctx, started))
		ended := started.Answered(start.Add(3*time.Minute), "").Ended(domain.PracticeSessionEnd{
			EventID: "e2", StudentID: studentA, PracticeSessionID: session1,
			OccurredAt: start.Add(11 * time.Minute), LeftEarly: true, AnsweredCount: 6,
		})
		require.NoError(t, repo.Put(ctx, ended))

		got, found, err := repo.Get(ctx, studentA, session1)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, ended, got)

		count, err := db.Collection("practice_sessions").CountDocuments(ctx, bson.D{{Key: "student_id", Value: studentA}})
		require.NoError(t, err)
		assert.EqualValues(t, 1, count)
	})

	t.Run("practised drills and felt ratings round-trip, and the felt-rated drills are stored for counting", func(t *testing.T) {
		rated := domain.PracticeSession{}.Started(domain.PracticeSessionStart{
			EventID: "e3", StudentID: studentC, PracticeSessionID: session1, OccurredAt: start, Minutes: 5,
		}).Answered(start.Add(time.Minute), "fretboard_cell:name_the_note").Ended(domain.PracticeSessionEnd{
			EventID: "e4", StudentID: studentC, PracticeSessionID: session1, OccurredAt: start.Add(5 * time.Minute), AnsweredCount: 4,
			FeltRatings: []domain.FeltRating{
				{DrillTemplateKey: "fretboard_cell:name_the_note", Felt: domain.FeltHard},
				{DrillTemplateKey: "exercise:image_choice", Felt: domain.FeltEasy},
			},
		})
		require.NoError(t, repo.Put(ctx, rated))

		got, found, err := repo.Get(ctx, studentC, session1)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, rated, got)

		var raw struct {
			FeltRatedTemplates []string `bson:"felt_rated_templates"`
		}
		require.NoError(t, db.Collection("practice_sessions").FindOne(ctx, bson.D{{Key: "student_id", Value: studentC}}).Decode(&raw))
		assert.Equal(t, []string{"fretboard_cell:name_the_note"}, raw.FeltRatedTemplates)
	})
}

func TestMongoLearningActivityRepository(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	repo := NewMongoLearningActivityRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))
	completed := domain.LearningActivity{
		EventID: "e0000000-0000-4000-8000-0000000000c1", StudentID: studentA,
		ContentNodeID: "c0000000-0000-4000-8000-000000000001", CompletedAt: time.Date(2026, 10, 5, 19, 30, 0, 0, time.UTC),
	}

	inserted, err := repo.Insert(ctx, completed)
	require.NoError(t, err)
	assert.True(t, inserted)

	again := completed
	again.EventID = "e0000000-0000-4000-8000-0000000000c2"
	again.CompletedAt = completed.CompletedAt.AddDate(0, 0, 1)
	inserted, err = repo.Insert(ctx, again)
	require.NoError(t, err)
	assert.True(t, inserted, "completing a node again is another completion")

	inserted, err = repo.Insert(ctx, completed)
	require.NoError(t, err)
	assert.False(t, inserted, "a redelivered event is stored once")

	var docs []bson.M
	cursor, err := db.Collection("learning_activity").Find(ctx, bson.D{})
	require.NoError(t, err)
	require.NoError(t, cursor.All(ctx, &docs))
	require.Len(t, docs, 2)
	assert.Equal(t, completed.EventID, docs[0]["event_id"])
	assert.Equal(t, studentA, docs[0]["student_id"])
	assert.Equal(t, completed.ContentNodeID, docs[0]["content_node_id"])
	assert.Equal(t, bson.NewDateTimeFromTime(completed.CompletedAt), docs[0]["completed_at"])
}

func TestMongoTapCheckRepository(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	repo := NewMongoTapCheckRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))
	check := domain.TapCheck{
		EventID: "e0000000-0000-4000-8000-0000000000f1", StudentID: studentA,
		DoneAt: time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC), MedianTapMs: 320, TapCount: 24,
	}

	inserted, err := repo.Insert(ctx, check)
	require.NoError(t, err)
	assert.True(t, inserted)

	again := check
	again.EventID = "e0000000-0000-4000-8000-0000000000f2"
	again.DoneAt = check.DoneAt.AddDate(0, 0, 31)
	inserted, err = repo.Insert(ctx, again)
	require.NoError(t, err)
	assert.True(t, inserted, "doing a tap check again is another tap check")

	inserted, err = repo.Insert(ctx, check)
	require.NoError(t, err)
	assert.False(t, inserted, "a redelivered event is stored once")

	var docs []bson.M
	cursor, err := db.Collection("tap_checks").Find(ctx, bson.D{})
	require.NoError(t, err)
	require.NoError(t, cursor.All(ctx, &docs))
	require.Len(t, docs, 2)
	assert.Equal(t, check.EventID, docs[0]["event_id"])
	assert.Equal(t, studentA, docs[0]["student_id"])
	assert.Equal(t, bson.NewDateTimeFromTime(check.DoneAt), docs[0]["done_at"])
	assert.EqualValues(t, 320, docs[0]["median_tap_ms"])
	assert.EqualValues(t, 24, docs[0]["tap_count"])
}

func TestMongoPracticeItemHistoryRepository(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	repo := NewMongoPracticeItemHistoryRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))
	itemKey := "play_along:" + diagram1
	monday := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	fold := domain.ItemFold{Attempts: 3, Counted: 3, Accuracy: 0.9, Fluency: 0.7, Box: 2, BestCleanBPM: intPtr(90)}

	require.NoError(t, repo.Put(ctx, studentA, itemKey, []domain.ItemSnapshot{{Day: monday, Fold: fold}}))
	better := fold
	better.Accuracy, better.Counted = 0.95, 5
	require.NoError(t, repo.Put(ctx, studentA, itemKey, []domain.ItemSnapshot{
		{Day: monday, Fold: better},
		{Day: monday.AddDate(0, 0, 1), Fold: fold},
	}))

	var docs []bson.M
	cursor, err := db.Collection("practice_item_history").Find(ctx, bson.D{})
	require.NoError(t, err)
	require.NoError(t, cursor.All(ctx, &docs))
	require.Len(t, docs, 2, "a day's snapshot is replaced, not duplicated")

	var mon bson.M
	require.NoError(t, db.Collection("practice_item_history").FindOne(ctx, bson.D{
		{Key: "student_id", Value: studentA}, {Key: "item_key", Value: itemKey}, {Key: "day", Value: monday},
	}).Decode(&mon))
	assert.InDelta(t, 0.95, mon["accuracy"], 1e-9)
	assert.InDelta(t, 0.7, mon["fluency"], 1e-9)
	assert.Equal(t, string(better.Level()), mon["level"])
	assert.EqualValues(t, 90, mon["best_clean_bpm"])
	assert.EqualValues(t, domain.PracticeRulesVersion, mon["rules_version"])
}
