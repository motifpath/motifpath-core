package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// DiagramSkill is the join entity behind Diagram's many-to-many
// "skills" edge — kept independent from ContentNodeSkill and
// ExerciseSkill since a diagram's classification is edited and queried
// independently of theirs.
type DiagramSkill struct {
	ent.Schema
}

func (DiagramSkill) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("diagram_id", uuid.UUID{}).
			Immutable(),

		field.UUID("skill_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (DiagramSkill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("diagram", Diagram.Type).
			Unique().
			Required().
			Immutable().
			Field("diagram_id"),

		edge.To("skill", Skill.Type).
			Unique().
			Required().
			Immutable().
			Field("skill_id"),
	}
}

func (DiagramSkill) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("diagram_id", "skill_id").Unique(),
	}
}
