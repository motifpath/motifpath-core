package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// MongoTapCheckRepository keeps every tap check in the `tap_checks` collection,
// append-only.
type MongoTapCheckRepository struct {
	collection *mongo.Collection
}

func NewMongoTapCheckRepository(db *mongo.Database) *MongoTapCheckRepository {
	return &MongoTapCheckRepository{collection: db.Collection("tap_checks")}
}

// EnsureIndexes creates the unique event_id index that keeps a redelivered tap
// check once, and the index a student's newest tap check is read by. It is
// idempotent.
func (r *MongoTapCheckRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "event_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "student_id", Value: 1}, {Key: "done_at", Value: -1}}},
	})
	return err
}

func (r *MongoTapCheckRepository) Insert(ctx context.Context, c domain.TapCheck) (bool, error) {
	_, err := r.collection.InsertOne(ctx, tapCheckDocument{
		EventID:     c.EventID,
		StudentID:   c.StudentID,
		DoneAt:      c.DoneAt,
		MedianTapMs: c.MedianTapMs,
		TapCount:    c.TapCount,
	})
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}
