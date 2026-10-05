package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// MongoPracticeItemHistoryRepository keeps each practised item's daily snapshots
// in the `practice_item_history` collection.
type MongoPracticeItemHistoryRepository struct {
	collection *mongo.Collection
	now        func() time.Time
}

func NewMongoPracticeItemHistoryRepository(db *mongo.Database) *MongoPracticeItemHistoryRepository {
	return &MongoPracticeItemHistoryRepository{collection: db.Collection("practice_item_history"), now: time.Now}
}

// EnsureIndexes creates the unique (student_id, item_key, day) index: one
// snapshot per item and day. Its student_id prefix also serves reading a
// student's snapshots up to a day. It is idempotent.
func (r *MongoPracticeItemHistoryRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "student_id", Value: 1}, {Key: "item_key", Value: 1}, {Key: "day", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (r *MongoPracticeItemHistoryRepository) Put(ctx context.Context, studentID, itemKey string, snapshots []domain.ItemSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	updatedAt := r.now().UTC()
	models := make([]mongo.WriteModel, len(snapshots))
	for i, s := range snapshots {
		f := s.Fold
		models[i] = mongo.NewReplaceOneModel().
			SetFilter(bson.D{{Key: "student_id", Value: studentID}, {Key: "item_key", Value: itemKey}, {Key: "day", Value: s.Day}}).
			SetReplacement(practiceItemSnapshotDocument{
				StudentID:            studentID,
				ItemKey:              itemKey,
				Day:                  s.Day,
				RulesVersion:         domain.PracticeRulesVersion,
				Level:                string(f.Level()),
				Counted:              f.Counted,
				Accuracy:             f.Accuracy,
				Fluency:              f.Fluency,
				Box:                  f.Box,
				BestCleanBPM:         f.BestCleanBPM,
				BestChangesPerMinute: f.BestChangesPerMinute,
				UpdatedAt:            updatedAt,
			}).
			SetUpsert(true)
	}
	_, err := r.collection.BulkWrite(ctx, models)
	return err
}
