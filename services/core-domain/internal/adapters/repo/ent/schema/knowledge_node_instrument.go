package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// KnowledgeNodeInstrument links a knowledge node to an instrument it is
// for. A node with no rows is for every instrument.
type KnowledgeNodeInstrument struct {
	ent.Schema
}

func (KnowledgeNodeInstrument) Fields() []ent.Field {
	return []ent.Field{
		field.Time("linked_at").
			Immutable().
			Default(time.Now),
		field.UUID("knowledge_node_id", uuid.UUID{}).Immutable(),
		field.UUID("instrument_id", uuid.UUID{}).Immutable(),
	}
}

func (KnowledgeNodeInstrument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("knowledge_node", KnowledgeNode.Type).
			Unique().
			Required().
			Immutable().
			Field("knowledge_node_id"),
		edge.To("instrument", Instrument.Type).
			Unique().
			Required().
			Immutable().
			Field("instrument_id"),
	}
}

func (KnowledgeNodeInstrument) Indexes() []ent.Index {
	return []ent.Index{index.Fields("knowledge_node_id", "instrument_id").Unique()}
}
