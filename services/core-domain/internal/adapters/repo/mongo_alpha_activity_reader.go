package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/core-domain/internal/domain"
)

// alphaSessionDocument holds the fields the alpha observation reads from a
// `practice_sessions` document, in the shape the Aggregation Worker writes
// it: end is null until the session's end arrives, and answered_event_ids
// is null on sessions kept before answers were.
type alphaSessionDocument struct {
	StudentID         string                   `bson:"student_id"`
	PracticeSessionID string                   `bson:"practice_session_id"`
	StartedAt         time.Time                `bson:"started_at"`
	Minutes           int                      `bson:"minutes"`
	LastEventAt       time.Time                `bson:"last_event_at"`
	AnsweredEventIDs  []string                 `bson:"answered_event_ids"`
	End               *alphaSessionEndDocument `bson:"end"`
}

type alphaSessionEndDocument struct {
	EndedAt       time.Time `bson:"ended_at"`
	LeftEarly     bool      `bson:"left_early"`
	AnsweredCount int       `bson:"answered_count"`
}

// firstTimeDocument is one key's earliest time from an aggregation.
type firstTimeDocument struct {
	Key string    `bson:"_id"`
	At  time.Time `bson:"at"`
}

// nodeStartDocument holds the fields read from an `aggregates` document.
type nodeStartDocument struct {
	ContentNodeID string    `bson:"content_node_id"`
	StartedAt     time.Time `bson:"started_at"`
}

// lastActivityDocument is a `student_activity` document.
type lastActivityDocument struct {
	StudentID    string    `bson:"student_id"`
	LastActiveAt time.Time `bson:"last_active_at"`
}

// MongoAlphaActivityReader reads the `practice_sessions`,
// `learning_activity`, `aggregates` and `student_activity` collections the
// Aggregation Worker owns. It never writes to them.
type MongoAlphaActivityReader struct {
	sessions   *mongo.Collection
	learning   *mongo.Collection
	aggregates *mongo.Collection
	activity   *mongo.Collection
}

func NewMongoAlphaActivityReader(db *mongo.Database) *MongoAlphaActivityReader {
	return &MongoAlphaActivityReader{
		sessions:   db.Collection("practice_sessions"),
		learning:   db.Collection("learning_activity"),
		aggregates: db.Collection("aggregates"),
		activity:   db.Collection("student_activity"),
	}
}

func (r *MongoAlphaActivityReader) SessionsStartedBetween(ctx context.Context, studentIDs []string, from, to time.Time) ([]domain.PracticeSessionRecord, error) {
	if len(studentIDs) == 0 {
		return []domain.PracticeSessionRecord{}, nil
	}
	// A null started_at, a session whose start hasn't arrived, never falls
	// in a range.
	cursor, err := r.sessions.Find(ctx, bson.D{
		{Key: "student_id", Value: bson.D{{Key: "$in", Value: studentIDs}}},
		{Key: "started_at", Value: bson.D{{Key: "$gte", Value: from}, {Key: "$lt", Value: to}}},
	})
	if err != nil {
		return nil, err
	}
	var docs []alphaSessionDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	records := make([]domain.PracticeSessionRecord, len(docs))
	for i, d := range docs {
		records[i] = domain.PracticeSessionRecord{
			ID: d.PracticeSessionID, StudentID: d.StudentID, StartedAt: d.StartedAt.UTC(),
			PlannedMinutes: d.Minutes, LastEventAt: d.LastEventAt.UTC(), AnswersArrived: len(d.AnsweredEventIDs),
		}
		if d.End != nil {
			records[i].End = &domain.PracticeSessionRecordEnd{At: d.End.EndedAt.UTC(), LeftEarly: d.End.LeftEarly, AnsweredCount: d.End.AnsweredCount}
		}
	}
	return records, nil
}

// finishedWeekDocument is one student's count of finished sessions in one
// week, the week given by its Monday 00:00 in the alpha's time zone.
type finishedWeekDocument struct {
	Key struct {
		StudentID string    `bson:"student"`
		Week      time.Time `bson:"week"`
	} `bson:"_id"`
	Finished int `bson:"finished"`
}

// FinishedByWeek counts in the database: a session is finished when it has
// an end and wasn't left early, and belongs to the week its start falls in.
func (r *MongoAlphaActivityReader) FinishedByWeek(ctx context.Context, studentIDs []string, from, to time.Time) (map[string]domain.FinishedByWeek, error) {
	counts := map[string]domain.FinishedByWeek{}
	if len(studentIDs) == 0 {
		return counts, nil
	}
	cursor, err := r.sessions.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "student_id", Value: bson.D{{Key: "$in", Value: studentIDs}}},
			{Key: "started_at", Value: bson.D{{Key: "$gte", Value: from}, {Key: "$lt", Value: to}}},
			{Key: "end", Value: bson.D{{Key: "$ne", Value: nil}}},
			{Key: "end.left_early", Value: false},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "student", Value: "$student_id"},
				{Key: "week", Value: bson.D{{Key: "$dateTrunc", Value: bson.D{
					{Key: "date", Value: "$started_at"},
					{Key: "unit", Value: "week"},
					{Key: "startOfWeek", Value: "monday"},
					{Key: "timezone", Value: domain.AlphaTimeZone},
				}}}},
			}},
			{Key: "finished", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	})
	if err != nil {
		return nil, err
	}
	var docs []finishedWeekDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		if counts[d.Key.StudentID] == nil {
			counts[d.Key.StudentID] = domain.FinishedByWeek{}
		}
		counts[d.Key.StudentID][domain.AlphaWeekOf(d.Key.Week).Key()] = d.Finished
	}
	return counts, nil
}

func (r *MongoAlphaActivityReader) FirstCompletions(ctx context.Context, studentID string, contentNodeIDs []string) (map[string]time.Time, error) {
	firsts := map[string]time.Time{}
	if len(contentNodeIDs) == 0 {
		return firsts, nil
	}
	cursor, err := r.learning.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "student_id", Value: studentID},
			{Key: "content_node_id", Value: bson.D{{Key: "$in", Value: contentNodeIDs}}},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$content_node_id"},
			{Key: "at", Value: bson.D{{Key: "$min", Value: "$completed_at"}}},
		}}},
	})
	if err != nil {
		return nil, err
	}
	var docs []firstTimeDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		firsts[d.Key] = d.At.UTC()
	}
	return firsts, nil
}

func (r *MongoAlphaActivityReader) FirstStarts(ctx context.Context, studentID string, contentNodeIDs []string) (map[string]time.Time, error) {
	starts := map[string]time.Time{}
	if len(contentNodeIDs) == 0 {
		return starts, nil
	}
	cursor, err := r.aggregates.Find(ctx, bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "content_node_id", Value: bson.D{{Key: "$in", Value: contentNodeIDs}}},
		{Key: "started_at", Value: bson.D{{Key: "$exists", Value: true}}},
	}, options.Find().SetProjection(bson.D{{Key: "content_node_id", Value: 1}, {Key: "started_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var docs []nodeStartDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		starts[d.ContentNodeID] = d.StartedAt.UTC()
	}
	return starts, nil
}

func (r *MongoAlphaActivityReader) LastActive(ctx context.Context, studentIDs []string) (map[string]time.Time, error) {
	last := map[string]time.Time{}
	if len(studentIDs) == 0 {
		return last, nil
	}
	cursor, err := r.activity.Find(ctx, bson.D{{Key: "student_id", Value: bson.D{{Key: "$in", Value: studentIDs}}}})
	if err != nil {
		return nil, err
	}
	var docs []lastActivityDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		last[d.StudentID] = d.LastActiveAt.UTC()
	}
	return last, nil
}
