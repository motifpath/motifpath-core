package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// MongoPracticeEvidenceRepository stores practice evidence in the
// `practice_evidence` collection, append-only.
type MongoPracticeEvidenceRepository struct {
	collection *mongo.Collection
}

func NewMongoPracticeEvidenceRepository(db *mongo.Database) *MongoPracticeEvidenceRepository {
	return &MongoPracticeEvidenceRepository{collection: db.Collection("practice_evidence")}
}

// EnsureIndexes creates the unique evidence_id index that makes an event count
// once, and the index an item's history is read by. It is idempotent.
func (r *MongoPracticeEvidenceRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "evidence_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "student_id", Value: 1}, {Key: "item_key", Value: 1}, {Key: "occurred_at", Value: 1}}},
	})
	return err
}

func (r *MongoPracticeEvidenceRepository) Insert(ctx context.Context, e domain.PracticeEvidence) (bool, error) {
	doc, err := toEvidenceDocument(e)
	if err != nil {
		return false, err
	}
	_, err = r.collection.InsertOne(ctx, doc)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

// ListForItem returns the item's evidence in insertion order: the driver's
// ObjectIDs grow with each insert, so sorting by _id keeps arrival order.
func (r *MongoPracticeEvidenceRepository) ListForItem(ctx context.Context, studentID, itemKey string) ([]domain.PracticeEvidence, error) {
	cursor, err := r.collection.Find(ctx,
		bson.D{{Key: "student_id", Value: studentID}, {Key: "item_key", Value: itemKey}},
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	var docs []practiceEvidenceDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	evidence := make([]domain.PracticeEvidence, len(docs))
	for i, d := range docs {
		if evidence[i], err = d.toDomain(); err != nil {
			return nil, err
		}
	}
	return evidence, nil
}
