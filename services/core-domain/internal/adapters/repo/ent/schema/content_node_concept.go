package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ContentNodeConcept is the join entity behind ContentNode's many-to-many
// "concepts" edge — kept independent from ExerciseConcept since a content
// node's and an exercise's concept classification are edited and queried
// independently.
type ContentNodeConcept struct {
	ent.Schema
}

func (ContentNodeConcept) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.UUID("concept_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ContentNodeConcept) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("content_node", ContentNode.Type).
			Unique().
			Required().
			Immutable().
			Field("content_node_id"),

		edge.To("concept", Concept.Type).
			Unique().
			Required().
			Immutable().
			Field("concept_id"),
	}
}

func (ContentNodeConcept) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_node_id", "concept_id").Unique(),
	}
}
