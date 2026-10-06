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

// TestMongoPracticeItemStateReader_ReadsAggregationWorkerShape verifies
// GetStates reads documents in exactly the shape the Aggregation Worker's
// evidence processor writes, null fields included; the worker's own write
// path is covered by its integration tests.
func TestMongoPracticeItemStateReader_ReadsAggregationWorkerShape(t *testing.T) {
	ctx := context.Background()
	db := mongoDatabase(t)
	dueAt := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	lastAt := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	stateDoc := func(studentID, itemKey string, fields bson.D) bson.D {
		doc := bson.D{
			{Key: "student_id", Value: studentID},
			{Key: "item_key", Value: itemKey},
			{Key: "rules_version", Value: 1},
			{Key: "attempts", Value: 3},
			{Key: "accuracy", Value: 0.7},
			{Key: "fluency", Value: 0.5},
			{Key: "best_changes_per_minute", Value: nil},
			{Key: "updated_at", Value: lastAt},
		}
		return append(doc, fields...)
	}
	_, err := db.Collection("practice_item_state").InsertMany(ctx, []any{
		stateDoc("alice", "play_along:d1", bson.D{
			{Key: "level", Value: "accurate"}, {Key: "counted", Value: 3}, {Key: "box", Value: 2},
			{Key: "due_at", Value: dueAt}, {Key: "last_at", Value: lastAt}, {Key: "best_clean_bpm", Value: 90},
		}),
		stateDoc("alice", "play_along:d2", bson.D{
			{Key: "level", Value: "new"}, {Key: "counted", Value: 0}, {Key: "box", Value: 0},
			{Key: "due_at", Value: nil}, {Key: "last_at", Value: lastAt}, {Key: "best_clean_bpm", Value: nil},
		}),
		stateDoc("alice", "fretboard_cell:g:6:3", bson.D{
			{Key: "level", Value: "learning"}, {Key: "counted", Value: 5}, {Key: "box", Value: 2},
			{Key: "due_at", Value: dueAt}, {Key: "last_at", Value: lastAt},
			{Key: "right_by_response", Value: bson.D{{Key: "name_the_note", Value: 4}, {Key: "find_the_note", Value: 1}}},
		}),
		// Different student — must never leak into alice's result.
		stateDoc("bob", "play_along:d1", bson.D{
			{Key: "level", Value: "fluent"}, {Key: "counted", Value: 6}, {Key: "box", Value: 4},
			{Key: "due_at", Value: dueAt}, {Key: "last_at", Value: lastAt}, {Key: "best_clean_bpm", Value: 120},
		}),
	})
	require.NoError(t, err)

	reader := NewMongoPracticeItemStateReader(db)

	states, err := reader.GetStates(ctx, "alice", []string{"play_along:d1", "play_along:d2", "play_along:d3", "fretboard_cell:g:6:3"})
	require.NoError(t, err)
	bpm := 90
	assert.Equal(t, map[string]domain.PracticeItemState{
		"play_along:d1": {ItemKey: "play_along:d1", RulesVersion: 1, Level: domain.KnowledgeLevelAccurate, Counted: 3, Box: 2, DueAt: &dueAt, LastAt: &lastAt, Accuracy: 0.7, Fluency: 0.5, BestCleanBPM: &bpm},
		"play_along:d2": {ItemKey: "play_along:d2", RulesVersion: 1, Level: domain.KnowledgeLevelNew, LastAt: &lastAt, Accuracy: 0.7, Fluency: 0.5},
		"fretboard_cell:g:6:3": {
			ItemKey: "fretboard_cell:g:6:3", RulesVersion: 1, Level: domain.KnowledgeLevelLearning, Counted: 5, Box: 2,
			DueAt: &dueAt, LastAt: &lastAt, Accuracy: 0.7, Fluency: 0.5,
			RightByResponse: map[string]int{"name_the_note": 4, "find_the_note": 1},
		},
	}, states)

	none, err := reader.GetStates(ctx, "alice", nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}
