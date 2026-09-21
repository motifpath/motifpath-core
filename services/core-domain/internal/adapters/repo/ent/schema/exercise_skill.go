package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ExerciseSkill is the join entity behind Exercise's many-to-many "skills"
// edge, replacing the freeform skill_tags string array.
type ExerciseSkill struct {
	ent.Schema
}

func (ExerciseSkill) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("exercise_id", uuid.UUID{}).
			Immutable(),

		field.UUID("skill_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ExerciseSkill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("exercise", Exercise.Type).
			Unique().
			Required().
			Immutable().
			Field("exercise_id"),

		edge.To("skill", Skill.Type).
			Unique().
			Required().
			Immutable().
			Field("skill_id"),
	}
}

func (ExerciseSkill) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("exercise_id", "skill_id").Unique(),
	}
}
