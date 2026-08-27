package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// PathAssignment records a learning path assigned to a student. Only one
// active assignment exists per student at MVP — the student_id unique
// constraint enforces that at the database level; PathAssignmentRepository's
// ReplaceActive deletes any existing row for the student before inserting
// the new one, so "assigning a new path" always produces a fresh
// assignment_id rather than mutating the old row in place.
type PathAssignment struct {
	ent.Schema
}

func (PathAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("student_id", uuid.UUID{}).
			Unique().
			Immutable(),

		field.UUID("learning_path_id", uuid.UUID{}).
			Immutable(),

		field.UUID("assigned_by", uuid.UUID{}).
			Immutable(),

		field.Time("assigned_at").
			Immutable().
			Default(time.Now),
	}
}
