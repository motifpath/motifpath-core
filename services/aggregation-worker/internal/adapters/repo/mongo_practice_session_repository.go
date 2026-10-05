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

// MongoPracticeSessionRepository keeps each practice session's raw record in the
// `practice_sessions` collection. Readers derive whether a session was abandoned.
type MongoPracticeSessionRepository struct {
	collection *mongo.Collection
	now        func() time.Time
}

func NewMongoPracticeSessionRepository(db *mongo.Database) *MongoPracticeSessionRepository {
	return &MongoPracticeSessionRepository{collection: db.Collection("practice_sessions"), now: time.Now}
}

// EnsureIndexes creates the unique (student_id, practice_session_id) index, and
// the index a student's sessions are read by, newest first. It is idempotent.
func (r *MongoPracticeSessionRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "student_id", Value: 1}, {Key: "practice_session_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "student_id", Value: 1}, {Key: "last_event_at", Value: -1}}},
	})
	return err
}

func (r *MongoPracticeSessionRepository) Get(ctx context.Context, studentID, sessionID string) (domain.PracticeSession, bool, error) {
	var doc practiceSessionDocument
	err := r.collection.FindOne(ctx, sessionFilter(studentID, sessionID)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.PracticeSession{}, false, nil
	}
	if err != nil {
		return domain.PracticeSession{}, false, err
	}
	return doc.toDomain(), true, nil
}

func (r *MongoPracticeSessionRepository) Put(ctx context.Context, s domain.PracticeSession) error {
	doc := toSessionDocument(s)
	doc.UpdatedAt = r.now().UTC()
	_, err := r.collection.ReplaceOne(ctx, sessionFilter(s.StudentID, s.ID), doc, options.Replace().SetUpsert(true))
	return err
}

func sessionFilter(studentID, sessionID string) bson.D {
	return bson.D{{Key: "student_id", Value: studentID}, {Key: "practice_session_id", Value: sessionID}}
}
