package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/event-ingestion/internal/domain"
)

// MongoEventRepository persists tracking events to the `events` collection per
// ADR-008. The collection is append-only: documents are never updated or deleted.
type MongoEventRepository struct {
	collection *mongo.Collection
}

func NewMongoEventRepository(db *mongo.Database) *MongoEventRepository {
	return &MongoEventRepository{collection: db.Collection("events")}
}

// EnsureIndexes creates the indexes ADR-008 assigns to this service's collection.
// Safe to call on every startup — index creation is idempotent.
func (r *MongoEventRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "event_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "student_id", Value: 1}, {Key: "occurred_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "event_type", Value: 1}, {Key: "occurred_at", Value: -1}},
		},
	})
	return err
}

// Save inserts event as a new document. Idempotency comes from the unique index on
// event_id, not from an update-style upsert: per ADR-008 the collection is
// append-only, so a duplicate delivery must neither error nor modify the existing
// document — it hits the unique index, and the resulting duplicate-key error is
// treated as success.
func (r *MongoEventRepository) Save(ctx context.Context, event domain.TrackingEvent) (time.Time, error) {
	receivedAt := time.Now().UTC()
	doc := toDocument(event, receivedAt)

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return receivedAt, nil
		}
		return time.Time{}, err
	}

	return receivedAt, nil
}
