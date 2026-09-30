package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
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

		// The template's presentation as it was when this copy was made, so
		// a learner's card never depends on a template that may since have
		// been edited or deleted. NULL when the template had none, or the
		// copy predates these columns.
		field.String("summary_snapshot").
			Optional().
			Nillable().
			Immutable(),

		field.Enum("level_snapshot").
			Values("beginner", "early_intermediate", "intermediate", "advanced", "expert").
			Optional().
			Nillable().
			Immutable(),

		field.String("thumbnail_url_snapshot").
			Optional().
			Nillable().
			Immutable(),

		field.UUID("created_by_snapshot", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
	}
}

func (StudentPath) Indexes() []ent.Index {
	return []ent.Index{
		// A learner holds at most one active standalone copy of a template:
		// starting the same path again reuses that copy, and a newer version
		// means archiving it first. Course checkpoint copies are part of
		// their enrollment and don't count.
		index.Fields("student_id", "source_template_id").
			Unique().
			Annotations(entsql.IndexWhere("archived_at IS NULL AND source_course_enrollment_id IS NULL")),
	}
}
