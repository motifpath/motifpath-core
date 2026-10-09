//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/motifpath/core-domain/internal/domain"
)

// TestMongoPracticeActivityReader_ReadsAggregationWorkerShape verifies the
// reader reads documents in exactly the shapes the Aggregation Worker
// writes them, null fields included; the worker's own write paths are
// covered by its integration tests.
func TestMongoPracticeActivityReader_ReadsAggregationWorkerShape(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }
	reader := NewMongoPracticeActivityReader(db)

	t.Run("finished sessions are those ended since, without leaving early", func(t *testing.T) {
		session := func(studentID, id string, instrumentID *string, end bson.D) bson.D {
			doc := bson.D{
				{Key: "student_id", Value: studentID},
				{Key: "practice_session_id", Value: id},
				{Key: "started_at", Value: at(3, 9)},
				{Key: "minutes", Value: 10},
				{Key: "planned_items", Value: bson.A{bson.D{{Key: "item_key", Value: "play_along:d1"}, {Key: "reason", Value: "new"}}}},
				{Key: "last_event_at", Value: at(3, 9)},
				{Key: "updated_at", Value: at(3, 9)},
			}
			if instrumentID != nil {
				doc = append(doc, bson.E{Key: "instrument_id", Value: *instrumentID})
			}
			if end == nil {
				return append(doc, bson.E{Key: "end", Value: nil})
			}
			return append(doc, bson.E{Key: "end", Value: end})
		}
		end := func(endedAt time.Time, leftEarly bool) bson.D {
			return bson.D{{Key: "event_id", Value: "e"}, {Key: "ended_at", Value: endedAt}, {Key: "left_early", Value: leftEarly}, {Key: "answered_count", Value: 3}}
		}
		guitar := "guitar"
		_, err := db.Collection("practice_sessions").InsertMany(ctx, []any{
			session("alice", "finished", &guitar, end(at(4, 10), false)),
			session("alice", "in-the-head", nil, end(at(4, 11), false)),
			session("alice", "left-early", &guitar, end(at(4, 12), true)),
			session("alice", "never-ended", &guitar, nil),
			session("alice", "too-old", &guitar, end(at(1, 10), false)),
			session("bob", "someone-else", &guitar, end(at(4, 10), false)),
		})
		require.NoError(t, err)

		got, err := reader.FinishedSessions(ctx, "alice", at(2, 0))

		require.NoError(t, err)
		assert.ElementsMatch(t, []domain.FinishedPracticeSession{
			{InstrumentID: &guitar, EndedAt: at(4, 10)},
			{EndedAt: at(4, 11)},
		}, got)
	})

	t.Run("completion times are each completion since", func(t *testing.T) {
		completion := func(eventID, studentID string, completedAt time.Time) bson.D {
			return bson.D{{Key: "event_id", Value: eventID}, {Key: "student_id", Value: studentID}, {Key: "content_node_id", Value: "node-1"}, {Key: "completed_at", Value: completedAt}}
		}
		_, err := db.Collection("learning_activity").InsertMany(ctx, []any{
			completion("c1", "alice", at(3, 10)),
			completion("c2", "alice", at(3, 11)),
			completion("c3", "alice", at(1, 10)),
			completion("c4", "bob", at(3, 10)),
		})
		require.NoError(t, err)

		got, err := reader.CompletionTimes(ctx, "alice", at(2, 0))

		require.NoError(t, err)
		assert.ElementsMatch(t, []time.Time{at(3, 10), at(3, 11)}, got)
	})

	t.Run("a snapshot at a time is each item's day whose end is nearest to it, never more than 12 hours later", func(t *testing.T) {
		snapshot := func(studentID, itemKey string, day time.Time, accuracy float64, bestCleanBPM *int) bson.D {
			return bson.D{
				{Key: "student_id", Value: studentID}, {Key: "item_key", Value: itemKey}, {Key: "day", Value: day},
				{Key: "rules_version", Value: 1}, {Key: "level", Value: "learning"}, {Key: "counted", Value: 2},
				{Key: "accuracy", Value: accuracy}, {Key: "fluency", Value: 0.25}, {Key: "box", Value: 1},
				{Key: "best_clean_bpm", Value: bestCleanBPM}, {Key: "best_changes_per_minute", Value: nil}, {Key: "updated_at", Value: day},
			}
		}
		bpm := 80
		_, err := db.Collection("practice_item_history").InsertMany(ctx, []any{
			snapshot("alice", "play_along:d1", at(1, 0), 0.5, nil),
			snapshot("alice", "play_along:d1", at(2, 0), 0.72, &bpm),
			snapshot("alice", "play_along:d1", at(3, 0), 0.9, &bpm), // its day ends 21 hours after 03:00
			snapshot("alice", "play_along:d2", at(3, 0), 0.4, nil),  // practised only since
			snapshot("alice", "play_along:d3", at(1, 0), 0.6, nil),  // not asked for
			snapshot("bob", "play_along:d1", at(2, 0), 0.1, nil),
		})
		require.NoError(t, err)

		got, err := reader.SnapshotsAt(ctx, "alice", []string{"play_along:d1", "play_along:d2"}, at(3, 3))

		require.NoError(t, err)
		assert.Equal(t, map[string]domain.PracticeItemSnapshot{
			"play_along:d1": {ItemKey: "play_along:d1", Counted: 2, Accuracy: 0.72, Fluency: 0.25, BestCleanBPM: &bpm},
		}, got)

		// 23:00 UTC, when a week begins east of UTC: the day ending an hour
		// later is nearer than the one that ended 23 hours before.
		east, err := reader.SnapshotsAt(ctx, "alice", []string{"play_along:d1"}, at(2, 23))
		require.NoError(t, err)
		assert.InDelta(t, 0.72, east["play_along:d1"].Accuracy, 1e-9)

		none, err := reader.SnapshotsAt(ctx, "alice", nil, at(3, 3))
		require.NoError(t, err)
		assert.Empty(t, none)
	})

	t.Run("song chart completions are every played mark of the student's", func(t *testing.T) {
		completion := func(studentID, eventID, chartID string, completedAt time.Time) bson.D {
			return bson.D{
				{Key: "event_id", Value: eventID},
				{Key: "student_id", Value: studentID},
				{Key: "song_chart_id", Value: chartID},
				{Key: "completed_at", Value: completedAt},
			}
		}
		_, err := db.Collection("song_chart_completions").InsertMany(ctx, []any{
			completion("alice", "s1", "chart-1", at(1, 20)),
			completion("alice", "s2", "chart-1", at(4, 20)),
			completion("alice", "s3", "chart-2", at(5, 20)),
			completion("bob", "s4", "chart-3", at(5, 20)),
		})
		require.NoError(t, err)

		got, err := reader.SongChartCompletions(ctx, "alice")
		require.NoError(t, err)
		assert.ElementsMatch(t, []domain.SongChartCompletion{
			{SongChartID: "chart-1", CompletedAt: at(1, 20)},
			{SongChartID: "chart-1", CompletedAt: at(4, 20)},
			{SongChartID: "chart-2", CompletedAt: at(5, 20)},
		}, got)

		got, err = reader.SongChartCompletions(ctx, "carol")
		require.NoError(t, err)
		assert.Empty(t, got, "a student who never marked a chart as played has none")
	})

	t.Run("the newest tap check is the one with the latest done_at", func(t *testing.T) {
		tapCheck := func(studentID, eventID string, doneAt time.Time) bson.D {
			return bson.D{
				{Key: "event_id", Value: eventID},
				{Key: "student_id", Value: studentID},
				{Key: "done_at", Value: doneAt},
				{Key: "median_tap_ms", Value: 320},
				{Key: "tap_count", Value: 24},
			}
		}
		_, err := db.Collection("tap_checks").InsertMany(ctx, []any{
			tapCheck("alice", "t1", at(1, 9)),
			tapCheck("alice", "t2", at(5, 9)),
			tapCheck("alice", "t3", at(3, 9)),
			tapCheck("bob", "t4", at(6, 9)),
		})
		require.NoError(t, err)

		got, found, err := reader.LastTapCheck(ctx, "alice")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, at(5, 9), got)

		_, found, err = reader.LastTapCheck(ctx, "carol")
		require.NoError(t, err)
		assert.False(t, found, "a student who never did a tap check has none")
	})

	t.Run("felt-rated sessions are counted per drill template asked about", func(t *testing.T) {
		rated := func(id string, templates ...string) bson.D {
			list := bson.A{}
			for _, tpl := range templates {
				list = append(list, tpl)
			}
			return bson.D{
				{Key: "student_id", Value: "alice"},
				{Key: "practice_session_id", Value: "felt-" + id},
				{Key: "practised_templates", Value: list},
				{Key: "felt_rated_templates", Value: list},
				{Key: "end", Value: nil},
			}
		}
		_, err := db.Collection("practice_sessions").InsertMany(ctx, []any{
			rated("1", "fretboard_cell:name_the_note", "exercise:text_response"),
			rated("2", "fretboard_cell:name_the_note"),
			rated("3", "exercise:image_choice"),
			rated("4"),
		})
		require.NoError(t, err)

		got, err := reader.FeltRatedSessions(ctx, []string{"fretboard_cell:name_the_note", "exercise:text_response", "fretboard_cell:find_the_note"})
		require.NoError(t, err)
		assert.Equal(t, map[string]int{"fretboard_cell:name_the_note": 2, "exercise:text_response": 1}, got)

		none, err := reader.FeltRatedSessions(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, none)
	})
}
