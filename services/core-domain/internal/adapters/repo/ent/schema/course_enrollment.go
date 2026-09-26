package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// CourseEnrollment is a student's self-enrollment in one course, pinned to
// the CourseVersion published at enrollment time. Its active checkpoint's
// StudentPath lives in the separate StudentPath table (see student_path.go,
// joined by source_course_enrollment_id), not here.
type CourseEnrollment struct {
	ent.Schema
}

func (CourseEnrollment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("student_id", uuid.UUID{}).
			Immutable(),

		field.UUID("course_id", uuid.UUID{}).
			Immutable(),

		field.String("course_title").
			Immutable(),

		// course_thumbnail_url is the pinned version's thumbnail, copied at
		// enrollment like course_title; NULL = none.
		field.String("course_thumbnail_url").
			Optional().
			Nillable(),

		field.Int("course_version_number").
			Immutable(),

		field.Enum("status").
			Values("active", "completed", "abandoned").
			Default("active"),

		// active_checkpoint_student_path_id and active_checkpoint_position
		// are set together, both nil once the enrollment is completed or
		// abandoned.
		field.UUID("active_checkpoint_student_path_id", uuid.UUID{}).
			Optional().
			Nillable(),

		field.Int("active_checkpoint_position").
			Optional().
			Nillable(),

		field.Time("enrolled_at").
			Immutable().
			Default(time.Now),
	}
}
