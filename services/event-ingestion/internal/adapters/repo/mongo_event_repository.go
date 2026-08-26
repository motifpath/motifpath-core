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

// Ping reports whether the underlying MongoDB connection is reachable, for the
// readiness probe.
func (r *MongoEventRepository) Ping(ctx context.Context) error {
	return r.collection.Database().Client().Ping(ctx, nil)
}

// Save inserts event as a new document. Idempotency comes from the unique index on
// event_id, not from an update-style upsert: per ADR-008 the collection is
// append-only, so a duplicate delivery must neither error nor modify the existing
// document — it hits the unique index, and the resulting duplicate-key error is
// treated as success, with alreadyExisted reporting that fact rather than a fresh
// insert (ADR-012 uses this to decide whether to create a new publish_outbox entry).
func (r *MongoEventRepository) Save(ctx context.Context, event domain.TrackingEvent) (receivedAt time.Time, alreadyExisted bool, err error) {
	receivedAt = time.Now().UTC()
	doc := toDocument(event, receivedAt)

	_, err = r.collection.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return receivedAt, true, nil
		}
		return time.Time{}, false, err
	}

	return receivedAt, false, nil
}

// FindByEventID reconstructs the stored event for eventID.
func (r *MongoEventRepository) FindByEventID(ctx context.Context, eventID string) (domain.TrackingEvent, error) {
	var doc eventDocument
	if err := r.collection.FindOne(ctx, bson.D{{Key: "event_id", Value: eventID}}).Decode(&doc); err != nil {
		return nil, err
	}
	return fromDocument(doc)
}
