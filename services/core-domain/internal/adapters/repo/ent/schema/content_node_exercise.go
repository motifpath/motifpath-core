package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ContentNodeExercise is the join entity behind ContentNode's and
// Exercise's many-to-many "path_exercises"/"content_nodes" edges — a path
// exercise's order is a stable, teacher-authored sequence, so link order
// must be real and queryable. Its auto-incrementing id column is assigned
// atomically by Postgres on insert (no read-then-write race), and already
// sorts rows in link order with no extra column or query needed.
type ContentNodeExercise struct {
	ent.Schema
}

func (ContentNodeExercise) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.UUID("exercise_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ContentNodeExercise) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("content_node", ContentNode.Type).
			Unique().
			Required().
			Immutable().
			Field("content_node_id"),

		edge.To("exercise", Exercise.Type).
			Unique().
			Required().
			Immutable().
			Field("exercise_id"),
	}
}

func (ContentNodeExercise) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_node_id", "exercise_id").Unique(),
	}
}
