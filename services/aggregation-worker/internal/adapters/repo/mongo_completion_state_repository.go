package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// MongoCompletionStateRepository persists node-completion status to the
// `aggregates` collection per ADR-011.
type MongoCompletionStateRepository struct {
	collection *mongo.Collection
}

func NewMongoCompletionStateRepository(db *mongo.Database) *MongoCompletionStateRepository {
	return &MongoCompletionStateRepository{collection: db.Collection("aggregates")}
}

// EnsureIndexes creates the unique compound index ADR-011 relies on: exactly
// one document per (student_id, content_node_id) pair. Safe to call on every
// startup — index creation is idempotent.
func (r *MongoCompletionStateRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "student_id", Value: 1}, {Key: "content_node_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// Ping reports whether the underlying MongoDB connection is reachable.
func (r *MongoCompletionStateRepository) Ping(ctx context.Context) error {
	return r.collection.Database().Client().Ping(ctx, nil)
}

func (r *MongoCompletionStateRepository) GetStatus(ctx context.Context, studentID, contentNodeID string) (domain.CompletionStatus, bool, error) {
	var doc completionStateDocument
	err := r.collection.FindOne(ctx, bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "content_node_id", Value: contentNodeID},
	}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", false, nil
		}
		return "", false, err
	}

	return domain.CompletionStatus(doc.Status), true, nil
}

func (r *MongoCompletionStateRepository) Upsert(ctx context.Context, studentID, contentNodeID string, status domain.CompletionStatus) error {
	filter := bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "content_node_id", Value: contentNodeID},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "status", Value: string(status)},
			{Key: "updated_at", Value: time.Now().UTC()},
		}},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}
