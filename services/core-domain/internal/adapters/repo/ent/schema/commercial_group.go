package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// CommercialGroup is a named set of users that a flag allows as one
// allowlist entry, group:<slug>.
type CommercialGroup struct {
	ent.Schema
}

func (CommercialGroup) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			StorageKey("slug").
			NotEmpty().
			Immutable(),
		field.String("name").NotEmpty().MaxLen(120),
		field.Time("updated_at"),
		field.UUID("updated_by", uuid.UUID{}),
	}
}
