package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/motifpath/core-domain/internal/domain"
)

// practiceItemStateDocument holds the fields this service reads from a
// `practice_item_state` document, in the shape the Aggregation Worker's
// evidence processor writes it.
type practiceItemStateDocument struct {
	ItemKey      string     `bson:"item_key"`
	RulesVersion int        `bson:"rules_version"`
	Level        string     `bson:"level"`
	Counted      int        `bson:"counted"`
	Box          int        `bson:"box"`
	DueAt        *time.Time `bson:"due_at"`
	LastAt       *time.Time `bson:"last_at"`
	Accuracy     float64    `bson:"accuracy"`
	Fluency      float64    `bson:"fluency"`
	BestCleanBPM *int       `bson:"best_clean_bpm"`
}

// MongoPracticeItemStateReader reads the `practice_item_state` collection
// the Aggregation Worker owns. It never writes to it.
type MongoPracticeItemStateReader struct {
	collection *mongo.Collection
}

func NewMongoPracticeItemStateReader(db *mongo.Database) *MongoPracticeItemStateReader {
	return &MongoPracticeItemStateReader{collection: db.Collection("practice_item_state")}
}

func (r *MongoPracticeItemStateReader) GetStates(ctx context.Context, studentID string, itemKeys []string) (map[string]domain.PracticeItemState, error) {
	result := map[string]domain.PracticeItemState{}
	if len(itemKeys) == 0 {
		return result, nil
	}

	cursor, err := r.collection.Find(ctx, bson.D{
		{Key: "student_id", Value: studentID},
		{Key: "item_key", Value: bson.D{{Key: "$in", Value: itemKeys}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc practiceItemStateDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.ItemKey] = domain.PracticeItemState{
			ItemKey:      doc.ItemKey,
			RulesVersion: doc.RulesVersion,
			Level:        domain.KnowledgeLevel(doc.Level),
			Counted:      doc.Counted,
			Box:          doc.Box,
			DueAt:        utcTime(doc.DueAt),
			LastAt:       utcTime(doc.LastAt),
			Accuracy:     doc.Accuracy,
			Fluency:      doc.Fluency,
			BestCleanBPM: doc.BestCleanBPM,
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func utcTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
