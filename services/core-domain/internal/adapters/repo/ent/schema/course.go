package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Course is an ordered sequence of learning-path checkpoints students
// progress through, with an explicit draft/publish split. This row is
// always the live, currently-being-authored draft. Its checkpoints live in
// the separate CourseCheckpoint table (see course_checkpoint.go), joined by
// course_id.
type Course struct {
	ent.Schema
}

func (Course) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.String("title"),

		field.String("summary"),

		field.Enum("level").
			Values("beginner", "early_intermediate", "intermediate", "advanced", "expert"),

		field.Enum("status").
			Values("draft", "published", "retired").
			Default("draft"),

		field.UUID("created_by", uuid.UUID{}).
			Immutable(),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}
