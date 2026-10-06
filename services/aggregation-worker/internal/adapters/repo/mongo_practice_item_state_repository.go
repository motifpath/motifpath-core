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

// MongoPracticeItemStateRepository keeps each item's folded state in the
// `practice_item_state` collection.
type MongoPracticeItemStateRepository struct {
	collection *mongo.Collection
	now        func() time.Time
}

func NewMongoPracticeItemStateRepository(db *mongo.Database) *MongoPracticeItemStateRepository {
	return &MongoPracticeItemStateRepository{collection: db.Collection("practice_item_state"), now: time.Now}
}

// EnsureIndexes creates the unique (student_id, item_key) index: exactly one
// state per student and item. It is idempotent.
func (r *MongoPracticeItemStateRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "student_id", Value: 1}, {Key: "item_key", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (r *MongoPracticeItemStateRepository) Get(ctx context.Context, studentID, itemKey string) (domain.ItemFold, int, bool, error) {
	var doc practiceItemStateDocument
	err := r.collection.FindOne(ctx, stateFilter(studentID, itemKey)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.ItemFold{}, 0, false, nil
	}
	if err != nil {
		return domain.ItemFold{}, 0, false, err
	}
	return doc.toDomain(), doc.RulesVersion, true, nil
}

func (r *MongoPracticeItemStateRepository) Put(ctx context.Context, studentID, itemKey string, f domain.ItemFold) error {
	doc := practiceItemStateDocument{
		StudentID:            studentID,
		ItemKey:              itemKey,
		RulesVersion:         domain.PracticeRulesVersion,
		Level:                string(f.Level()),
		Attempts:             f.Attempts,
		Counted:              f.Counted,
		Accuracy:             f.Accuracy,
		Fluency:              f.Fluency,
		Box:                  f.Box,
		DueAt:                f.DueAt,
		LastAt:               f.LastAt,
		BestCleanBPM:         f.BestCleanBPM,
		BestChangesPerMinute: f.BestChangesPerMinute,
		RightByResponse:      rightByResponseDocument(f.RightByResponse),
		UpdatedAt:            r.now().UTC(),
	}
	_, err := r.collection.ReplaceOne(ctx, stateFilter(studentID, itemKey), doc, options.Replace().SetUpsert(true))
	return err
}

func stateFilter(studentID, itemKey string) bson.D {
	return bson.D{{Key: "student_id", Value: studentID}, {Key: "item_key", Value: itemKey}}
}
