package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/motifpath/core-domain/internal/domain"
)

// completionStateDocument mirrors the `aggregates` collection schema fixed
// by ADR-011 — the same shape aggregation-worker's
// MongoCompletionStateRepository writes.
type completionStateDocument struct {
	StudentID     string `bson:"student_id"`
	ContentNodeID string `bson:"content_node_id"`
	Status        string `bson:"status"`
}

// MongoCompletionStateReader reads the `aggregates` collection the
// Aggregation Worker owns (ADR-011). It is read-only from this service's
// perspective and never writes to this collection.
type MongoCompletionStateReader struct {
	collection *mongo.Collection
}

func NewMongoCompletionStateReader(db *mongo.Database) *MongoCompletionStateReader {
	return &MongoCompletionStateReader{collection: db.Collection("aggregates")}
}

// Ping reports whether the underlying MongoDB connection is reachable.
func (r *MongoCompletionStateReader) Ping(ctx context.Context) error {
	return r.collection.Database().Client().Ping(ctx, nil)
}

func (r *MongoCompletionStateReader) GetStatuses(ctx context.Context, studentID string, contentNodeIDs []string) (map[string]domain.CompletionStatus, error) {
	result := map[string]domain.CompletionStatus{}
	if len(contentNodeIDs) == 0 {
		return result, nil
	}

	cursor, err := r.collection.Find(ctx, bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "content_node_id", Value: bson.D{{Key: "$in", Value: contentNodeIDs}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc completionStateDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.ContentNodeID] = domain.CompletionStatus(doc.Status)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
