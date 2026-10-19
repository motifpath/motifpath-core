package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ChallengeExercise is the join entity behind Challenge's and Exercise's
// many-to-many "exercises"/"challenges" edges. It exists as its own entity,
// rather than an implicit ent join table, so link order is real and
// queryable — its auto-incrementing id column is assigned atomically by
// Postgres on insert (no read-then-write race), and already sorts rows in
// link order with no extra column or query needed.
type ChallengeExercise struct {
	ent.Schema
}

func (ChallengeExercise) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("challenge_id", uuid.UUID{}).
			Immutable(),

		field.UUID("exercise_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ChallengeExercise) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("challenge", Challenge.Type).
			Unique().
			Required().
			Immutable().
			Field("challenge_id"),

		edge.To("exercise", Exercise.Type).
			Unique().
			Required().
			Immutable().
			Field("exercise_id"),
	}
}

func (ChallengeExercise) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("challenge_id", "exercise_id").Unique(),
	}
}
