package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ContentNodeSkill is the join entity behind ContentNode's many-to-many
// "skills" edge — kept independent from ExerciseSkill since a content
// node's and an exercise's skill classification are edited and queried
// independently.
type ContentNodeSkill struct {
	ent.Schema
}

func (ContentNodeSkill) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.UUID("skill_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ContentNodeSkill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("content_node", ContentNode.Type).
			Unique().
			Required().
			Immutable().
			Field("content_node_id"),

		edge.To("skill", Skill.Type).
			Unique().
			Required().
			Immutable().
			Field("skill_id"),
	}
}

func (ContentNodeSkill) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_node_id", "skill_id").Unique(),
	}
}
