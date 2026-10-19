package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// MongoSongChartCompletionRepository keeps every time a student marked a song
// chart as played in the `song_chart_completions` collection, append-only.
type MongoSongChartCompletionRepository struct {
	collection *mongo.Collection
}

func NewMongoSongChartCompletionRepository(db *mongo.Database) *MongoSongChartCompletionRepository {
	return &MongoSongChartCompletionRepository{collection: db.Collection("song_chart_completions")}
}

// EnsureIndexes creates the unique event_id index that keeps a redelivered
// completion once, and the index a student's completions are read by. It is
// idempotent.
func (r *MongoSongChartCompletionRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "event_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "student_id", Value: 1}, {Key: "completed_at", Value: -1}}},
	})
	return err
}

func (r *MongoSongChartCompletionRepository) Insert(ctx context.Context, c domain.SongChartCompletion) (bool, error) {
	_, err := r.collection.InsertOne(ctx, songChartCompletionDocument{
		EventID:     c.EventID,
		StudentID:   c.StudentID,
		SongChartID: c.SongChartID,
		CompletedAt: c.CompletedAt,
	})
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}
