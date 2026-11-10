package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// CommercialGroupMember makes one core user a member of one rollout group.
type CommercialGroupMember struct {
	ent.Schema
}

func (CommercialGroupMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("group_slug").NotEmpty().Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
	}
}

func (CommercialGroupMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("group", CommercialGroup.Type).
			Field("group_slug").
			Unique().
			Required().
			Immutable(),
		edge.To("user", User.Type).
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (CommercialGroupMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_slug", "user_id").Unique(),
	}
}
