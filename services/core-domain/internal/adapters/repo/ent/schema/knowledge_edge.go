package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// KnowledgeEdge is a typed link between two knowledge nodes: applies
// (skill → concept, no level) or requires (any → any, with a level). At
// most one edge of each type links the same two nodes in the same
// direction. Which kinds a type may link, the level rules and the
// no-cycle rule for requires are checked in the application layer.
type KnowledgeEdge struct {
	ent.Schema
}

func (KnowledgeEdge) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("from_id", uuid.UUID{}).
			Immutable(),

		field.UUID("to_id", uuid.UUID{}).
			Immutable(),

		field.Enum("type").
			Values("applies", "requires").
			Immutable(),

		// level is null for applies.
		field.Enum("level").
			Values("accurate", "fluent", "retained").
			Optional().
			Nillable(),
	}
}

func (KnowledgeEdge) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("from", KnowledgeNode.Type).
			Unique().
			Required().
			Immutable().
			Field("from_id"),

		edge.To("to", KnowledgeNode.Type).
			Unique().
			Required().
			Immutable().
			Field("to_id"),
	}
}

func (KnowledgeEdge) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("from_id", "to_id", "type").Unique(),
		index.Fields("to_id"),
	}
}
