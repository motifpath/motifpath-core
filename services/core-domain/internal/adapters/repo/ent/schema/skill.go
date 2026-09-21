package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Skill is a self-referential tree node naming an observable, practicable
// skill a ContentNode or Exercise can be classified under. parent_id null
// means a root skill; any skill may have children, to any depth. A name is
// unique among siblings sharing the same parent_id — including among other
// roots — not globally, enforced in the application layer since a nullable
// parent_id column cannot carry that constraint as a plain unique index
// (Postgres treats NULL as distinct from NULL).
type Skill struct {
	ent.Schema
}

func (Skill) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.String("name"),

		field.UUID("parent_id", uuid.UUID{}).
			Optional().
			Nillable(),
	}
}

func (Skill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("parent", Skill.Type).
			Unique().
			Field("parent_id").
			From("children"),

		edge.From("content_nodes", ContentNode.Type).
			Ref("skills").
			Through("content_node_skills", ContentNodeSkill.Type),

		edge.From("exercises", Exercise.Type).
			Ref("skills").
			Through("exercise_skills", ExerciseSkill.Type),

		edge.From("diagrams", Diagram.Type).
			Ref("skills").
			Through("diagram_skills", DiagramSkill.Type),
	}
}
