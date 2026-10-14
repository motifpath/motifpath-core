package repo

import (
	"context"
	"errors"
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
		// LatestTap: a student's newest event of one type before a given time.
		{
			Keys: bson.D{{Key: "student_id", Value: 1}, {Key: "event_type", Value: 1}, {Key: "occurred_at", Value: -1}},
		},
		// List: the event log in received order, everyone's or one student's.
		{
			Keys: bson.D{{Key: "received_at", Value: -1}, {Key: "event_id", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "student_id", Value: 1}, {Key: "received_at", Value: -1}, {Key: "event_id", Value: -1}},
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

// List reads one page of the event log, newest received first. event_id
// breaks a tie in received_at, so the order holds across pages.
func (r *MongoEventRepository) List(ctx context.Context, q domain.EventLogQuery) ([]domain.LoggedEvent, int, error) {
	filter := bson.D{}
	if q.StudentID != "" {
		filter = append(filter, bson.E{Key: "student_id", Value: q.StudentID})
	}
	// Events of a retired type stay in the log but can't be read back, so
	// they are left out of the page and of the total alike.
	eventTypes := q.EventTypes
	if len(eventTypes) == 0 {
		eventTypes = domain.ReadableEventTypes()
	}
	types := make([]string, len(eventTypes))
	for i, t := range eventTypes {
		types[i] = string(t)
	}
	filter = append(filter, bson.E{Key: "event_type", Value: bson.D{{Key: "$in", Value: types}}})

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "received_at", Value: -1}, {Key: "event_id", Value: -1}}).
		SetSkip(int64(q.Offset)).
		SetLimit(int64(q.Limit))
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	var docs []eventDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	logged := make([]domain.LoggedEvent, len(docs))
	for i, doc := range docs {
		event, err := fromDocument(doc)
		if err != nil {
			return nil, 0, err
		}
		logged[i] = domain.LoggedEvent{ReceivedAt: doc.ReceivedAt.UTC(), Event: event}
	}
	return logged, int(total), nil
}

// LatestTap returns the median tap of the student's newest practice.tap_check_completed
// event that occurred strictly before the given time. Ordered by occurred_at, the
// student's own clock, not received_at: a tap check done offline and sent late still
// counts from when it was done.
func (r *MongoEventRepository) LatestTap(ctx context.Context, studentID string, before time.Time) (int, bool, error) {
	filter := bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "event_type", Value: string(domain.EventTypePracticeTapCheckCompleted)},
		{Key: "occurred_at", Value: bson.D{{Key: "$lt", Value: before}}},
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "occurred_at", Value: -1}})

	var doc eventDocument
	if err := r.collection.FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if doc.MedianTapMs == nil {
		return 0, false, nil
	}
	return *doc.MedianTapMs, true, nil
}
