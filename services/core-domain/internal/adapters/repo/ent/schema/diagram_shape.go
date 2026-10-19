package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// DiagramShape makes a catalog diagram a drill shape: the member of a
// DiagramShapeFamily it is. Reference data, installed by migration with a
// fixed id and never edited.
type DiagramShape struct {
	ent.Schema
}

func (DiagramShape) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Immutable(),

		field.UUID("diagram_id", uuid.UUID{}).
			Unique().
			Immutable(),

		field.UUID("family_id", uuid.UUID{}).
			Immutable(),

		// shape is the member's key in its family, such as "A".
		field.String("shape").
			NotEmpty().
			Immutable(),
	}
}

func (DiagramShape) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("diagram", Diagram.Type).
			Ref("shape").
			Unique().
			Required().
			Immutable().
			Field("diagram_id"),
		edge.To("family", DiagramShapeFamily.Type).
			Unique().
			Required().
			Immutable().
			Field("family_id"),
	}
}
