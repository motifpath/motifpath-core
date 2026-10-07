package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/motifpath/core-domain/internal/domain"
)

// practiceReferenceSnapshotVersion is the shape of the documents this
// writer stores. Bump it when a document's fields change, so a reader can
// tell the shapes apart. Version 2 added the exercise options and the
// instrument documents; version 3 the drill shape fields of a diagram.
const practiceReferenceSnapshotVersion = 3

// diagramReferenceDocument is a `practice_reference` document of kind
// "diagram", in the shape ADR-047 fixes for the Aggregation Worker's
// graders. TempoBPM is stored as null for a diagram without playback. A
// drill shape also holds its layout, family, member, its family's members
// and its positions; every other diagram leaves them out.
type diagramReferenceDocument struct {
	Kind               string                  `bson:"kind"`
	ID                 string                  `bson:"id"`
	InstrumentIDs      []string                `bson:"instrument_ids"`
	TempoBPM           *int                    `bson:"tempo_bpm"`
	LayoutInstrumentID string                  `bson:"layout_instrument_id,omitempty"`
	ShapeFamily        string                  `bson:"shape_family,omitempty"`
	Shape              string                  `bson:"shape,omitempty"`
	FamilyMembers      []string                `bson:"family_members,omitempty"`
	Positions          []shapePositionDocument `bson:"positions,omitempty"`
	UpdatedAt          time.Time               `bson:"updated_at"`
	SnapshotVersion    int                     `bson:"snapshot_version"`
}

// shapePositionDocument is one marker of a drill shape: where it is, and its
// interval from the diagram's root.
type shapePositionDocument struct {
	String   int    `bson:"string"`
	Fret     int    `bson:"fret"`
	Interval string `bson:"interval"`
}

// exerciseReferenceDocument is a `practice_reference` document of kind
// "exercise". InstrumentIDs is empty, never null, for an exercise for every
// instrument.
type exerciseReferenceDocument struct {
	Kind             string   `bson:"kind"`
	ID               string   `bson:"id"`
	ExerciseType     string   `bson:"exercise_type"`
	OptionIDs        []string `bson:"option_ids"`
	CorrectOptionIDs []string `bson:"correct_option_ids"`
	InstrumentIDs    []string `bson:"instrument_ids"`
	// Options keeps every option in the shape the API shows it, so a grader
	// can copy what the student saw into the evidence as it is.
	Options         []optionDocument `bson:"options"`
	UpdatedAt       time.Time        `bson:"updated_at"`
	SnapshotVersion int              `bson:"snapshot_version"`
}

// optionDocument is an exercise option as the API shows it; an absent part
// is left out, as the API leaves it out.
type optionDocument struct {
	OptionID          string                `bson:"option_id"`
	IsCorrect         bool                  `bson:"is_correct"`
	Label             *string               `bson:"label,omitempty"`
	ImageURL          *string               `bson:"image_url,omitempty"`
	DiagramRef        bson.D                `bson:"diagram_ref,omitempty"`
	AudioURL          *string               `bson:"audio_url,omitempty"`
	Region            *optionRegionDocument `bson:"region,omitempty"`
	DiagramID         *string               `bson:"diagram_id,omitempty"`
	DiagramPositionID *string               `bson:"diagram_position_id,omitempty"`
	FretCell          *fretCellDocument     `bson:"fret_cell,omitempty"`
}

type optionRegionDocument struct {
	X      float64 `bson:"x"`
	Y      float64 `bson:"y"`
	Width  float64 `bson:"width"`
	Height float64 `bson:"height"`
	Shape  string  `bson:"shape"`
}

type fretCellDocument struct {
	String int `bson:"string"`
	Fret   int `bson:"fret"`
}

// instrumentReferenceDocument is a `practice_reference` document of kind
// "instrument": what gives each fretboard cell its pitch. StringCount is null
// and Tuning empty for an instrument without strings.
type instrumentReferenceDocument struct {
	Kind            string    `bson:"kind"`
	ID              string    `bson:"id"`
	Family          string    `bson:"family"`
	StringCount     *int      `bson:"string_count"`
	Tuning          []string  `bson:"tuning"`
	UpdatedAt       time.Time `bson:"updated_at"`
	SnapshotVersion int       `bson:"snapshot_version"`
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
// This service is its only writer; the Aggregation Worker reads
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
		options, err := optionDocuments(ref.Options)
		if err != nil {
			return fmt.Errorf("exercise %s: %w", ref.ID, err)
		}
		docs[i] = exerciseReferenceDocument{
			Kind:             "exercise",
			ID:               ref.ID,
			ExerciseType:     string(ref.ExerciseType),
			OptionIDs:        nonNil(ref.OptionIDs),
			CorrectOptionIDs: nonNil(ref.CorrectOptionIDs),
			InstrumentIDs:    nonNil(ref.InstrumentIDs),
			Options:          options,
			UpdatedAt:        at,
			SnapshotVersion:  practiceReferenceSnapshotVersion,
		}
	}
	return upsert(ctx, w.collection, docs, func(d exerciseReferenceDocument) (string, string) { return d.Kind, d.ID })
}

// optionDocuments converts options to their API shape. A diagram ref is
// stored as the document its JSON form describes.
func optionDocuments(options []domain.Option) ([]optionDocument, error) {
	docs := make([]optionDocument, len(options))
	for i, o := range options {
		doc := optionDocument{
			OptionID: o.ID, IsCorrect: o.IsCorrect, Label: o.Label, ImageURL: o.ImageURL, AudioURL: o.AudioURL,
			DiagramID: o.DiagramID, DiagramPositionID: o.DiagramPositionID,
		}
		if o.Region != nil {
			doc.Region = &optionRegionDocument{X: o.Region.X, Y: o.Region.Y, Width: o.Region.Width, Height: o.Region.Height, Shape: string(o.Region.Shape)}
		}
		if o.FretCell != nil {
			doc.FretCell = &fretCellDocument{String: o.FretCell.String, Fret: o.FretCell.Fret}
		}
		if o.DiagramRef != nil {
			data, err := json.Marshal(o.DiagramRef)
			if err != nil {
				return nil, err
			}
			if err := bson.UnmarshalExtJSON(data, false, &doc.DiagramRef); err != nil {
				return nil, err
			}
		}
		docs[i] = doc
	}
	return docs, nil
}

// PutInstruments upserts every reference in one unordered bulk write.
func (w *MongoPracticeReferenceWriter) PutInstruments(ctx context.Context, refs []domain.InstrumentReference) error {
	at := w.now()
	docs := make([]instrumentReferenceDocument, len(refs))
	for i, ref := range refs {
		docs[i] = instrumentReferenceDocument{
			Kind:            "instrument",
			ID:              ref.ID,
			Family:          string(ref.Family),
			StringCount:     ref.StringCount,
			Tuning:          nonNil(ref.Tuning),
			UpdatedAt:       at,
			SnapshotVersion: practiceReferenceSnapshotVersion,
		}
	}
	return upsert(ctx, w.collection, docs, func(d instrumentReferenceDocument) (string, string) { return d.Kind, d.ID })
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
		if shape := ref.Shape; shape != nil {
			docs[i].LayoutInstrumentID = shape.LayoutInstrumentID
			docs[i].ShapeFamily = shape.Family
			docs[i].Shape = shape.Shape
			docs[i].FamilyMembers = shape.FamilyMembers
			docs[i].Positions = make([]shapePositionDocument, len(shape.Positions))
			for j, p := range shape.Positions {
				docs[i].Positions[j] = shapePositionDocument{String: p.String, Fret: p.Fret, Interval: p.Interval}
			}
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
