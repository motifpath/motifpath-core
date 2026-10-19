package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// StudentPathItem is a single content node, pinned to the ContentNodeVersion
// it was at copy time, at a given 1-based position within a StudentPath.
type StudentPathItem struct {
	ent.Schema
}

func (StudentPathItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("student_path_id", uuid.UUID{}).
			Immutable(),

		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		// content_node_version_id pins this item to the ContentNodeVersion
		// its content node was at the moment this StudentPath was copied —
		// never retargeted by a later publish of the same node.
		field.UUID("content_node_version_id", uuid.UUID{}).
			Immutable(),

		field.Int("position").
			Immutable(),

		field.String("section_label").
			Optional().
			Nillable().
			Immutable(),
	}
}

func (StudentPathItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("student_path_id", "position").Unique(),
	}
}
