package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Challenge is the assessment unit for a content node: it groups exercises
// and carries the subject tag, pass threshold, and optional remediation
// target used by the rules-based recommendation engine.
type Challenge struct {
	ent.Schema
}

func (Challenge) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("content_node_id", uuid.UUID{}).
			Immutable(),

		field.String("subject_tag"),
		field.Int("pass_threshold"),

		field.UUID("remediation_target_content_node_id", uuid.UUID{}).
			Optional().
			Nillable(),

		field.Time("created_at").
			Immutable().
			Default(time.Now),
	}
}

func (Challenge) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("content_node_id"),
	}
}
