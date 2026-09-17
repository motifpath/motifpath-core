package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ExerciseOption is one selectable answer choice within an Exercise. It is
// its own entity (not embedded JSON on Exercise) so options are queryable
// and updatable individually. Which of Label, ImageURL, AudioURL, or the
// Region* fields is populated depends on the parent exercise's exercise_type.
type ExerciseOption struct {
	ent.Schema
}

func (ExerciseOption) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("exercise_id", uuid.UUID{}),

		field.Bool("is_correct"),

		field.String("label").
			Optional().
			Nillable(),

		field.String("image_url").
			Optional().
			Nillable(),

		field.String("audio_url").
			Optional().
			Nillable(),

		field.Float("region_x").
			Optional().
			Nillable(),
		field.Float("region_y").
			Optional().
			Nillable(),
		field.Float("region_width").
			Optional().
			Nillable(),
		field.Float("region_height").
			Optional().
			Nillable(),
		field.Enum("region_shape").
			Values("rectangle", "circle").
			Optional().
			Nillable(),
	}
}

func (ExerciseOption) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("exercise", Exercise.Type).
			Ref("options").
			Field("exercise_id").
			Unique().
			Required(),
	}
}

func (ExerciseOption) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("exercise_id"),
	}
}
