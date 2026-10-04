package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// diagramReferenceDocument is a `practice_reference` document of kind "diagram",
// as core-domain writes it. core-domain is the collection's only writer.
type diagramReferenceDocument struct {
	ID            string   `bson:"id"`
	InstrumentIDs []string `bson:"instrument_ids"`
	TempoBPM      *int     `bson:"tempo_bpm"`
}

// MongoPracticeReferenceReader reads the reference data the practice graders use.
type MongoPracticeReferenceReader struct {
	collection *mongo.Collection
}

func NewMongoPracticeReferenceReader(db *mongo.Database) *MongoPracticeReferenceReader {
	return &MongoPracticeReferenceReader{collection: db.Collection("practice_reference")}
}

func (r *MongoPracticeReferenceReader) Diagrams(ctx context.Context, ids []string) (map[string]domain.DiagramReference, error) {
	found := map[string]domain.DiagramReference{}
	if len(ids) == 0 {
		return found, nil
	}
	cursor, err := r.collection.Find(ctx, bson.D{
		{Key: "kind", Value: "diagram"},
		{Key: "id", Value: bson.D{{Key: "$in", Value: ids}}},
	})
	if err != nil {
		return nil, err
	}
	var docs []diagramReferenceDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		found[d.ID] = domain.DiagramReference{ID: d.ID, InstrumentIDs: d.InstrumentIDs, TempoBPM: d.TempoBPM}
	}
	return found, nil
}
