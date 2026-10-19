package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CourseCheckpoint is one stage of a course's journey at a given 1-based
// position, pointing at a LearningPath template. title is an optional
// override shown instead of the learning path's own title.
type CourseCheckpoint struct {
	ent.Schema
}

func (CourseCheckpoint) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("course_id", uuid.UUID{}).
			Immutable(),

		field.UUID("learning_path_id", uuid.UUID{}).
			Immutable(),

		field.Int("position").
			Immutable(),

		field.String("title").
			Optional().
			Nillable().
			Immutable(),
	}
}

func (CourseCheckpoint) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("course_id", "position").Unique(),
	}
}
