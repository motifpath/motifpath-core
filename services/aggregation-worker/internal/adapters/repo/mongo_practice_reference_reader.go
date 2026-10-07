package repo

import (
	"context"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/motifpath/aggregation-worker/internal/domain"
)

// diagramReferenceDocument is a `practice_reference` document of kind "diagram",
// as core-domain writes it. core-domain is the collection's only writer. A drill
// shape also carries its layout, family, member, its family's members and its
// positions; the other diagrams leave them out.
type diagramReferenceDocument struct {
	ID                 string                    `bson:"id"`
	InstrumentIDs      []string                  `bson:"instrument_ids"`
	TempoBPM           *int                      `bson:"tempo_bpm"`
	LayoutInstrumentID string                    `bson:"layout_instrument_id,omitempty"`
	ShapeFamily        string                    `bson:"shape_family,omitempty"`
	Shape              string                    `bson:"shape,omitempty"`
	FamilyMembers      []string                  `bson:"family_members,omitempty"`
	Positions          []diagramPositionDocument `bson:"positions,omitempty"`
}

// diagramPositionDocument is one marker of a drill shape: where it is, and its
// interval from the diagram's root.
type diagramPositionDocument struct {
	String   int    `bson:"string"`
	Fret     int    `bson:"fret"`
	Interval string `bson:"interval"`
}

// exerciseReferenceDocument is a `practice_reference` document of kind "exercise".
type exerciseReferenceDocument struct {
	ID               string   `bson:"id"`
	ExerciseType     string   `bson:"exercise_type"`
	OptionIDs        []string `bson:"option_ids"`
	CorrectOptionIDs []string `bson:"correct_option_ids"`
	InstrumentIDs    []string `bson:"instrument_ids"`
	// Options is every option as core keeps it, absent from a document written
	// before core began keeping them.
	Options []bson.Raw `bson:"options"`
}

// instrumentReferenceDocument is a `practice_reference` document of kind
// "instrument": the tuning that gives each fretboard cell its pitch.
type instrumentReferenceDocument struct {
	ID     string   `bson:"id"`
	Tuning []string `bson:"tuning"`
}

// drillThresholdDocument is a `practice_reference` document of kind
// "drill_threshold": one version of a drill template's fluent time.
type drillThresholdDocument struct {
	TemplateKey   string    `bson:"template_key"`
	Version       int       `bson:"version"`
	EffectiveFrom time.Time `bson:"effective_from"`
	FluentNetMs   int       `bson:"fluent_net_ms"`
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
		diagram := domain.DiagramReference{
			ID: d.ID, InstrumentIDs: d.InstrumentIDs, TempoBPM: d.TempoBPM, LayoutInstrumentID: d.LayoutInstrumentID,
			ShapeFamily: d.ShapeFamily, Shape: d.Shape, FamilyMembers: d.FamilyMembers,
		}
		for _, p := range d.Positions {
			diagram.Positions = append(diagram.Positions, domain.DiagramPosition(p))
		}
		found[d.ID] = diagram
	}
	return found, nil
}

func (r *MongoPracticeReferenceReader) Exercises(ctx context.Context, ids []string) (map[string]domain.ExerciseReference, error) {
	found := map[string]domain.ExerciseReference{}
	if len(ids) == 0 {
		return found, nil
	}
	cursor, err := r.collection.Find(ctx, bson.D{
		{Key: "kind", Value: "exercise"},
		{Key: "id", Value: bson.D{{Key: "$in", Value: ids}}},
	})
	if err != nil {
		return nil, err
	}
	var docs []exerciseReferenceDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		options, err := answerOptions(d.Options)
		if err != nil {
			return nil, err
		}
		found[d.ID] = domain.ExerciseReference{
			ID: d.ID, ExerciseType: d.ExerciseType, OptionIDs: d.OptionIDs,
			CorrectOptionIDs: d.CorrectOptionIDs, InstrumentIDs: d.InstrumentIDs, Options: options,
		}
	}
	return found, nil
}

// answerOptions reads each option's id and verdict, and keeps the whole option
// exactly as core stored it.
func answerOptions(raws []bson.Raw) ([]domain.AnswerOption, error) {
	if len(raws) == 0 {
		return nil, nil
	}
	options := make([]domain.AnswerOption, len(raws))
	for i, raw := range raws {
		var head struct {
			OptionID  string `bson:"option_id"`
			IsCorrect bool   `bson:"is_correct"`
		}
		if err := bson.Unmarshal(raw, &head); err != nil {
			return nil, err
		}
		options[i] = domain.AnswerOption{OptionID: head.OptionID, IsCorrect: head.IsCorrect, Shown: slices.Clone([]byte(raw))}
	}
	return options, nil
}

func (r *MongoPracticeReferenceReader) Instruments(ctx context.Context, ids []string) (map[string]domain.InstrumentReference, error) {
	found := map[string]domain.InstrumentReference{}
	if len(ids) == 0 {
		return found, nil
	}
	cursor, err := r.collection.Find(ctx, bson.D{
		{Key: "kind", Value: "instrument"},
		{Key: "id", Value: bson.D{{Key: "$in", Value: ids}}},
	})
	if err != nil {
		return nil, err
	}
	var docs []instrumentReferenceDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		found[d.ID] = domain.InstrumentReference{ID: d.ID, Tuning: d.Tuning}
	}
	return found, nil
}

func (r *MongoPracticeReferenceReader) FluentTimes(ctx context.Context, templateKey string) ([]domain.FluentTime, error) {
	cursor, err := r.collection.Find(ctx, bson.D{
		{Key: "kind", Value: "drill_threshold"},
		{Key: "template_key", Value: templateKey},
	})
	if err != nil {
		return nil, err
	}
	var docs []drillThresholdDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	times := make([]domain.FluentTime, len(docs))
	for i, d := range docs {
		times[i] = domain.FluentTime{Version: d.Version, EffectiveFrom: d.EffectiveFrom.UTC(), FluentNetMs: d.FluentNetMs}
	}
	slices.SortFunc(times, func(a, b domain.FluentTime) int { return a.Version - b.Version })
	return times, nil
}
