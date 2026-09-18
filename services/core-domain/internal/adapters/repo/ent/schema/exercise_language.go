package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ExerciseLanguage is the join entity behind Exercise's many-to-many
// "languages" edge — kept independent from ContentNodeLanguage: an exercise
// is reusable across content nodes (ADR-019), so it has no single parent to
// inherit a language from, and its language tagging is edited and queried
// independently. See ADR-024's 2026-09-18 amendment.
type ExerciseLanguage struct {
	ent.Schema
}

func (ExerciseLanguage) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("exercise_id", uuid.UUID{}).
			Immutable(),

		field.UUID("language_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ExerciseLanguage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("exercise", Exercise.Type).
			Unique().
			Required().
			Immutable().
			Field("exercise_id"),

		edge.To("language", Language.Type).
			Unique().
			Required().
			Immutable().
			Field("language_id"),
	}
}

func (ExerciseLanguage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("exercise_id", "language_id").Unique(),
	}
}
