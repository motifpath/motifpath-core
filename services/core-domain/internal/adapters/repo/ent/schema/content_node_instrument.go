package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ContentNodeInstrument is the join entity behind ContentNode's many-to-many "instruments"
// edge: the instruments a content node is for. None at all means it suits every
// instrument.
type ContentNodeInstrument struct {
	ent.Schema
}

func (ContentNodeInstrument) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.UUID("instrument_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ContentNodeInstrument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("content_node", ContentNode.Type).
			Unique().
			Required().
			Immutable().
			Field("content_node_id"),

		edge.To("instrument", Instrument.Type).
			Unique().
			Required().
			Immutable().
			Field("instrument_id"),
	}
}

func (ContentNodeInstrument) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_node_id", "instrument_id").Unique(),
	}
}
