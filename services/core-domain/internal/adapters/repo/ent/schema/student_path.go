package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// StudentPath is a student's own copy of a learning path template's items,
// created by copying the template's items at assign or enrollment time.
// Independently editable afterwards and never affected by later changes to
// the template it was copied from. Its items live in the separate
// StudentPathItem table (see student_path_item.go), joined by
// student_path_id.
type StudentPath struct {
	ent.Schema
}

func (StudentPath) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("student_id", uuid.UUID{}).
			Immutable(),

		field.UUID("source_template_id", uuid.UUID{}).
			Immutable(),

		field.String("title"),

		field.UUID("assigned_by", uuid.UUID{}).
			Immutable(),

		field.Time("assigned_at").
			Immutable().
			Default(time.Now),

		field.Time("archived_at").
			Optional().
			Nillable(),

		// source_course_enrollment_id and course_checkpoint_position are set
		// together, both nil for a standalone path (staff-assigned directly,
		// not via a course).
		field.UUID("source_course_enrollment_id", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),

		field.Int("course_checkpoint_position").
			Optional().
			Nillable().
			Immutable(),
	}
}
