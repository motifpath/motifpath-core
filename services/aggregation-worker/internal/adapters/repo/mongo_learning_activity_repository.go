package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// MongoLearningActivityRepository keeps every content node completion in the
// `learning_activity` collection, append-only.
type MongoLearningActivityRepository struct {
	collection *mongo.Collection
}

func NewMongoLearningActivityRepository(db *mongo.Database) *MongoLearningActivityRepository {
	return &MongoLearningActivityRepository{collection: db.Collection("learning_activity")}
}

// EnsureIndexes creates the unique event_id index that keeps a redelivered
// completion once, and the index a student's recent completions are read by. It
// is idempotent.
func (r *MongoLearningActivityRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "event_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "student_id", Value: 1}, {Key: "completed_at", Value: -1}}},
	})
	return err
}

func (r *MongoLearningActivityRepository) Insert(ctx context.Context, a domain.LearningActivity) (bool, error) {
	_, err := r.collection.InsertOne(ctx, learningActivityDocument{
		EventID:       a.EventID,
		StudentID:     a.StudentID,
		ContentNodeID: a.ContentNodeID,
		CompletedAt:   a.CompletedAt,
	})
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}
