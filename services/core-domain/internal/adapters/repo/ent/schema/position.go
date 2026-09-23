package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Position is one marked location in a Diagram. A single table carries both
// coordinate shapes with nullable field groups — string_number and fret for
// a fretted diagram, key for a keyboard one — because a Diagram never mixes
// shapes and a Position is never shared across Diagrams. Which group is
// populated follows the parent Diagram's instrument family, validated in the
// domain constructor, not by a column constraint.
type Position struct {
	ent.Schema
}

func (Position) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("diagram_id", uuid.UUID{}),

		// ordinal preserves the order the author listed positions in.
		field.Int("ordinal"),

		field.String("interval"),

		field.String("note_name"),

		field.Enum("shape").
			Values("dot", "square", "star").
			Default("dot"),

		field.Int("sequence_index").
			Optional().
			Nillable(),

		field.Int("string_number").
			Optional().
			Nillable(),

		field.Int("fret").
			Optional().
			Nillable(),

		field.String("key").
			Optional().
			Nillable(),
	}
}

func (Position) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("diagram", Diagram.Type).
			Ref("positions").
			Unique().
			Required().
			Field("diagram_id"),
	}
}

func (Position) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("diagram_id", "ordinal").Unique(),
	}
}
