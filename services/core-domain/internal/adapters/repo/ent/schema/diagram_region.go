package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// DiagramRegion is one highlighted area of a Diagram. Like Position, a single
// table carries both coordinate shapes with nullable field groups — frets
// (and optionally strings) for a fretted diagram, keys for a keyboard one —
// and which group is populated follows the parent Diagram's instrument
// family, validated in the domain constructor, not by a column constraint.
type DiagramRegion struct {
	ent.Schema
}

func (DiagramRegion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("diagram_id", uuid.UUID{}),

		// ordinal preserves the drawing order: later regions on top.
		field.Int("ordinal"),

		field.Int("fret_start").
			Optional().
			Nillable(),

		field.Int("fret_end").
			Optional().
			Nillable(),

		field.Int("string_start").
			Optional().
			Nillable(),

		field.Int("string_end").
			Optional().
			Nillable(),

		field.String("key_start").
			Optional().
			Nillable(),

		field.String("key_end").
			Optional().
			Nillable(),

		// description maps a language code to the caption in that language,
		// covering exactly the parent diagram's languages.
		field.JSON("description", map[string]string{}),

		// color is the band's tint as #RRGGBB; NULL = the default tint.
		field.String("color").
			Optional().
			Nillable(),
	}
}

func (DiagramRegion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("diagram", Diagram.Type).
			Ref("regions").
			Unique().
			Required().
			Field("diagram_id"),
	}
}

func (DiagramRegion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("diagram_id", "ordinal").Unique(),
	}
}
