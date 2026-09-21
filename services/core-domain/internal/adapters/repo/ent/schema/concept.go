package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Concept is a self-referential tree node naming an intellectual concept a
// ContentNode or Exercise can be classified under. It forms a tree the same
// way Skill does — see Skill's doc comment for the shared shape and
// sibling-uniqueness rule.
type Concept struct {
	ent.Schema
}

func (Concept) Fields() []ent.Field {
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

func (Concept) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("parent", Concept.Type).
			Unique().
			Field("parent_id").
			From("children"),

		edge.From("content_nodes", ContentNode.Type).
			Ref("concepts").
			Through("content_node_concepts", ContentNodeConcept.Type),

		edge.From("exercises", Exercise.Type).
			Ref("concepts").
			Through("exercise_concepts", ExerciseConcept.Type),
	}
}
