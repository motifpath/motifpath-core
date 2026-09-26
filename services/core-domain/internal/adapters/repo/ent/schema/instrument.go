package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Instrument is what a Diagram is authored against. family decides which
// field group is populated: string_count and tuning for fretted, the
// key_range_* pair for keyboard — the other group stays null. The
// combination is validated in the domain constructor, since neither a
// column constraint nor ent expresses "these fields iff that enum value".
type Instrument struct {
	ent.Schema
}

func (Instrument) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		// names maps a language code to the instrument's name in that
		// language; every language MotifPath offers is required on write.
		field.JSON("names", map[string]string{}),

		field.Enum("family").
			Values("fretted", "keyboard").
			Immutable(),

		field.Int("string_count").
			Optional().
			Nillable(),

		field.Strings("tuning").
			Optional(),

		field.String("key_range_lowest").
			Optional().
			Nillable(),

		field.String("key_range_highest").
			Optional().
			Nillable(),
	}
}

func (Instrument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("diagrams", Diagram.Type).
			Ref("instrument"),

		edge.From("courses", Course.Type).
			Ref("instruments").
			Through("course_instruments", CourseInstrument.Type),

		edge.From("learning_paths", LearningPath.Type).
			Ref("instruments").
			Through("learning_path_instruments", LearningPathInstrument.Type),

		edge.From("content_nodes", ContentNode.Type).
			Ref("instruments").
			Through("content_node_instruments", ContentNodeInstrument.Type),
	}
}
