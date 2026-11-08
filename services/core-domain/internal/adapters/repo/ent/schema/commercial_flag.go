package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// CommercialFlag is one registered flag's switch and audience in this
// environment. The migration inserts every registered flag OFF with an empty
// audience; a flag without a row is denied.
type CommercialFlag struct {
	ent.Schema
}

func (CommercialFlag) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			StorageKey("flag").
			NotEmpty().
			Immutable(),
		field.Bool("enabled").Default(false),
		field.Bool("all_users").Default(false),
		// subject_ids are allowlist entries: user:<id>, group:<slug> or
		// service:<name>.
		field.Strings("subject_ids"),
		field.Time("updated_at").Optional().Nillable(),
		field.UUID("updated_by", uuid.UUID{}).Optional().Nillable(),
	}
}
