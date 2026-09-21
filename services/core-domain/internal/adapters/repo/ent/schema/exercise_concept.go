package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ExerciseConcept is the join entity behind Exercise's many-to-many
// "concepts" edge, replacing the freeform skill_tags string array's concept
// counterpart (which never existed as a separate field — both skill and
// concept classification are new for Exercise as of this schema).
type ExerciseConcept struct {
	ent.Schema
}

func (ExerciseConcept) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("exercise_id", uuid.UUID{}).
			Immutable(),

		field.UUID("concept_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ExerciseConcept) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("exercise", Exercise.Type).
			Unique().
			Required().
			Immutable().
			Field("exercise_id"),

		edge.To("concept", Concept.Type).
			Unique().
			Required().
			Immutable().
			Field("concept_id"),
	}
}

func (ExerciseConcept) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("exercise_id", "concept_id").Unique(),
	}
}
