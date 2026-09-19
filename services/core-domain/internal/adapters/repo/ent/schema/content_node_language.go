package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

// ContentNodeLanguage is the join entity behind ContentNode's many-to-many
// "languages" edge — kept independent from ExerciseLanguage since a content
// node's and an exercise's language tagging are edited and queried
// independently. See ADR-024.
type ContentNodeLanguage struct {
	ent.Schema
}

func (ContentNodeLanguage) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.UUID("language_id", uuid.UUID{}).
			Immutable(),

		field.Time("linked_at").
			Immutable().
			Default(time.Now),
	}
}

func (ContentNodeLanguage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("content_node", ContentNode.Type).
			Unique().
			Required().
			Immutable().
			Field("content_node_id"),

		edge.To("language", Language.Type).
			Unique().
			Required().
			Immutable().
			Field("language_id"),
	}
}

func (ContentNodeLanguage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_node_id", "language_id").Unique(),
	}
}
