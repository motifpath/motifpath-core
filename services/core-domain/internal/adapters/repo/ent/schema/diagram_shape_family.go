package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// DiagramShapeFamily is a practice drill catalog family of diagram shapes,
// such as the CAGED grips. Reference data, installed by migration with a
// fixed id and never edited.
type DiagramShapeFamily struct {
	ent.Schema
}

func (DiagramShapeFamily) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		field.String("key").
			NotEmpty().
			Unique().
			Immutable(),

		// names maps a language code to the family's name in it.
		field.JSON("names", map[string]string{}).
			Immutable(),

		// members lists the family's shapes in catalog order, the order a
		// shape to name offers them in.
		field.JSON("members", []ShapeMember{}).
			Immutable(),
	}
}

// ShapeMember is one stored member of a DiagramShapeFamily.
type ShapeMember struct {
	Shape string            `json:"shape"`
	Names map[string]string `json:"names"`
}

func (DiagramShapeFamily) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shapes", DiagramShape.Type).
			Ref("family"),
	}
}
