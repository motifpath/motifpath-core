package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// StudentLearningState tracks the single course enrollment or standalone
// path a student is currently working through, keyed by student_id — a
// role-scoped extension row rather than a field on the shared User record,
// since only students carry this state. At most one of
// current_course_enrollment_id / current_standalone_path_id is set at a
// time; that invariant is enforced in the domain and application layers,
// not by a database constraint, since it depends on values that can each
// independently be null.
type StudentLearningState struct {
	ent.Schema
}

func (StudentLearningState) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("student_id", uuid.UUID{}).
			Unique().
			Immutable(),

		field.UUID("current_course_enrollment_id", uuid.UUID{}).
			Optional().
			Nillable(),

		field.UUID("current_standalone_path_id", uuid.UUID{}).
			Optional().
			Nillable(),
	}
}
