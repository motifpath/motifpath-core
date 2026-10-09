package repo

import (
	"context"
	"errors"
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

// snapshotReach is how far past a time a snapshot's day may end and still
// stand for the state at that time: half a day, so the day whose end is
// nearest is the one taken.
const snapshotReach = snapshotDay / 2

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

// songChartCompletionDocument holds the fields this service reads from a
// `song_chart_completions` document.
type songChartCompletionDocument struct {
	SongChartID string    `bson:"song_chart_id"`
	CompletedAt time.Time `bson:"completed_at"`
}

// tapCheckDocument holds the fields this service reads from a
// `tap_checks` document.
type tapCheckDocument struct {
	DoneAt time.Time `bson:"done_at"`
}

// MongoPracticeActivityReader reads the `practice_sessions`,
// `learning_activity`, `practice_item_history`, `tap_checks` and
// `song_chart_completions` collections the Aggregation Worker owns. It
// never writes to them.
type MongoPracticeActivityReader struct {
	sessions   *mongo.Collection
	learning   *mongo.Collection
	history    *mongo.Collection
	tapChecks  *mongo.Collection
	songCharts *mongo.Collection
}

func NewMongoPracticeActivityReader(db *mongo.Database) *MongoPracticeActivityReader {
	return &MongoPracticeActivityReader{
		sessions:   db.Collection("practice_sessions"),
		learning:   db.Collection("learning_activity"),
		history:    db.Collection("practice_item_history"),
		tapChecks:  db.Collection("tap_checks"),
		songCharts: db.Collection("song_chart_completions"),
	}
}

// feltRatedCountDocument is one template's count from FeltRatedSessions'
// aggregation.
type feltRatedCountDocument struct {
	TemplateKey string `bson:"_id"`
	Sessions    int    `bson:"sessions"`
}

// FeltRatedSessions counts, per template, the sessions whose
// felt_rated_templates hold it: the worker keeps only the felt ratings of
// templates a session practised there, each once.
func (r *MongoPracticeActivityReader) FeltRatedSessions(ctx context.Context, templateKeys []string) (map[string]int, error) {
	counts := map[string]int{}
	if len(templateKeys) == 0 {
		return counts, nil
	}
	asked := bson.D{{Key: "felt_rated_templates", Value: bson.D{{Key: "$in", Value: templateKeys}}}}
	cursor, err := r.sessions.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: asked}},
		{{Key: "$unwind", Value: "$felt_rated_templates"}},
		{{Key: "$match", Value: asked}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$felt_rated_templates"},
			{Key: "sessions", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc feltRatedCountDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		counts[doc.TemplateKey] = doc.Sessions
	}
	return counts, cursor.Err()
}

// LastTapCheck reads the student's newest tap check by done_at, the
// student's own clock, on the worker's student_id + done_at index.
func (r *MongoPracticeActivityReader) LastTapCheck(ctx context.Context, studentID string) (time.Time, bool, error) {
	var doc tapCheckDocument
	err := r.tapChecks.FindOne(ctx,
		bson.D{{Key: "student_id", Value: studentID}},
		options.FindOne().SetSort(bson.D{{Key: "done_at", Value: -1}}),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return doc.DoneAt.UTC(), true, nil
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
		{Key: "day", Value: bson.D{{Key: "$lte", Value: at.Add(snapshotReach - snapshotDay)}}},
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

// SongChartCompletions reads every played mark of studentID's, on the
// worker's student_id + completed_at index.
func (r *MongoPracticeActivityReader) SongChartCompletions(ctx context.Context, studentID string) ([]domain.SongChartCompletion, error) {
	cursor, err := r.songCharts.Find(ctx, bson.D{{Key: "student_id", Value: studentID}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var completions []domain.SongChartCompletion
	for cursor.Next(ctx) {
		var doc songChartCompletionDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		completions = append(completions, domain.SongChartCompletion{SongChartID: doc.SongChartID, CompletedAt: doc.CompletedAt.UTC()})
	}
	return completions, cursor.Err()
}
