package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// ContentNode is the base unit of a class — a video or article published by
// a teacher. Classification's skill/concept dimensions are many-to-many
// relations against the shared Skill/Concept trees; difficulty_level and
// review_state remain scalar fields on the node itself.
type ContentNode struct {
	ent.Schema
}

func (ContentNode) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("teacher_id", uuid.UUID{}).
			Immutable(),

		field.String("title"),

		field.Enum("content_type").
			Values("video", "article").
			Immutable(),

		field.Enum("difficulty_level").
			Values("beginner", "early_intermediate", "intermediate", "advanced", "expert"),

		// ReviewState always starts pending on creation regardless of any
		// value supplied by the caller — enforced in the domain constructor,
		// not here.
		field.Enum("review_state").
			Values("pending", "confirmed", "overridden").
			Default("pending"),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}

func (ContentNode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("path_exercises", Exercise.Type).
			Ref("content_nodes").
			Through("content_node_exercises", ContentNodeExercise.Type),

		edge.To("languages", Language.Type).
			Through("content_node_languages", ContentNodeLanguage.Type),

		edge.To("skills", Skill.Type).
			Through("content_node_skills", ContentNodeSkill.Type),

		edge.To("concepts", Concept.Type).
			Through("content_node_concepts", ContentNodeConcept.Type),
	}
}
