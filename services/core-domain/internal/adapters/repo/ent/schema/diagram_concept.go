package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// DiagramConcept is the join entity behind Diagram's many-to-many
// "concepts" edge — kept independent from ContentNodeConcept and
// ExerciseConcept since a diagram's classification is edited and queried
// independently of theirs.
type DiagramConcept struct {
	ent.Schema
}

func (DiagramConcept) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("diagram_id", uuid.UUID{}).
			Immutable(),

		field.UUID("concept_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (DiagramConcept) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("diagram", Diagram.Type).
			Unique().
			Required().
			Immutable().
			Field("diagram_id"),

		edge.To("concept", Concept.Type).
			Unique().
			Required().
			Immutable().
			Field("concept_id"),
	}
}

func (DiagramConcept) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("diagram_id", "concept_id").Unique(),
	}
}
