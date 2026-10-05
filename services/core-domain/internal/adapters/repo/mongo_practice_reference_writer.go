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

// exerciseReferenceDocument is a `practice_reference` document of kind
// "exercise". InstrumentIDs is empty, never null, for an exercise for every
// instrument.
type exerciseReferenceDocument struct {
	Kind             string    `bson:"kind"`
	ID               string    `bson:"id"`
	ExerciseType     string    `bson:"exercise_type"`
	OptionIDs        []string  `bson:"option_ids"`
	CorrectOptionIDs []string  `bson:"correct_option_ids"`
	InstrumentIDs    []string  `bson:"instrument_ids"`
	UpdatedAt        time.Time `bson:"updated_at"`
	SnapshotVersion  int       `bson:"snapshot_version"`
}

// drillThresholdDocument is a `practice_reference` document of kind
// "drill_threshold": one fluent time version of a timed drill template.
type drillThresholdDocument struct {
	Kind            string    `bson:"kind"`
	ID              string    `bson:"id"`
	TemplateKey     string    `bson:"template_key"`
	Version         int       `bson:"version"`
	EffectiveFrom   time.Time `bson:"effective_from"`
	FluentNetMs     int       `bson:"fluent_net_ms"`
	Source          string    `bson:"source"`
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

// PutExercises upserts every reference in one unordered bulk write.
func (w *MongoPracticeReferenceWriter) PutExercises(ctx context.Context, refs []domain.ExerciseReference) error {
	at := w.now()
	docs := make([]exerciseReferenceDocument, len(refs))
	for i, ref := range refs {
		docs[i] = exerciseReferenceDocument{
			Kind:             "exercise",
			ID:               ref.ID,
			ExerciseType:     string(ref.ExerciseType),
			OptionIDs:        nonNil(ref.OptionIDs),
			CorrectOptionIDs: nonNil(ref.CorrectOptionIDs),
			InstrumentIDs:    nonNil(ref.InstrumentIDs),
			UpdatedAt:        at,
			SnapshotVersion:  practiceReferenceSnapshotVersion,
		}
	}
	return upsert(ctx, w.collection, docs, func(d exerciseReferenceDocument) (string, string) { return d.Kind, d.ID })
}

// PutDrillThresholds upserts every version in one unordered bulk write.
func (w *MongoPracticeReferenceWriter) PutDrillThresholds(ctx context.Context, thresholds []domain.DrillThreshold) error {
	at := w.now()
	docs := make([]drillThresholdDocument, len(thresholds))
	for i, th := range thresholds {
		docs[i] = drillThresholdDocument{
			Kind:            "drill_threshold",
			ID:              th.ID,
			TemplateKey:     th.TemplateKey,
			Version:         th.Version,
			EffectiveFrom:   th.EffectiveFrom,
			FluentNetMs:     th.FluentNetMs,
			Source:          string(th.Source),
			UpdatedAt:       at,
			SnapshotVersion: practiceReferenceSnapshotVersion,
		}
	}
	return upsert(ctx, w.collection, docs, func(d drillThresholdDocument) (string, string) { return d.Kind, d.ID })
}

// PutDiagrams upserts every reference in one unordered bulk write, so a
// sync of the whole catalog costs one round trip per page, not per diagram.
func (w *MongoPracticeReferenceWriter) PutDiagrams(ctx context.Context, refs []domain.DiagramReference) error {
	at := w.now()
	docs := make([]diagramReferenceDocument, len(refs))
	for i, ref := range refs {
		docs[i] = diagramReferenceDocument{
			Kind:            "diagram",
			ID:              ref.ID,
			InstrumentIDs:   ref.InstrumentIDs,
			TempoBPM:        ref.TempoBPM,
			UpdatedAt:       at,
			SnapshotVersion: practiceReferenceSnapshotVersion,
		}
	}
	return upsert(ctx, w.collection, docs, func(d diagramReferenceDocument) (string, string) { return d.Kind, d.ID })
}

// upsert replaces each document by its {kind, id}, inserting it when new.
func upsert[T any](ctx context.Context, collection *mongo.Collection, docs []T, key func(T) (kind, id string)) error {
	if len(docs) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, len(docs))
	for i, doc := range docs {
		kind, id := key(doc)
		models[i] = mongo.NewReplaceOneModel().
			SetFilter(bson.D{{Key: "kind", Value: kind}, {Key: "id", Value: id}}).
			SetReplacement(doc).
			SetUpsert(true)
	}
	_, err := collection.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}

func nonNil(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
