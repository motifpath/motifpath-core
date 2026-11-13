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
		ended := started.Answered("answer-1", start.Add(3*time.Minute), "").Ended(domain.PracticeSessionEnd{
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
		}).Answered("answer-1", start.Add(time.Minute), "fretboard_cell:name_the_note").Ended(domain.PracticeSessionEnd{
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
	t.Run("its moments round-trip, by the names core-domain reads them by", func(t *testing.T) {
		threeMinutes, fiveMinutes := 3*60_000, 5*60_000
		seen := domain.PracticeSession{}.Started(domain.PracticeSessionStart{
			EventID: "e5", StudentID: studentD, PracticeSessionID: session1, OccurredAt: start, Minutes: 10,
		}).Seen("answer-1", start.Add(3*time.Minute), &threeMinutes).Seen("answer-2", start.Add(40*time.Minute), nil).
			Ended(domain.PracticeSessionEnd{EventID: "e6", StudentID: studentD, PracticeSessionID: session1,
				OccurredAt: start.Add(50 * time.Minute), VisibleMs: &fiveMinutes})
		require.NoError(t, repo.Put(ctx, seen))

		got, found, err := repo.Get(ctx, studentD, session1)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, seen, got)

		var raw struct {
			Moments []struct {
				At        time.Time `bson:"at"`
				VisibleMs *int64    `bson:"visible_ms"`
			} `bson:"moments"`
		}
		require.NoError(t, db.Collection("practice_sessions").FindOne(ctx, bson.D{{Key: "student_id", Value: studentD}}).Decode(&raw))
		require.Len(t, raw.Moments, 3)
		assert.Equal(t, start.Add(3*time.Minute), raw.Moments[0].At.UTC())
		assert.EqualValues(t, threeMinutes, *raw.Moments[0].VisibleMs)
		assert.Nil(t, raw.Moments[1].VisibleMs, "an answer that didn't measure it keeps none")
	})
}

func TestMongoPracticeSessionRepository_ListsTheSessionsStoredWithoutMoments(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	repo := NewMongoPracticeSessionRepository(db)
	start := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	_, err := db.Collection("practice_sessions").InsertOne(ctx, bson.D{
		{Key: "student_id", Value: studentA}, {Key: "practice_session_id", Value: session1},
		{Key: "started_at", Value: start}, {Key: "minutes", Value: 10},
		{Key: "answered_event_ids", Value: bson.A{"answer-1"}}, {Key: "last_event_at", Value: start.Add(3 * time.Minute)},
	})
	require.NoError(t, err)
	require.NoError(t, repo.Put(ctx, domain.PracticeSession{}.Started(domain.PracticeSessionStart{
		StudentID: studentB, PracticeSessionID: session1, OccurredAt: start, Minutes: 10,
	})))

	older, err := repo.ListWithoutMoments(ctx)

	require.NoError(t, err)
	require.Len(t, older, 1)
	assert.Equal(t, studentA, older[0].StudentID)
	assert.Equal(t, []string{"answer-1"}, older[0].AnsweredEventIDs)
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
	fold := domain.ItemFold{Attempts: 3, Counted: 3, Accuracy: 0.9, Fluency: 0.7, Box: 2, BestCleanBPM: intPtr(90), BestCleanNoteValue: &eighths}

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
	assert.Equal(t, "eighths", mon["best_clean_note_value"])
	assert.EqualValues(t, domain.PracticeRulesVersion, mon["rules_version"])
}

func TestMongoSongChartCompletionRepository(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	repo := NewMongoSongChartCompletionRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))
	completion := domain.SongChartCompletion{
		EventID: "e0000000-0000-4000-8000-0000000000a1", StudentID: studentA,
		SongChartID: "c0000000-0000-4000-8000-0000000000a1", CompletedAt: time.Date(2026, 10, 6, 20, 0, 0, 0, time.UTC),
	}

	inserted, err := repo.Insert(ctx, completion)
	require.NoError(t, err)
	assert.True(t, inserted)

	again := completion
	again.EventID = "e0000000-0000-4000-8000-0000000000a2"
	again.CompletedAt = completion.CompletedAt.AddDate(0, 0, 1)
	inserted, err = repo.Insert(ctx, again)
	require.NoError(t, err)
	assert.True(t, inserted, "marking a chart as played again is another completion")

	inserted, err = repo.Insert(ctx, completion)
	require.NoError(t, err)
	assert.False(t, inserted, "a redelivered event is stored once")

	var docs []bson.M
	cursor, err := db.Collection("song_chart_completions").Find(ctx, bson.D{})
	require.NoError(t, err)
	require.NoError(t, cursor.All(ctx, &docs))
	require.Len(t, docs, 2)
	assert.Equal(t, completion.EventID, docs[0]["event_id"])
	assert.Equal(t, studentA, docs[0]["student_id"])
	assert.Equal(t, completion.SongChartID, docs[0]["song_chart_id"])
	assert.Equal(t, bson.NewDateTimeFromTime(completion.CompletedAt), docs[0]["completed_at"])
}

func TestMongoLastActivityRepository(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	repo := NewMongoLastActivityRepository(db)
	require.NoError(t, repo.EnsureIndexes(ctx))
	evening := time.Date(2026, 10, 5, 21, 15, 0, 0, time.UTC)

	require.NoError(t, repo.Touch(ctx, studentA, evening.Add(-3*time.Hour)))
	require.NoError(t, repo.Touch(ctx, studentA, evening))
	require.NoError(t, repo.Touch(ctx, studentA, evening.Add(-time.Hour)), "a late event")
	require.NoError(t, repo.Touch(ctx, studentB, evening.Add(time.Hour)))

	var docs []bson.M
	cursor, err := db.Collection("student_activity").Find(ctx, bson.D{{Key: "student_id", Value: studentA}})
	require.NoError(t, err)
	require.NoError(t, cursor.All(ctx, &docs))
	require.Len(t, docs, 1, "one document per student")
	assert.Equal(t, bson.NewDateTimeFromTime(evening), docs[0]["last_active_at"], "a late event does not move last activity back")
}
