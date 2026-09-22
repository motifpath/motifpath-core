package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CourseVersionCheckpoint is one checkpoint's identity as it existed at the
// moment its CourseVersion was published: position, the LearningPath
// template it pointed at, and the title actually shown then. A checkpoint's
// items are never snapshotted here — resolving a published course's
// outline reads each checkpoint's current items live from the referenced
// LearningPath at read time, the same "resolve live from current state"
// pattern StudentPathItem already uses for its title/content_type.
type CourseVersionCheckpoint struct {
	ent.Schema
}

func (CourseVersionCheckpoint) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("course_version_id", uuid.UUID{}).
			Immutable(),

		field.UUID("learning_path_id", uuid.UUID{}).
			Immutable(),

		field.Int("position").
			Immutable(),

		field.String("effective_title").
			Immutable(),
	}
}

func (CourseVersionCheckpoint) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("course_version_id", "position").Unique(),
	}
}
