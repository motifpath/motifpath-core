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

// TestMongoAlphaActivityReader_ReadsAggregationWorkerShape reads documents
// in exactly the shapes the Aggregation Worker writes them, null fields
// included; the worker's own write paths are covered by its integration
// tests.
func TestMongoAlphaActivityReader_ReadsAggregationWorkerShape(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }
	reader := NewMongoAlphaActivityReader(db)

	t.Run("sessions started in the range, in any state, with their answers", func(t *testing.T) {
		session := func(studentID, id string, startedAt *time.Time, lastEventAt time.Time, answered bson.A, end bson.D) bson.D {
			doc := bson.D{
				{Key: "student_id", Value: studentID},
				{Key: "practice_session_id", Value: id},
				{Key: "started_at", Value: startedAt},
				{Key: "minutes", Value: 10},
				{Key: "planned_items", Value: bson.A{}},
				{Key: "practised_templates", Value: nil},
				{Key: "felt_rated_templates", Value: nil},
				{Key: "answered_event_ids", Value: answered},
				{Key: "last_event_at", Value: lastEventAt},
				{Key: "updated_at", Value: lastEventAt},
			}
			if end == nil {
				return append(doc, bson.E{Key: "end", Value: nil})
			}
			return append(doc, bson.E{Key: "end", Value: end})
		}
		end := func(endedAt time.Time, leftEarly bool, answered int) bson.D {
			return bson.D{{Key: "event_id", Value: "e"}, {Key: "ended_at", Value: endedAt}, {Key: "left_early", Value: leftEarly}, {Key: "answered_count", Value: answered}, {Key: "felt_ratings", Value: nil}}
		}
		_, err := db.Collection("practice_sessions").InsertMany(ctx, []any{
			session("alice", "finished", ptrTime(at(5, 9)), at(5, 9).Add(11*time.Minute), bson.A{"a1", "a2"}, end(at(5, 9).Add(11*time.Minute), false, 9)),
			session("alice", "abandoned", ptrTime(at(6, 9)), at(6, 9).Add(3*time.Minute), bson.A{"a3"}, nil),
			session("alice", "before-answers-were-kept", ptrTime(at(7, 9)), at(7, 9), nil, nil),
			session("alice", "no-start-yet", nil, at(7, 10), bson.A{"a4"}, nil),
			session("alice", "too-late", ptrTime(at(12, 9)), at(12, 9), nil, nil),
			session("bob", "bobs", ptrTime(at(5, 10)), at(5, 10), nil, end(at(5, 10).Add(5*time.Minute), true, 2)),
			session("carla", "not-asked", ptrTime(at(5, 10)), at(5, 10), nil, nil),
		})
		require.NoError(t, err)

		got, err := reader.SessionsStartedBetween(ctx, []string{"alice", "bob"}, at(5, 0), at(12, 0))

		require.NoError(t, err)
		assert.ElementsMatch(t, []domain.PracticeSessionRecord{
			{ID: "finished", StudentID: "alice", StartedAt: at(5, 9), PlannedMinutes: 10, LastEventAt: at(5, 9).Add(11 * time.Minute),
				End: &domain.PracticeSessionRecordEnd{At: at(5, 9).Add(11 * time.Minute), AnsweredCount: 9}, AnswersArrived: 2},
			{ID: "abandoned", StudentID: "alice", StartedAt: at(6, 9), PlannedMinutes: 10, LastEventAt: at(6, 9).Add(3 * time.Minute), AnswersArrived: 1},
			{ID: "before-answers-were-kept", StudentID: "alice", StartedAt: at(7, 9), PlannedMinutes: 10, LastEventAt: at(7, 9)},
			{ID: "bobs", StudentID: "bob", StartedAt: at(5, 10), PlannedMinutes: 10, LastEventAt: at(5, 10),
				End: &domain.PracticeSessionRecordEnd{At: at(5, 10).Add(5 * time.Minute), LeftEarly: true, AnsweredCount: 2}},
		}, got)
	})

	t.Run("first completion of each node", func(t *testing.T) {
		completion := func(eventID, nodeID string, completedAt time.Time) bson.D {
			return bson.D{{Key: "event_id", Value: eventID}, {Key: "student_id", Value: "alice"}, {Key: "content_node_id", Value: nodeID}, {Key: "completed_at", Value: completedAt}}
		}
		_, err := db.Collection("learning_activity").InsertMany(ctx, []any{
			completion("c1", "n1", at(6, 9)), completion("c2", "n1", at(5, 9)), completion("c3", "n2", at(7, 9)),
			completion("c4", "n3", at(7, 9)),
			bson.D{{Key: "event_id", Value: "c5"}, {Key: "student_id", Value: "bob"}, {Key: "content_node_id", Value: "n1"}, {Key: "completed_at", Value: at(1, 9)}},
		})
		require.NoError(t, err)

		got, err := reader.FirstCompletions(ctx, "alice", []string{"n1", "n2", "n4"})

		require.NoError(t, err)
		assert.Equal(t, map[string]time.Time{"n1": at(5, 9), "n2": at(7, 9)}, got)
	})

	t.Run("first start of each node, absent where none is kept", func(t *testing.T) {
		state := func(nodeID string, startedAt *time.Time) bson.D {
			doc := bson.D{{Key: "student_id", Value: "alice"}, {Key: "content_node_id", Value: nodeID}, {Key: "status", Value: "in_progress"}, {Key: "updated_at", Value: at(8, 9)}}
			if startedAt != nil {
				doc = append(doc, bson.E{Key: "started_at", Value: *startedAt})
			}
			return doc
		}
		_, err := db.Collection("aggregates").InsertMany(ctx, []any{
			state("n1", ptrTime(at(4, 9))), state("n2", nil), state("n9", ptrTime(at(4, 9))),
		})
		require.NoError(t, err)

		got, err := reader.FirstStarts(ctx, "alice", []string{"n1", "n2", "n3"})

		require.NoError(t, err)
		assert.Equal(t, map[string]time.Time{"n1": at(4, 9)}, got)
	})

	t.Run("last activity of each student, absent with none", func(t *testing.T) {
		_, err := db.Collection("student_activity").InsertMany(ctx, []any{
			bson.D{{Key: "student_id", Value: "alice"}, {Key: "last_active_at", Value: at(9, 21)}},
			bson.D{{Key: "student_id", Value: "carla"}, {Key: "last_active_at", Value: at(9, 22)}},
		})
		require.NoError(t, err)

		got, err := reader.LastActive(ctx, []string{"alice", "bob"})

		require.NoError(t, err)
		assert.Equal(t, map[string]time.Time{"alice": at(9, 21)}, got)
	})
}

func ptrTime(t time.Time) *time.Time { return &t }
