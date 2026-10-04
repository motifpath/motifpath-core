package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/core-domain/internal/domain"
)

// practiceReferenceSnapshotVersion is the shape of the documents this
// writer stores. Bump it when a document's fields change, so a reader can
// tell the shapes apart.
const practiceReferenceSnapshotVersion = 1

// diagramReferenceDocument is a `practice_reference` document of kind
// "diagram", in the shape ADR-047 fixes for the Aggregation Worker's
// graders. TempoBPM is stored as null for a diagram without playback.
type diagramReferenceDocument struct {
	Kind            string    `bson:"kind"`
	ID              string    `bson:"id"`
	InstrumentIDs   []string  `bson:"instrument_ids"`
	TempoBPM        *int      `bson:"tempo_bpm"`
	UpdatedAt       time.Time `bson:"updated_at"`
	SnapshotVersion int       `bson:"snapshot_version"`
}

// MongoPracticeReferenceWriter keeps the `practice_reference` collection
// (ADR-047). This service is its only writer; the Aggregation Worker reads
// it. Documents are keyed by {kind, id}, replaced in place, never removed.
type MongoPracticeReferenceWriter struct {
	collection *mongo.Collection
	now        func() time.Time
}

func NewMongoPracticeReferenceWriter(db *mongo.Database, now func() time.Time) *MongoPracticeReferenceWriter {
	return &MongoPracticeReferenceWriter{collection: db.Collection("practice_reference"), now: now}
}

// EnsureIndexes creates the unique {kind, id} index. It is idempotent.
func (w *MongoPracticeReferenceWriter) EnsureIndexes(ctx context.Context) error {
	_, err := w.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "kind", Value: 1}, {Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (w *MongoPracticeReferenceWriter) PutDiagram(ctx context.Context, ref domain.DiagramReference) error {
	doc := diagramReferenceDocument{
		Kind:            "diagram",
		ID:              ref.ID,
		InstrumentIDs:   ref.InstrumentIDs,
		TempoBPM:        ref.TempoBPM,
		UpdatedAt:       w.now(),
		SnapshotVersion: practiceReferenceSnapshotVersion,
	}
	_, err := w.collection.ReplaceOne(ctx,
		bson.D{{Key: "kind", Value: doc.Kind}, {Key: "id", Value: doc.ID}},
		doc,
		options.Replace().SetUpsert(true),
	)
	return err
}
