package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ChordVoicing is one fingering of a chord, whose positions and playbacks
// are its diagram's. Voicings are reference data, installed only by
// migrations with fixed IDs; only their status changes, when the catalog
// withdraws one.
type ChordVoicing struct {
	ent.Schema
}

// VoicingFinger is the fretting-hand finger on one position of the voicing's
// diagram.
type VoicingFinger struct {
	PositionID string `json:"position_id"`
	Finger     string `json:"finger"`
}

func (ChordVoicing) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		field.UUID("chord_definition_id", uuid.UUID{}).
			Immutable(),

		field.UUID("diagram_id", uuid.UUID{}).
			Unique().
			Immutable(),

		field.UUID("instrument_id", uuid.UUID{}).
			Immutable(),

		field.String("tuning_fingerprint").
			MaxLen(100).
			NotEmpty().
			Immutable(),

		field.Int("lowest_fret").
			NonNegative().
			Immutable(),

		field.Int("highest_fret").
			NonNegative().
			Immutable(),

		field.JSON("fingering", []VoicingFinger{}).
			Immutable(),

		field.JSON("muted_strings", []int{}).
			Immutable(),

		field.JSON("omitted_intervals", []string{}).
			Immutable(),

		field.Enum("difficulty").
			Values("beginner", "intermediate", "advanced").
			Immutable(),

		field.JSON("technique_tags", []string{}).
			Immutable(),

		field.Enum("shape_family").
			Values("open", "e_shape", "a_shape", "d_shape", "shell", "drop_2", "drop_3").
			Optional().
			Nillable().
			Immutable(),

		field.Bool("is_movable").
			Immutable(),

		field.Int("recommended_rank").
			Positive(),

		field.Enum("status").
			Values("active", "withdrawn").
			Default("active"),

		// template_key names the movable shape that generated the voicing;
		// null for a hand-authored one.
		field.String("template_key").
			MaxLen(100).
			Optional().
			Nillable().
			Immutable(),
	}
}

// Each diagram is the fingering of at most one voicing.
func (ChordVoicing) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("diagram_id").
			Unique(),
	}
}

func (ChordVoicing) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("chord_definition", ChordDefinition.Type).
			Field("chord_definition_id").
			Unique().
			Required().
			Immutable(),
		edge.To("diagram", Diagram.Type).
			Field("diagram_id").
			Unique().
			Required().
			Immutable(),
		edge.To("instrument", Instrument.Type).
			Field("instrument_id").
			Unique().
			Required().
			Immutable(),
	}
}
