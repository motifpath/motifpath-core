package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoLastActivityRepository keeps when each student's latest event occurred
// in the `student_activity` collection: one document per student.
type MongoLastActivityRepository struct {
	collection *mongo.Collection
}

func NewMongoLastActivityRepository(db *mongo.Database) *MongoLastActivityRepository {
	return &MongoLastActivityRepository{collection: db.Collection("student_activity")}
}

// EnsureIndexes creates the unique student_id index that keeps one document per
// student. It is idempotent.
func (r *MongoLastActivityRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "student_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// Touch raises the student's last activity to at with $max, so events applied
// in any order, or more than once, leave the latest.
func (r *MongoLastActivityRepository) Touch(ctx context.Context, studentID string, at time.Time) error {
	_, err := r.collection.UpdateOne(ctx,
		bson.D{{Key: "student_id", Value: studentID}},
		bson.D{{Key: "$max", Value: bson.D{{Key: "last_active_at", Value: at.UTC()}}}},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}
