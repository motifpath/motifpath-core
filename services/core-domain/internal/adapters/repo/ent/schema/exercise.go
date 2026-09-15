package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

// Exercise is a reusable, standalone practice item classified by skill tags
// and independent of any single challenge — it may be linked to zero, one,
// or many challenges via the many-to-many "challenges" edge.
type Exercise struct {
	ent.Schema
}

func (Exercise) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.String("title"),
		field.Text("prompt"),

		field.Enum("exercise_type").
			Values("text_response", "audio_recognition", "image_recognition", "image_choice").
			Immutable(),

		field.JSON("skill_tags", []string{}).
			Optional(),

		field.String("image_url").
			Optional().
			Nillable(),

		field.String("audio_url").
			Optional().
			Nillable(),

		field.Int("estimated_duration_seconds").
			Optional().
			Nillable(),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}

func (Exercise) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("challenges", Challenge.Type),
		edge.To("content_nodes", ContentNode.Type),
		edge.To("options", ExerciseOption.Type),
	}
}
