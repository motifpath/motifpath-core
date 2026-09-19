package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Language is a lookup entity for languages MotifPath content or a user's
// locale preference can be tagged with. Seeded by the Atlas migration with
// system rows: en, pt_BR, and the literal "any" (language-agnostic content).
type Language struct {
	ent.Schema
}

func (Language) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.String("code").
			Unique().
			Immutable(),

		field.String("name"),
	}
}

func (Language) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("content_nodes", ContentNode.Type).
			Ref("languages").
			Through("content_node_languages", ContentNodeLanguage.Type),

		edge.From("exercises", Exercise.Type).
			Ref("languages").
			Through("exercise_languages", ExerciseLanguage.Type),
	}
}
