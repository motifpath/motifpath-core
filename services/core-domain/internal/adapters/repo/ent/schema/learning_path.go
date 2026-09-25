package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// LearningPath is an ordered sequence of content nodes assigned to students
// as a structured curriculum. Its items live in the separate
// LearningPathItem table (see learning_path_item.go), joined by
// learning_path_id.
type LearningPath struct {
	ent.Schema
}

func (LearningPath) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("teacher_id", uuid.UUID{}).
			Immutable(),

		field.String("title"),

		// level is the level a learner should be at to follow the path.
		// NULL only for a path created before levels were recorded, until
		// it is next saved.
		field.Enum("level").
			Values("beginner", "early_intermediate", "intermediate", "advanced", "expert").
			Optional().
			Nillable(),

		// updated_at is when the path was created or last replaced.
		field.Time("updated_at").
			Default(time.Now),

		// thumbnail_url is the image shown for it in lists and cards; NULL =
		// none.
		field.String("thumbnail_url").
			Optional().
			Nillable(),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}

func (LearningPath) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("instruments", Instrument.Type).
			Through("learning_path_instruments", LearningPathInstrument.Type),
	}
}
