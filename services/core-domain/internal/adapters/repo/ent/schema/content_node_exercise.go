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
// exercise's order is a stable, teacher-authored sequence, so link order is
// a real, queried column (position) rather than incidental row order.
type ContentNodeExercise struct {
	ent.Schema
}

func (ContentNodeExercise) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.UUID("exercise_id", uuid.UUID{}).
			Immutable(),

		// Position is assigned at link time, one greater than the highest
		// position already linked to this content node — 0 for the first
		// exercise linked, monotonically increasing after that, never
		// reused when a link is removed.
		field.Int("position").
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
		index.Fields("content_node_id", "position"),
	}
}
