package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/motifpath/event-ingestion/internal/domain"
	"github.com/motifpath/event-ingestion/internal/ports"
)

// MongoPublishOutboxRepository persists publish_outbox entries per ADR-012.
// The collection needs no additional index: EventID is the document's _id,
// which MongoDB already indexes uniquely by default.
type MongoPublishOutboxRepository struct {
	collection *mongo.Collection
}

func NewMongoPublishOutboxRepository(db *mongo.Database) *MongoPublishOutboxRepository {
	return &MongoPublishOutboxRepository{collection: db.Collection("publish_outbox")}
}

// EnsureIndexes creates the index the retry sweep relies on to find due
// entries without a full collection scan. Safe to call on every startup.
func (r *MongoPublishOutboxRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "next_attempt_at", Value: 1}},
	})
	return err
}

func (r *MongoPublishOutboxRepository) Create(ctx context.Context, eventID string) error {
	now := time.Now().UTC()
	doc := outboxDocument{
		EventID:       eventID,
		Status:        string(domain.OutboxStatusPending),
		Attempts:      0,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err := r.collection.InsertOne(ctx, doc)
	return err
}

func (r *MongoPublishOutboxRepository) Get(ctx context.Context, eventID string) (ports.OutboxEntry, bool, error) {
	var doc outboxDocument
	err := r.collection.FindOne(ctx, bson.D{{Key: "_id", Value: eventID}}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ports.OutboxEntry{}, false, nil
		}
		return ports.OutboxEntry{}, false, err
	}
	return toOutboxEntry(doc), true, nil
}

func (r *MongoPublishOutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	_, err := r.collection.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: eventID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: string(domain.OutboxStatusPublished)},
			{Key: "updated_at", Value: time.Now().UTC()},
		}}},
	)
	return err
}

func (r *MongoPublishOutboxRepository) Update(ctx context.Context, entry ports.OutboxEntry) error {
	_, err := r.collection.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: entry.EventID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: string(entry.Status)},
			{Key: "attempts", Value: entry.Attempts},
			{Key: "last_error", Value: entry.LastError},
			{Key: "next_attempt_at", Value: entry.NextAttemptAt},
			{Key: "updated_at", Value: time.Now().UTC()},
		}}},
	)
	return err
}

func (r *MongoPublishOutboxRepository) MarkResolvedManually(ctx context.Context, eventID string, reason string) error {
	_, err := r.collection.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: eventID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: string(domain.OutboxStatusResolvedManually)},
			{Key: "resolution_reason", Value: reason},
			{Key: "updated_at", Value: time.Now().UTC()},
		}}},
	)
	return err
}

func (r *MongoPublishOutboxRepository) ListDueForRetry(ctx context.Context, now time.Time) ([]ports.OutboxEntry, error) {
	cursor, err := r.collection.Find(ctx, bson.D{
		{Key: "status", Value: string(domain.OutboxStatusPending)},
		{Key: "next_attempt_at", Value: bson.D{{Key: "$lte", Value: now}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []outboxDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	entries := make([]ports.OutboxEntry, 0, len(docs))
	for _, doc := range docs {
		entries = append(entries, toOutboxEntry(doc))
	}
	return entries, nil
}

func toOutboxEntry(doc outboxDocument) ports.OutboxEntry {
	return ports.OutboxEntry{
		EventID:       doc.EventID,
		Status:        domain.OutboxStatus(doc.Status),
		Attempts:      doc.Attempts,
		LastError:     doc.LastError,
		NextAttemptAt: doc.NextAttemptAt,
		UpdatedAt:     doc.UpdatedAt,
	}
}
