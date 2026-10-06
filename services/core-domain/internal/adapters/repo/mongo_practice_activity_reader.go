package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/core-domain/internal/domain"
)

// snapshotDay is how long a `practice_item_history` snapshot's day lasts:
// a snapshot holds the item's state at the end of the UTC day starting at
// its day.
const snapshotDay = 24 * time.Hour

// practiceSessionDocument holds the fields this service reads from a
// `practice_sessions` document, in the shape the Aggregation Worker writes
// it: end is null until the session's end arrives.
type practiceSessionDocument struct {
	InstrumentID string                      `bson:"instrument_id"`
	End          *practiceSessionEndDocument `bson:"end"`
}

type practiceSessionEndDocument struct {
	EndedAt   time.Time `bson:"ended_at"`
	LeftEarly bool      `bson:"left_early"`
}

// learningActivityDocument holds the fields this service reads from a
// `learning_activity` document.
type learningActivityDocument struct {
	CompletedAt time.Time `bson:"completed_at"`
}

// practiceItemSnapshotDocument holds the fields this service reads from a
// `practice_item_history` document.
type practiceItemSnapshotDocument struct {
	ItemKey      string  `bson:"item_key"`
	Counted      int     `bson:"counted"`
	Accuracy     float64 `bson:"accuracy"`
	Fluency      float64 `bson:"fluency"`
	BestCleanBPM *int    `bson:"best_clean_bpm"`
}

// MongoPracticeActivityReader reads the `practice_sessions`,
// `learning_activity` and `practice_item_history` collections the
// Aggregation Worker owns. It never writes to them.
type MongoPracticeActivityReader struct {
	sessions *mongo.Collection
	learning *mongo.Collection
	history  *mongo.Collection
}

func NewMongoPracticeActivityReader(db *mongo.Database) *MongoPracticeActivityReader {
	return &MongoPracticeActivityReader{
		sessions: db.Collection("practice_sessions"),
		learning: db.Collection("learning_activity"),
		history:  db.Collection("practice_item_history"),
	}
}

func (r *MongoPracticeActivityReader) FinishedSessions(ctx context.Context, studentID string, since time.Time) ([]domain.FinishedPracticeSession, error) {
	cursor, err := r.sessions.Find(ctx, bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "end.ended_at", Value: bson.D{{Key: "$gte", Value: since}}},
		{Key: "end.left_early", Value: false},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sessions []domain.FinishedPracticeSession
	for cursor.Next(ctx) {
		var doc practiceSessionDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		session := domain.FinishedPracticeSession{EndedAt: doc.End.EndedAt.UTC()}
		if doc.InstrumentID != "" {
			session.InstrumentID = &doc.InstrumentID
		}
		sessions = append(sessions, session)
	}
	return sessions, cursor.Err()
}

func (r *MongoPracticeActivityReader) CompletionTimes(ctx context.Context, studentID string, since time.Time) ([]time.Time, error) {
	cursor, err := r.learning.Find(ctx, bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "completed_at", Value: bson.D{{Key: "$gte", Value: since}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var times []time.Time
	for cursor.Next(ctx) {
		var doc learningActivityDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		times = append(times, doc.CompletedAt.UTC())
	}
	return times, cursor.Err()
}

func (r *MongoPracticeActivityReader) SnapshotsAt(ctx context.Context, studentID string, itemKeys []string, at time.Time) (map[string]domain.PracticeItemSnapshot, error) {
	result := map[string]domain.PracticeItemSnapshot{}
	if len(itemKeys) == 0 {
		return result, nil
	}

	cursor, err := r.history.Find(ctx, bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "item_key", Value: bson.D{{Key: "$in", Value: itemKeys}}},
		{Key: "day", Value: bson.D{{Key: "$lte", Value: at.Add(-snapshotDay)}}},
	}, options.Find().SetSort(bson.D{{Key: "item_key", Value: 1}, {Key: "day", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc practiceItemSnapshotDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		if _, ok := result[doc.ItemKey]; ok {
			continue
		}
		result[doc.ItemKey] = domain.PracticeItemSnapshot{
			ItemKey: doc.ItemKey, Counted: doc.Counted, Accuracy: doc.Accuracy, Fluency: doc.Fluency, BestCleanBPM: doc.BestCleanBPM,
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
