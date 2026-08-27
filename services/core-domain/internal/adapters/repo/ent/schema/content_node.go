package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// ContentNode is the base unit of a class — a video or article published by
// a teacher. Classification (skill, concept, difficulty_level, review_state)
// is embedded directly on the node rather than modeled as a separate
// entity, matching the merged core-domain-service.yaml spec.
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

		field.String("skill"),
		field.String("concept"),

		field.Enum("difficulty_level").
			Values("beginner", "intermediate", "advanced"),

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
