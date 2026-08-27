package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// LearningPathItem is a single content node at a given 1-based position
// within a learning path.
type LearningPathItem struct {
	ent.Schema
}

func (LearningPathItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("learning_path_id", uuid.UUID{}).
			Immutable(),

		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.Int("position").
			Immutable(),
	}
}

func (LearningPathItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("learning_path_id", "position").Unique(),
	}
}
